package posplay

import (
	"net/http"
	"sort"
	"time"

	"github.com/gorilla/mux"
	"github.com/underlx/disturbancesmlx/dataobjects"
)

func profilePage(w http.ResponseWriter, r *http.Request) {
	session, redirected, err := GetSession(r, w, false)
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
	defer tx.Commit() // read-only tx

	var player *dataobjects.PPPlayer
	if session != nil {
		player, err = dataobjects.GetPPPlayer(tx, uidConvS(session.DiscordInfo.ID))
		if err != nil {
			config.Log.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}

	profile, err := dataobjects.GetPPPlayer(tx, uidConvS(mux.Vars(r)["id"]))
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	p := struct {
		pageCommons

		ShowAsPrivate      bool
		ProfilePlayer      *dataobjects.PPPlayer
		ProfileXP          int
		ProfileLevel       int
		AllTimeLeaderboard []dataobjects.PPLeaderboardEntry
		WeekLeaderboard    []dataobjects.PPLeaderboardEntry
		Achieved           []*dataobjects.PPAchievement
		AchievedPlayer     map[string]*dataobjects.PPPlayerAchievement
	}{
		ProfilePlayer:  profile,
		AchievedPlayer: make(map[string]*dataobjects.PPPlayerAchievement),
	}

	switch profile.ProfilePrivacy {
	case PrivateProfilePrivacy:
		p.ShowAsPrivate = player == nil || player.DiscordID != profile.DiscordID
	case PlayersOnlyProfilePrivacy:
		p.ShowAsPrivate = player == nil
	case PublicProfilePrivacy:
		p.ShowAsPrivate = false
	}

	p.pageCommons, err = initPageCommons(tx, w, r, profile.CachedName, session, player)
	if err != nil {
		config.Log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	p.UserInfoInHeader = true

	p.ProfileXP, p.ProfileLevel, _, err = profile.Level(tx)
	if err != nil {
		config.Log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// leaderboards
	mustIncludes := []*dataobjects.PPPlayer{profile}
	if player != nil {
		mustIncludes = append(mustIncludes, player)
	}

	entries, err := dataobjects.PPLeaderboardBetween(tx, time.Time{}, time.Now(), 3, 1, mustIncludes...)
	if err != nil {
		config.Log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
	}
	if len(entries) == 1 && entries[0].Position == 0 {
		// avoid showing just this player in the 0th place
		entries = []dataobjects.PPLeaderboardEntry{}
	}

	p.AllTimeLeaderboard = entries

	weekEntries, err := dataobjects.PPLeaderboardBetween(tx, WeekStart(), time.Now(), 3, 1, mustIncludes...)
	if err != nil {
		config.Log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
	}
	if len(weekEntries) == 1 && weekEntries[0].Position == 0 {
		// avoid showing just this player in the 0th place
		weekEntries = []dataobjects.PPLeaderboardEntry{}
	}

	p.WeekLeaderboard = weekEntries

	// achievements

	playerAchieved, err := profile.Achievements(tx)
	if err != nil {
		config.Log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	for _, achplayer := range playerAchieved {
		if achplayer.Achieved {
			p.Achieved = append(p.Achieved, achplayer.Achievement)
			p.AchievedPlayer[achplayer.Achievement.ID] = achplayer
		}
	}

	sort.Slice(p.Achieved, func(i, j int) bool {
		return p.AchievedPlayer[p.Achieved[i].ID].AchievedTime.After(p.AchievedPlayer[p.Achieved[j].ID].AchievedTime)
	})
	max := 4
	if len(p.Achieved) < 4 {
		max = len(p.Achieved)
	}
	p.Achieved = p.Achieved[0:max]

	err = webtemplate.ExecuteTemplate(w, "profile.html", p)
	if err != nil {
		config.Log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
	}
}

func profileAchievementsPage(w http.ResponseWriter, r *http.Request) {
	session, redirected, err := GetSession(r, w, false)
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
	defer tx.Commit() // read-only tx

	var player *dataobjects.PPPlayer
	if session != nil {
		player, err = dataobjects.GetPPPlayer(tx, uidConvS(session.DiscordInfo.ID))
		if err != nil {
			config.Log.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}

	profile, err := dataobjects.GetPPPlayer(tx, uidConvS(mux.Vars(r)["id"]))
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	p := struct {
		pageCommons

		ShowAsPrivate  bool
		ProfilePlayer  *dataobjects.PPPlayer
		ProfileXP      int
		ProfileLevel   int
		Achieved       []*dataobjects.PPAchievement
		AchievedPlayer map[string]*dataobjects.PPPlayerAchievement
	}{
		ProfilePlayer:  profile,
		AchievedPlayer: make(map[string]*dataobjects.PPPlayerAchievement),
	}

	switch profile.ProfilePrivacy {
	case PrivateProfilePrivacy:
		p.ShowAsPrivate = player == nil || player.DiscordID != profile.DiscordID
	case PlayersOnlyProfilePrivacy:
		p.ShowAsPrivate = player == nil
	case PublicProfilePrivacy:
		p.ShowAsPrivate = false
	}

	p.pageCommons, err = initPageCommons(tx, w, r, profile.CachedName, session, player)
	if err != nil {
		config.Log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	p.UserInfoInHeader = true

	p.ProfileXP, p.ProfileLevel, _, err = profile.Level(tx)
	if err != nil {
		config.Log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// achievements

	playerAchieved, err := profile.Achievements(tx)
	if err != nil {
		config.Log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	for _, achplayer := range playerAchieved {
		if achplayer.Achieved {
			p.Achieved = append(p.Achieved, achplayer.Achievement)
			p.AchievedPlayer[achplayer.Achievement.ID] = achplayer
		}
	}

	sort.Slice(p.Achieved, func(i, j int) bool {
		return p.AchievedPlayer[p.Achieved[i].ID].AchievedTime.After(p.AchievedPlayer[p.Achieved[j].ID].AchievedTime)
	})

	err = webtemplate.ExecuteTemplate(w, "profile-achievements.html", p)
	if err != nil {
		config.Log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
	}
}
