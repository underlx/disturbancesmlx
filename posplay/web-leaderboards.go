package posplay

import (
	"net/http"
	"time"

	"github.com/underlx/disturbancesmlx/types"
)

func leaderboardsWeekPage(w http.ResponseWriter, r *http.Request) {
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

	player, err := types.GetPPPlayer(tx, discordID)
	if err != nil {
		config.Log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	p := struct {
		pageCommons
		Leaderboards []struct {
			Start   time.Time
			Entries []types.PPLeaderboardEntry
		}
	}{}
	p.pageCommons, err = initPageCommons(tx, w, r, "Classificações semanais", session, player)
	if err != nil {
		config.Log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	p.SidebarSelected = "leaderboards"

	start := WeekStart()
	end := time.Now()
	for i := 0; i < 5; i++ {
		entries, err := types.PPLeaderboardBetween(tx, start, end, 15, 2, player)
		if err != nil {
			config.Log.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
		}
		if len(entries) == 1 && entries[0].Position == 0 {
			// avoid showing just this player in the 0th place
			entries = []types.PPLeaderboardEntry{}
		}

		p.Leaderboards = append(p.Leaderboards, struct {
			Start   time.Time
			Entries []types.PPLeaderboardEntry
		}{
			Start:   start,
			Entries: entries,
		})

		end = start
		start = start.AddDate(0, 0, -7)
	}

	err = webtemplate.ExecuteTemplate(w, "leaderboards-week.html", p)
	if err != nil {
		config.Log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
	}
}

func leaderboardsMonthPage(w http.ResponseWriter, r *http.Request) {
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

	player, err := types.GetPPPlayer(tx, discordID)
	if err != nil {
		config.Log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	p := struct {
		pageCommons
		Leaderboards []struct {
			Start   time.Time
			Entries []types.PPLeaderboardEntry
		}
	}{}
	p.pageCommons, err = initPageCommons(tx, w, r, "Classificações mensais", session, player)
	if err != nil {
		config.Log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	p.SidebarSelected = "leaderboards"

	origStart := MonthStart()
	start := origStart
	end := time.Now()
	for i := 0; i < 5; i++ {
		entries, err := types.PPLeaderboardBetween(tx, start, end, 15, 2, player)
		if err != nil {
			config.Log.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
		}
		if len(entries) == 1 && entries[0].Position == 0 {
			// avoid showing just this player in the 0th place
			entries = []types.PPLeaderboardEntry{}
		}

		p.Leaderboards = append(p.Leaderboards, struct {
			Start   time.Time
			Entries []types.PPLeaderboardEntry
		}{
			Start:   start,
			Entries: entries,
		})

		end = start
		start = origStart.AddDate(0, -(i + 1), 0)
	}

	err = webtemplate.ExecuteTemplate(w, "leaderboards-month.html", p)
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

	player, err := types.GetPPPlayer(tx, discordID)
	if err != nil {
		config.Log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	p := struct {
		pageCommons
		Leaderboard struct {
			Entries []types.PPLeaderboardEntry
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
	entries, err := types.PPLeaderboardBetween(tx, start, end, 50, 2, player)
	if err != nil {
		config.Log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
	}
	if len(entries) == 1 && entries[0].Position == 0 {
		// avoid showing just this player in the 0th place
		entries = []types.PPLeaderboardEntry{}
	}

	p.Leaderboard = struct {
		Entries []types.PPLeaderboardEntry
	}{
		Entries: entries,
	}

	err = webtemplate.ExecuteTemplate(w, "leaderboards-alltime.html", p)
	if err != nil {
		config.Log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
	}
}
