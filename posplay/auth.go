package posplay

import (
	"net/http"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/dchest/uniuri"
	"github.com/gbl08ma/sqalx"
	"github.com/underlx/disturbancesmlx/dataobjects"
	"golang.org/x/oauth2"
)

func callbackHandler(w http.ResponseWriter, r *http.Request) {
	tx, err := config.Node.Beginx()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		config.Log.Println(err)
		return
	}
	defer tx.Rollback()

	code := r.FormValue("code")
	token, err := oauthConfig.Exchange(oauth2.NoContext, code)
	if err != nil {
		config.Log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if !token.Valid() {
		config.Log.Println("Retrieved invalid token")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	state := r.FormValue("state")

	session, _ := config.Store.Get(r, SessionName)

	if state != session.Values["oauthState"] {
		config.Log.Println("Session state does not match state in callback request")
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	_, err = NewSession(tx, r, w, token)
	if err != nil {
		config.Log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, config.PathPrefix+"/", http.StatusTemporaryRedirect)
	tx.Commit()
}

// ConnectionHandler implements resource.PairConnectionHandler
type ConnectionHandler struct {
	mu              sync.Mutex
	processesByID   map[uint64]*pairProcess
	processesByCode map[string]*pairProcess
}

type pairProcess struct {
	DiscordID uint64
	Code      string
	Expires   time.Time
}

// TheConnectionHandler is the pair connection handler for PosPlay
var TheConnectionHandler *ConnectionHandler

func init() {
	TheConnectionHandler = &ConnectionHandler{
		processesByID:   make(map[uint64]*pairProcess),
		processesByCode: make(map[string]*pairProcess),
	}
}

// TryCreateConnection implements resource.PairConnectionHandler
func (h *ConnectionHandler) TryCreateConnection(node sqalx.Node, code string, pair *dataobjects.APIPair) bool {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.removeExpired(false)

	code = strings.ToLower(code)
	code = strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return r
	}, code)

	process, hasProcess := h.processesByCode[code]
	if !hasProcess {
		return false
	}

	tx, err := node.Beginx()
	if err != nil {
		config.Log.Println(err)
		return false
	}
	defer tx.Rollback()

	// TODO remove any current pair for this discord ID
	// TODO remove any current pair for this APIPair
	pppair := dataobjects.PPPair{
		DiscordID: process.DiscordID,
		Pair:      pair,
		Paired:    time.Now(),
	}

	err = pppair.Update(tx)
	if err != nil {
		config.Log.Println(err)
		return false
	}

	return true
}

// DisplayName implements resource.PairConnectionHandler
func (h *ConnectionHandler) DisplayName() string {
	return "PosPlay"
}

func (h *ConnectionHandler) removeExpired(lock bool) {
	if lock {
		h.mu.Lock()
		defer h.mu.Unlock()
	}

	for id, process := range h.processesByID {
		if time.Now().After(process.Expires) {
			delete(h.processesByID, id)
			delete(h.processesByCode, process.Code)
		}
	}
}

func (h *ConnectionHandler) getProcess(discordID uint64) *pairProcess {
	h.mu.Lock()
	defer h.mu.Unlock()

	process, hasProcess := h.processesByID[discordID]
	if hasProcess {
		return process
	}

	code := uniuri.NewLenChars(6, []byte("0123456789abcdefghjkmnpqrstwxyz"))

	niceCode := strings.ToUpper(code)
	niceCode = niceCode[0:3] + " " + niceCode[3:6]

	process = &pairProcess{
		DiscordID: discordID,
		Expires:   time.Now().Add(PairProcessLongevity),
		Code:      niceCode,
	}

	h.processesByID[discordID] = process
	h.processesByCode[code] = process
	return process
}
