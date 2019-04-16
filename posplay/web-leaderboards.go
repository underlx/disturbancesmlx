package posplay

import (
	"net/http"
	"time"

	"github.com/underlx/disturbancesmlx/dataobjects"
)

func leaderboardsPage(w http.ResponseWriter, r *http.Request) {
	session, redirected, err := GetSession(r, w, true)
	if err != nil {
		config.Log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if redirected {
		return
	}

	tx, err := config.Node.Beginx()
	if err != nil {
		config.Log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer tx.Commit() // read-only

	discordID := uidConvS(session.DiscordInfo.ID)

	player, err := dataobjects.GetPPPlayer(tx, discordID)
	if err != nil {
		config.Log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	p := struct {
		pageCommons
		Leaderboards []struct {
			Start   time.Time
			Entries []dataobjects.PPLeaderboardEntry
		}
	}{}
	p.pageCommons, err = initPageCommons(tx, w, r, "Classificações", session, player)
	if err != nil {
		config.Log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	p.SidebarSelected = "leaderboards"

	start := WeekStart()
	end := time.Now()
	for i := 0; i < 5; i++ {
		entries, err := dataobjects.PPLeaderboardBetween(tx, start, end, 15, player)
		if err != nil {
			config.Log.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
		}
		if len(entries) == 1 && entries[0].Position == 0 {
			// avoid showing just this player in the 0th place
			entries = []dataobjects.PPLeaderboardEntry{}
		}

		p.Leaderboards = append(p.Leaderboards, struct {
			Start   time.Time
			Entries []dataobjects.PPLeaderboardEntry
		}{
			Start:   start,
			Entries: entries,
		})

		end = start
		start = start.AddDate(0, 0, -7)
	}

	err = webtemplate.ExecuteTemplate(w, "leaderboards.html", p)
	if err != nil {
		config.Log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
	}
}

func leaderboardsAllTimePage(w http.ResponseWriter, r *http.Request) {
	session, redirected, err := GetSession(r, w, true)
	if err != nil {
		config.Log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if redirected {
		return
	}

	tx, err := config.Node.Beginx()
	if err != nil {
		config.Log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer tx.Commit() // read-only

	discordID := uidConvS(session.DiscordInfo.ID)

	player, err := dataobjects.GetPPPlayer(tx, discordID)
	if err != nil {
		config.Log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	p := struct {
		pageCommons
		Leaderboard struct {
			Entries []dataobjects.PPLeaderboardEntry
		}
	}{}
	p.pageCommons, err = initPageCommons(tx, w, r, "Classificações globais", session, player)
	if err != nil {
		config.Log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	p.SidebarSelected = "leaderboards"

	start := time.Time{}
	end := time.Now()
	entries, err := dataobjects.PPLeaderboardBetween(tx, start, end, 50, player)
	if err != nil {
		config.Log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
	}
	if len(entries) == 1 && entries[0].Position == 0 {
		// avoid showing just this player in the 0th place
		entries = []dataobjects.PPLeaderboardEntry{}
	}

	p.Leaderboard = struct {
		Entries []dataobjects.PPLeaderboardEntry
	}{
		Entries: entries,
	}

	err = webtemplate.ExecuteTemplate(w, "leaderboards-alltime.html", p)
	if err != nil {
		config.Log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
	}
}
