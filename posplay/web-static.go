package posplay

import (
	"net/http"

	"github.com/underlx/disturbancesmlx/types"
)

func homePage(w http.ResponseWriter, r *http.Request) {
	session, _, err := GetSession(r, w, false)
	if err != nil {
		config.Log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if session != nil {
		dashboardPage(w, r, session)
		return
	}

	tx, err := config.Node.Beginx()
	if err != nil {
		config.Log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer tx.Commit() // read-only tx

	p := struct {
		pageCommons
		TotalPlayers      int
		TotalXP           int
		TotalTrips        int
		TotalAchievements int
	}{}

	p.TotalPlayers, err = types.CountPPPlayers(tx)
	if err != nil {
		config.Log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	p.TotalXP, err = types.GetPPXPTransactionsTotal(tx)
	if err != nil {
		config.Log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	p.TotalTrips, err = types.CountPPXPTransactionsWithType(tx, "TRIP_SUBMIT_REWARD")
	if err != nil {
		config.Log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	p.TotalAchievements, err = types.CountPPPlayerAchievementsAchieved(tx)
	if err != nil {
		config.Log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	p.pageCommons, err = initPageCommons(nil, w, r, "Página principal", session, nil)
	if err != nil {
		config.Log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	err = webtemplate.ExecuteTemplate(w, "index.html", p)
	if err != nil {
		config.Log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
	}
}

func privacyPolicyPage(w http.ResponseWriter, r *http.Request) {
	session, _, err := GetSession(r, w, false)
	if err != nil {
		config.Log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	tx, err := config.Node.Beginx()
	if err != nil {
		config.Log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer tx.Commit() // read-only tx

	p := struct {
		pageCommons
	}{}
	p.pageCommons, err = initPageCommons(tx, w, r, "Política de Privacidade", session, nil)
	if err != nil {
		config.Log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	err = webtemplate.ExecuteTemplate(w, "privacy.html", p)
	if err != nil {
		config.Log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
	}
}
