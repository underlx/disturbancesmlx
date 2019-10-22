package posplay

import (
	"net/http"
	"sort"
	"time"

	"github.com/gorilla/mux"
	"github.com/underlx/disturbancesmlx/types"
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

	var player *types.PPPlayer
	if session != nil {
		player, err = types.GetPPPlayer(tx, uidConvS(session.DiscordInfo.ID))
		if err != nil {
			config.Log.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}

	profile, err := types.GetPPPlayer(tx, uidConvS(mux.Vars(r)["id"]))
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	p := struct {
		pageCommons

		ShowAsPrivate      bool
		ProfilePlayer      *types.PPPlayer
		ProfileXP          int
		ProfileLevel       int
		AllTimeLeaderboard []types.PPLeaderboardEntry
		WeekLeaderboard    []types.PPLeaderboardEntry
		Achieved           []*types.PPAchievement
		AchievedPlayer     map[string]*types.PPPlayerAchievement
	}{
		ProfilePlayer:  profile,
		AchievedPlayer: make(map[string]*types.PPPlayerAchievement),
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
	mustIncludes := []*types.PPPlayer{profile}
	if player != nil {
		mustIncludes = append(mustIncludes, player)
	}

	entries, err := types.PPLeaderboardBetween(tx, time.Time{}, time.Now(), 3, 1, mustIncludes...)
	if err != nil {
		config.Log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
	}
	if len(entries) == 1 && entries[0].Position == 0 {
		// avoid showing just this player in the 0th place
		entries = []types.PPLeaderboardEntry{}
	}

	p.AllTimeLeaderboard = entries

	weekEntries, err := types.PPLeaderboardBetween(tx, WeekStart(), time.Now(), 3, 1, mustIncludes...)
	if err != nil {
		config.Log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
	}
	if len(weekEntries) == 1 && weekEntries[0].Position == 0 {
		// avoid showing just this player in the 0th place
		weekEntries = []types.PPLeaderboardEntry{}
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

	var player *types.PPPlayer
	if session != nil {
		player, err = types.GetPPPlayer(tx, uidConvS(session.DiscordInfo.ID))
		if err != nil {
			config.Log.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}

	profile, err := types.GetPPPlayer(tx, uidConvS(mux.Vars(r)["id"]))
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	p := struct {
		pageCommons

		ShowAsPrivate  bool
		ProfilePlayer  *types.PPPlayer
		ProfileXP      int
		ProfileLevel   int
		Achieved       []*types.PPAchievement
		AchievedPlayer map[string]*types.PPPlayerAchievement
	}{
		ProfilePlayer:  profile,
		AchievedPlayer: make(map[string]*types.PPPlayerAchievement),
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
