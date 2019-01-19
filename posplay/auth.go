package posplay

import (
	"net/http"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/bwmarrin/discordgo"

	"github.com/dchest/uniuri"
	"github.com/gbl08ma/sqalx"
	"github.com/underlx/disturbancesmlx/dataobjects"
	"github.com/underlx/disturbancesmlx/discordbot"
)

func callbackHandler(w http.ResponseWriter, r *http.Request) {
	tx, err := config.Node.Beginx()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		config.Log.Println(err)
		return
	}
	defer tx.Rollback()

	state := r.FormValue("state")

	session, _ := config.Store.Get(r, SessionName)

	if state != session.Values["oauthState"] {
		config.Log.Println("Session state does not match state in callback request")
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	code := r.FormValue("code")
	token, err := oauthConfig.Exchange(r.Context(), code)
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
	DiscordID       uint64
	Code            string
	Expires         time.Time
	Completed       bool
	RemovedExisting bool
}

// TheConnectionHandler is the pair connection handler for PosPlay
var TheConnectionHandler *ConnectionHandler

func init() {
	TheConnectionHandler = &ConnectionHandler{
		processesByID:   make(map[uint64]*pairProcess),
		processesByCode: make(map[string]*pairProcess),
	}
}

// ID implements resource.PairConnectionHandler
func (h *ConnectionHandler) ID() string {
	return "posplay"
}

// TryCreateConnection implements resource.PairConnectionHandler
func (h *ConnectionHandler) TryCreateConnection(node sqalx.Node, code, deviceName string, pair *dataobjects.APIPair) bool {
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
	if !hasProcess || process.Completed {
		return false
	}

	tx, err := node.Beginx()
	if err != nil {
		config.Log.Println(err)
		return false
	}
	defer tx.Rollback()

	// remove any current pair for this discord ID

	existing, err := dataobjects.GetPPPair(tx, process.DiscordID)
	if err == nil {
		err = existing.Delete(tx)
		if err != nil {
			config.Log.Println(err)
			return false
		}
		process.RemovedExisting = true
	}

	// remove any current pair for this API key
	removedExistingKey := false
	existingPair, err := dataobjects.GetPPPairForKey(tx, pair.Key)
	if err == nil {
		err = existingPair.Delete(tx)
		if err != nil {
			config.Log.Println(err)
			return false
		}
		removedExistingKey = true
	}

	// save pair
	pppair := dataobjects.PPPair{
		DiscordID:  process.DiscordID,
		Pair:       pair,
		Paired:     time.Now(),
		DeviceName: deviceName,
	}

	err = pppair.Update(tx)
	if err != nil {
		config.Log.Println(err)
		return false
	}

	// give XP bonus if it has never been given before
	player, err := dataobjects.GetPPPlayer(tx, process.DiscordID)
	if err != nil {
		config.Log.Println(err)
		return false
	}
	txs, err := player.XPTransactionsWithType(tx, "PAIR_BONUS")
	if err != nil {
		config.Log.Println(err)
		return false
	}

	if len(txs) == 0 {
		err = DoXPTransaction(tx, player, pppair.Paired, 30, "PAIR_BONUS", nil, false)
		if err != nil {
			config.Log.Println(err)
			return false
		}
	}

	// send notifications after tx commits successfully
	tx.DeferToCommit(func() {
		process.Completed = true

		content := ""

		if process.RemovedExisting {
			content = "Acabou de trocar o dispositivo associado com a sua conta PosPlay.\n"
			content += "Dispositivo anterior: **" + existing.DeviceName + "**\n"
			content += "Novo dispositivo: **" + deviceName + "**"
			if removedExistingKey {
				content += "(anteriormente associado com outra conta)"
			}
		} else {
			content = "Acabou de associar um dispositivo (**" + deviceName + "**) com a sua conta PosPlay."
			if removedExistingKey {
				content += " Este dispositivo estava anteriormente associado com outra conta."
			}
		}

		discordbot.SendDMtoUser(uidConvI(process.DiscordID), &discordgo.MessageSend{
			Content: content,
		})

		if removedExistingKey {
			discordbot.SendDMtoUser(uidConvI(existingPair.DiscordID), &discordgo.MessageSend{
				Content: "⚠ O dispositivo **" + existingPair.DeviceName + "** passou a estar associado com outra conta PosPlay.\n" +
					"Se esta não foi uma acção iniciada por si, certifique-se que mais ninguém tem acesso ao seu aparelho, e torne a associá-lo a esta conta.\n" +
					"Em caso de dúvida, contacte-nos através do endereço de email underlx@tny.im",
			})
		}
	})

	err = tx.Commit()
	if err != nil {
		config.Log.Println(err)
		return false
	}

	return true
}

// GetConnectionsForPair implements resource.PairConnectionHandler
func (h *ConnectionHandler) GetConnectionsForPair(node sqalx.Node, pair *dataobjects.APIPair) ([]dataobjects.PairConnection, error) {
	tx, err := node.Beginx()
	if err != nil {
		return nil, err
	}
	defer tx.Commit()

	ppPair, err := dataobjects.GetPPPairForKey(tx, pair.Key)
	if err != nil {
		return []dataobjects.PairConnection{}, nil
	}

	info, err := playerXPinfoWithTx(tx, uidConvI(ppPair.DiscordID))
	if err != nil {
		return []dataobjects.PairConnection{}, err
	}

	return []dataobjects.PairConnection{&PairConnection{
		pair:    pair,
		created: ppPair.Paired,
		extra: PairConnectionExtra{
			DiscordID:     ppPair.DiscordID,
			Username:      info.Username,
			AvatarURL:     info.AvatarURL,
			Level:         info.Level,
			LevelProgress: info.LevelProgress,
			XP:            info.XP,
			XPthisWeek:    info.XPthisWeek,
			Rank:          info.Rank,
			RankThisWeek:  info.RankThisWeek,
		},
	}}, nil
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
			delete(h.processesByCode, process.Code)
			if !process.Completed {
				delete(h.processesByID, id)
			} else if time.Now().After(process.Expires.Add(PairProcessLongevity)) {
				delete(h.processesByID, id)
			}
		}
	}
}

func (h *ConnectionHandler) getProcess(discordID uint64) *pairProcess {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.removeExpired(false)

	process, hasProcess := h.processesByID[discordID]
	if hasProcess && (!time.Now().After(process.Expires) || process.Completed) {
		return process
	}

	code := uniuri.NewLenChars(6, []byte("123456789bcdfghjkmnpqrstvwxyz"))

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
