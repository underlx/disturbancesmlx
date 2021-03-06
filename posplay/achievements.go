package posplay

import (
	"fmt"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/gbl08ma/sqalx"
	"github.com/underlx/disturbancesmlx/types"
	"github.com/underlx/disturbancesmlx/discordbot"
)

var allAchievements []*types.PPAchievement
var allAchievementsByID map[string]*types.PPAchievement
var allAchievementsMutex sync.RWMutex

// ReloadAchievements reloads the achievement cache
func ReloadAchievements() error {
	tx, err := config.Node.Beginx()
	if err != nil {
		return err
	}
	defer tx.Commit() // read-only tx

	allAchievementsMutex.Lock()
	allAchievementsByID = make(map[string]*types.PPAchievement)
	allAchievements, err = types.GetPPAchievements(tx)
	for _, a := range allAchievements {
		allAchievementsByID[a.ID] = a
	}
	allAchievementsMutex.Unlock()
	return err
}

func achieveAchievement(tx sqalx.Node, player *types.PPPlayer, achievement *types.PPAchievement, achievedTime time.Time) error {
	tx, err := tx.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	pach := types.PPPlayerAchievement{
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

	// notification sending code
	sendDiscordNotif, err := types.GetPPNotificationSetting(tx, player.DiscordID, NotificationTypeAchievementAchieved, NotificationMethodDiscordDM, NotificationDefaults)
	if err != nil {
		return err
	}

	sendAppNotif, err := types.GetPPNotificationSetting(tx, player.DiscordID, NotificationTypeAchievementAchieved, NotificationMethodAppNotif, NotificationDefaults)
	if err != nil {
		return err
	}
	var appNotifPair *types.APIPair
	if sendAppNotif {
		pair, err := types.GetPPPair(tx, player.DiscordID)
		if err != nil {
			// app notification setting enabled, but the user doesn't have an associated app
			sendAppNotif = false
		} else {
			appNotifPair = pair.Pair
		}
	}

	tx.DeferToCommit(func() {
		rewardText := ""
		if achievement.XPReward != 0 {
			rewardText = fmt.Sprintf(" e recebeu %d XP como recompensa", achievement.XPReward)
		}
		if sendDiscordNotif {
			discordbot.SendDMtoUser(uidConvI(player.DiscordID), &discordgo.MessageSend{
				Content: fmt.Sprintf("Acabou de alcançar a proeza \"%s\"%s 👍\n%s",
					achievement.Names[achievement.MainLocale], rewardText, BaseURL()+"/achievements/"+achievement.ID),
			})
		}
		if sendAppNotif {
			data := map[string]string{
				"title": fmt.Sprintf("Alcançou a proeza \"%s\"", achievement.Names[achievement.MainLocale]),
				"body":  fmt.Sprintf("Acabou de alcançar a proeza \"%s\"%s", achievement.Names[achievement.MainLocale], rewardText),
				"url":   BaseURL() + "/achievements/" + achievement.ID,
			}
			config.SendAppNotification(appNotifPair, "posplay-notification", data)
		}
	})
	// end of notification sending code

	return tx.Commit()
}

func forEachAchievementWithIDorPair(tx sqalx.Node, discordID uint64, pairKey string, doFunc func(context *types.PPAchievementContext)) error {
	if discordID == 0 && pairKey != "" {
		// is this submitter even linked with a PosPlay account?
		pair, err := types.GetPPPairForKey(tx, pairKey)
		if err != nil {
			// the answer is no, move on
			return nil
		}
		discordID = pair.DiscordID
	}

	player, err := types.GetPPPlayer(tx, discordID)
	if err != nil {
		return err
	}

	return forEachAchievement(tx, player, doFunc)
}

func forEachAchievement(tx sqalx.Node, player *types.PPPlayer, doFunc func(context *types.PPAchievementContext)) error {
	context := types.PPAchievementContext{
		Node:   tx,
		Player: player,
	}

	cacheMap := make(map[string]*sync.Map)

	allAchievementsMutex.RLock()
	achCopy := make([]*types.PPAchievement, len(allAchievements))
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

	trip, err := types.GetTrip(tx, id)
	if err != nil {
		return err
	}

	forEachAchievementWithIDorPair(tx, 0, trip.Submitter.Key, func(context *types.PPAchievementContext) {
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

	trip, err := types.GetTrip(tx, id)
	if err != nil {
		return err
	}

	forEachAchievementWithIDorPair(tx, 0, trip.Submitter.Key, func(context *types.PPAchievementContext) {
		context.Achievement.Strategy.HandleTripEdit(context, trip)
	})

	return tx.Commit()
}

func processReportForAchievements(report types.Report) error {
	tx, err := config.Node.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	switch r := report.(type) {
	case *types.LineDisturbanceReport:
		forEachAchievementWithIDorPair(tx, 0, report.Submitter().Key, func(context *types.PPAchievementContext) {
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

	xptx, err := types.GetPPXPTransaction(tx, id)
	if err != nil {
		return err
	}

	forEachAchievementWithIDorPair(tx, xptx.DiscordID, "", func(context *types.PPAchievementContext) {
		context.Achievement.Strategy.HandleXPTransaction(context, xptx, actualValueDiff)
	})

	return tx.Commit()
}
