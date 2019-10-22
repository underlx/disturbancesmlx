package posplay

import (
	"fmt"
	"html/template"
	"math"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/underlx/disturbancesmlx/types"
)

func achievementsPage(w http.ResponseWriter, r *http.Request) {
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
	defer tx.Commit() // read-only tx

	player, err := types.GetPPPlayer(tx, uidConvS(session.DiscordInfo.ID))
	if err != nil {
		config.Log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	p := struct {
		pageCommons

		Achieved        []*types.PPAchievement
		NonAchieved     []*types.PPAchievement
		Achieving       map[string]*types.PPPlayerAchievement
		ProgressCurrent map[string]int
		ProgressTotal   map[string]int
		ProgressPct     map[string]int
	}{
		Achieving:       make(map[string]*types.PPPlayerAchievement),
		ProgressCurrent: make(map[string]int),
		ProgressTotal:   make(map[string]int),
		ProgressPct:     make(map[string]int),
	}

	p.pageCommons, err = initPageCommons(tx, w, r, "Proezas", session, player)
	if err != nil {
		config.Log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	p.SidebarSelected = "achievements"

	forEachAchievement(tx, player, func(context *types.PPAchievementContext) {
		current, total, e := context.Achievement.Strategy.Progress(context)
		if e != nil {
			if err == nil {
				err = e
			}
			return
		}
		p.ProgressCurrent[context.Achievement.ID] = current
		p.ProgressTotal[context.Achievement.ID] = total
		if total > 0 {
			p.ProgressPct[context.Achievement.ID] = int(math.Round((float64(current) / float64(total)) * 100))
		}
	})
	if err != nil {
		config.Log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	playerAchieved, err := player.Achievements(tx)
	if err != nil {
		config.Log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	for _, pach := range playerAchieved {
		p.Achieving[pach.Achievement.ID] = pach
	}

	allAchievementsMutex.RLock()
	for _, achievement := range allAchievements {
		if p.Achieving[achievement.ID] != nil && p.Achieving[achievement.ID].Achieved {
			p.Achieved = append(p.Achieved, achievement)
		} else {
			if p.ProgressTotal[achievement.ID] < -1 {
				// this achievement is still locked and shoud not be shown at all
				continue
			}
			p.NonAchieved = append(p.NonAchieved, achievement)
		}
	}
	allAchievementsMutex.RUnlock()

	sort.Slice(p.Achieved, func(i, j int) bool {
		return p.Achieving[p.Achieved[i].ID].AchievedTime.Before(p.Achieving[p.Achieved[j].ID].AchievedTime)
	})
	sort.Slice(p.NonAchieved, func(i, j int) bool {
		// sort by locked status, unlocked first
		if p.ProgressTotal[p.NonAchieved[i].ID] >= 0 && p.ProgressTotal[p.NonAchieved[j].ID] >= 0 {
			// sort by completion, more complete first
			iPct := p.ProgressPct[p.NonAchieved[i].ID]
			jPct := p.ProgressPct[p.NonAchieved[j].ID]
			if iPct == jPct {
				// sort alphabetically
				iName := p.NonAchieved[i].Names[p.NonAchieved[i].MainLocale]
				jName := p.NonAchieved[j].Names[p.NonAchieved[j].MainLocale]
				return strings.Compare(iName, jName) < 0
			}
			return iPct >= jPct
		}
		return p.ProgressTotal[p.NonAchieved[i].ID] >= p.ProgressTotal[p.NonAchieved[j].ID]
	})

	err = webtemplate.ExecuteTemplate(w, "achievements.html", p)
	if err != nil {
		config.Log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
	}
}

func achievementPage(w http.ResponseWriter, r *http.Request) {
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

	achievement, err := types.GetPPAchievement(tx, mux.Vars(r)["id"])
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	var player *types.PPPlayer
	if session != nil {
		player, err = types.GetPPPlayer(tx, uidConvS(session.DiscordInfo.ID))
		if err != nil {
			config.Log.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}

	p := struct {
		pageCommons

		Achievement       *types.PPAchievement
		PlayerAchievement *types.PPPlayerAchievement
		ProgressCurrent   int
		ProgressTotal     int
		ProgressPct       int
		ProgressHTML      template.HTML
		Criteria          template.HTML
		AchievedBy        int
		Rarity            float64
		RarityDescription string
		FirstAchieved     time.Time
		LastAchieved      time.Time
	}{
		Achievement: achievement,
	}

	p.pageCommons, err = initPageCommons(tx, w, r, fmt.Sprintf("Proeza \"%s\"", achievement.Names[achievement.MainLocale]), session, player)
	if err != nil {
		config.Log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	p.UserInfoInHeader = true

	context := &types.PPAchievementContext{
		Node:             tx,
		Achievement:      achievement,
		Player:           player,
		StrategyOwnCache: new(sync.Map),
	}

	if player != nil {
		p.PlayerAchievement, _ = player.Achievement(tx, achievement.ID)
		// if an error is returned the player still has everything left to do, not a problem

		current, total, err := achievement.Strategy.Progress(context)
		if err != nil {
			config.Log.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		p.ProgressCurrent = current
		p.ProgressTotal = total
		if total > 0 {
			p.ProgressPct = int(math.Round((float64(current) / float64(total)) * 100))
		}
		p.ProgressHTML = template.HTML(achievement.Strategy.ProgressHTML(context))
	}

	p.Criteria = template.HTML(achievement.Strategy.CriteriaHTML(context))

	totalUsers, err := types.CountPPPlayers(tx)
	if err != nil {
		config.Log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
	}

	p.AchievedBy, err = achievement.CountAchieved(tx)
	if err != nil {
		config.Log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
	}
	if p.AchievedBy > 0 {
		p.FirstAchieved, err = achievement.FirstAchieved(tx)
		if err != nil {
			config.Log.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
		}
		p.LastAchieved, err = achievement.LastAchieved(tx)
		if err != nil {
			config.Log.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
		}
	}

	p.Rarity = (float64(p.AchievedBy) / float64(totalUsers)) * 100
	p.RarityDescription = DescriptionForRarity(p.Rarity)

	err = webtemplate.ExecuteTemplate(w, "achievement.html", p)
	if err != nil {
		config.Log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
	}
}
