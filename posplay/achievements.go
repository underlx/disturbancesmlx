package posplay

import (
	"sync"
	"time"

	"github.com/gbl08ma/sqalx"
	"github.com/underlx/disturbancesmlx/dataobjects"
)

var allAchievements []*dataobjects.PPAchievement
var allAchievementsByID map[string]*dataobjects.PPAchievement
var allAchievementsMutex sync.RWMutex

// ReloadAchievements reloads the achievement cache
func ReloadAchievements() error {
	tx, err := config.Node.Beginx()
	if err != nil {
		return err
	}
	defer tx.Commit() // read-only tx

	allAchievementsMutex.Lock()
	allAchievementsByID = make(map[string]*dataobjects.PPAchievement)
	allAchievements, err = dataobjects.GetPPAchievements(tx)
	for _, a := range allAchievements {
		allAchievementsByID[a.ID] = a
	}
	allAchievementsMutex.Unlock()
	return err
}

func achieveAchievement(tx sqalx.Node, player *dataobjects.PPPlayer, achievement *dataobjects.PPAchievement, achievedTime time.Time) error {
	tx, err := tx.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	pach := dataobjects.PPPlayerAchievement{
		DiscordID:    player.DiscordID,
		Achievement:  achievement,
		Achieved:     true,
		AchievedTime: achievedTime,
	}
	err = pach.Update(tx)
	if err != nil {
		return err
	}

	if achievement.XPReward != 0 {
		err = DoXPTransaction(tx, player, pach.AchievedTime, achievement.XPReward, "ACHIEVEMENT_REWARD", map[string]interface{}{
			"achievement_id": achievement.ID,
		}, false)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func forEachAchievementWithIDorPair(tx sqalx.Node, discordID uint64, pairKey string, doFunc func(context *dataobjects.PPAchievementContext)) error {
	if discordID == 0 && pairKey != "" {
		// is this submitter even linked with a PosPlay account?
		pair, err := dataobjects.GetPPPairForKey(tx, pairKey)
		if err != nil {
			// the answer is no, move on
			return nil
		}
		discordID = pair.DiscordID
	}

	player, err := dataobjects.GetPPPlayer(tx, discordID)
	if err != nil {
		return err
	}

	return forEachAchievement(tx, player, doFunc)
}

func forEachAchievement(tx sqalx.Node, player *dataobjects.PPPlayer, doFunc func(context *dataobjects.PPAchievementContext)) error {
	context := dataobjects.PPAchievementContext{
		Node:   tx,
		Player: player,
	}

	cacheMap := make(map[string]*sync.Map)

	allAchievementsMutex.RLock()
	achCopy := make([]*dataobjects.PPAchievement, len(allAchievements))
	copy(achCopy, allAchievements)
	allAchievementsMutex.RUnlock()

	for _, achievement := range achCopy {
		context.Achievement = achievement
		var present bool
		context.StrategyOwnCache, present = cacheMap[achievement.Strategy.ID()]
		if !present {
			context.StrategyOwnCache = new(sync.Map)
			cacheMap[achievement.Strategy.ID()] = context.StrategyOwnCache
		}
		doFunc(&context)
	}

	return nil
}

func processTripForAchievements(id string) error {
	tx, err := config.Node.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	trip, err := dataobjects.GetTrip(tx, id)
	if err != nil {
		return err
	}

	forEachAchievementWithIDorPair(tx, 0, trip.Submitter.Key, func(context *dataobjects.PPAchievementContext) {
		context.Achievement.Strategy.HandleTrip(context, trip)
	})

	return tx.Commit()
}

func processTripEditForAchievements(id string) error {
	tx, err := config.Node.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	trip, err := dataobjects.GetTrip(tx, id)
	if err != nil {
		return err
	}

	forEachAchievementWithIDorPair(tx, 0, trip.Submitter.Key, func(context *dataobjects.PPAchievementContext) {
		context.Achievement.Strategy.HandleTripEdit(context, trip)
	})

	return tx.Commit()
}

func processReportForAchievements(report dataobjects.Report) error {
	tx, err := config.Node.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	switch r := report.(type) {
	case *dataobjects.LineDisturbanceReport:
		forEachAchievementWithIDorPair(tx, 0, report.Submitter().Key, func(context *dataobjects.PPAchievementContext) {
			context.Achievement.Strategy.HandleDisturbanceReport(context, r)
		})
	}

	return tx.Commit()
}

func processXPTxForAchievements(id string, actualValueDiff int) error {
	tx, err := config.Node.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	xptx, err := dataobjects.GetPPXPTransaction(tx, id)
	if err != nil {
		return err
	}

	forEachAchievementWithIDorPair(tx, xptx.DiscordID, "", func(context *dataobjects.PPAchievementContext) {
		context.Achievement.Strategy.HandleXPTransaction(context, xptx, actualValueDiff)
	})

	return tx.Commit()
}
