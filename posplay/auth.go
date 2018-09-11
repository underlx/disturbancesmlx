package posplay

import (
	"net/http"

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
type ConnectionHandler struct{}

// TryCreateConnection implements resource.PairConnectionHandler
func (h *ConnectionHandler) TryCreateConnection(node sqalx.Node, code string, pair *dataobjects.APIPair) bool {
	// TODO
	return false
}
