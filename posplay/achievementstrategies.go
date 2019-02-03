package posplay

import (
	"strings"
	"time"

	"github.com/thoas/go-funk"

	"github.com/underlx/disturbancesmlx/dataobjects"
)

func init() {
	dataobjects.RegisterPPAchievementStrategy(new(ReachLevelAchievementStrategy))
	dataobjects.RegisterPPAchievementStrategy(new(VisitStationsAchievementStrategy))
	dataobjects.RegisterPPAchievementStrategy(new(VisitThroughoutLineAchievementStrategy))
	dataobjects.RegisterPPAchievementStrategy(new(SubmitAchievementStrategy))
}

// StubAchievementStrategy partially implements dataobjects.PPAchievementStrategy
type StubAchievementStrategy struct{}

// ID is not implemented on purpose (to force strategies to specify theirs)

// HandleTrip implements dataobjects.PPAchievementStrategy
func (s *StubAchievementStrategy) HandleTrip(context *dataobjects.PPAchievementContext, trip *dataobjects.Trip) error {
	return nil
}

// HandleTripEdit implements dataobjects.PPAchievementStrategy
func (s *StubAchievementStrategy) HandleTripEdit(context *dataobjects.PPAchievementContext, trip *dataobjects.Trip) error {
	return nil
}

// HandleDisturbanceReport implements dataobjects.PPAchievementStrategy
func (s *StubAchievementStrategy) HandleDisturbanceReport(context *dataobjects.PPAchievementContext, report *dataobjects.LineDisturbanceReport) error {
	return nil
}

// HandleXPTransaction implements dataobjects.PPAchievementStrategy
func (s *StubAchievementStrategy) HandleXPTransaction(context *dataobjects.PPAchievementContext, transaction *dataobjects.PPXPTransaction, actualValueDiff int) error {
	return nil
}

// Progress implements dataobjects.PPAchievementStrategy
// If total == 0: this achievement has no progress, it's "all or nothing"
// If total < 0: this achievement is still locked for the user
func (s *StubAchievementStrategy) Progress(context *dataobjects.PPAchievementContext) (current, total int, err error) {
	return 0, -1, nil
}

// ReachLevelAchievementStrategy is an achievement strategy that rewards users when they reach a specified level
type ReachLevelAchievementStrategy struct {
	StubAchievementStrategy
}

// ID returns the ID for this PPAchievementStrategy
func (s *ReachLevelAchievementStrategy) ID() string {
	return "reach_level"
}

// HandleXPTransaction implements dataobjects.PPAchievementStrategy
func (s *ReachLevelAchievementStrategy) HandleXPTransaction(context *dataobjects.PPAchievementContext, transaction *dataobjects.PPXPTransaction, actualValueDiff int) error {
	tx, err := context.Node.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	existingData, err := context.Player.Achievement(tx, context.Achievement.ID)
	if err == nil && existingData.Achieved {
		// the player already has this achievement
		return tx.Commit()
	}

	curXP, err := context.Player.XPBalanceBetween(tx, context.Player.Joined, transaction.Time)
	if err != nil {
		return err
	}

	prevXP := curXP - actualValueDiff

	curLevel, _ := dataobjects.PosPlayPlayerLevel(curXP)
	prevLevel, _ := dataobjects.PosPlayPlayerLevel(prevXP)

	var config map[string]interface{}
	context.Achievement.UnmarshalConfig(&config)
	achievementLevel := int(config["level"].(float64))

	if curLevel > prevLevel && prevLevel < achievementLevel && curLevel >= achievementLevel {
		// ensure the achievement reward tx always appears after the cause by adding 1 ms
		err = achieveAchievement(tx, context.Player, context.Achievement, transaction.Time.Add(1*time.Millisecond))
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

// Progress implements dataobjects.PPAchievementStrategy
func (s *ReachLevelAchievementStrategy) Progress(context *dataobjects.PPAchievementContext) (current, total int, err error) {
	var config map[string]interface{}
	context.Achievement.UnmarshalConfig(&config)
	achievementLevel := int(config["level"].(float64))

	curXP, present := context.StrategyOwnCache.Load(context.Player.DiscordID)
	if !present {
		tx, err := context.Node.Beginx()
		if err != nil {
			return 0, 0, err
		}
		defer tx.Commit() // read-only tx

		curXP, err = context.Player.XPBalance(tx)
		if err != nil {
			return 0, 0, err
		}

		context.StrategyOwnCache.Store(context.Player.DiscordID, curXP.(int))
	}
	curLevel, _ := dataobjects.PosPlayPlayerLevel(curXP.(int))

	if curLevel > achievementLevel {
		curLevel = achievementLevel
	}
	return curLevel, achievementLevel, nil
}

// VisitStationsAchievementStrategy is an achievement strategy that rewards users when they visit certain stations
type VisitStationsAchievementStrategy struct {
	StubAchievementStrategy
}

type visitStationsConfig struct {
	Stations   []string `json:"stations"`
	SingleTrip bool     `json:"singleTrip"`
}

type visitStationsExtra struct {
	VisitedStations []string `json:"visitedStations"`
}

// ID returns the ID for this PPAchievementStrategy
func (s *VisitStationsAchievementStrategy) ID() string {
	return "visit_stations"
}

// HandleTrip implements dataobjects.PPAchievementStrategy
func (s *VisitStationsAchievementStrategy) HandleTrip(context *dataobjects.PPAchievementContext, trip *dataobjects.Trip) error {
	tx, err := context.Node.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	existingData, err := context.Player.Achievement(tx, context.Achievement.ID)
	if err == nil && existingData.Achieved {
		// the player already has this achievement
		return tx.Commit()
	}

	var config visitStationsConfig
	context.Achievement.UnmarshalConfig(&config)

	stationsLeftToVisit := config.Stations
	if !config.SingleTrip && existingData != nil {
		var extra visitStationsExtra
		existingData.UnmarshalExtra(&extra)

		stationsLeftToVisit = funk.FilterString(stationsLeftToVisit, func(x string) bool {
			return !funk.ContainsString(extra.VisitedStations, x)
		})
	}

	visitedInThisTrip := make(map[string]bool)
	for _, use := range trip.StationUses {
		if !use.Manual {
			visitedInThisTrip[use.Station.ID] = true
		}
	}

	stationsLeftToVisit = funk.FilterString(stationsLeftToVisit, func(x string) bool {
		return !visitedInThisTrip[x]
	})

	if !config.SingleTrip {
		// store progress back
		extra := visitStationsExtra{}
		extra.VisitedStations = funk.FilterString(config.Stations, func(x string) bool {
			return !funk.ContainsString(stationsLeftToVisit, x)
		})
		if existingData == nil {
			existingData = &dataobjects.PPPlayerAchievement{
				DiscordID:   context.Player.DiscordID,
				Achievement: context.Achievement,
			}
		}
		err = existingData.MarshalExtra(extra)
		if err != nil {
			return err
		}
		err = existingData.Update(tx)
		if err != nil {
			return err
		}
	}

	if len(stationsLeftToVisit) == 0 {
		// ensure the achievement reward tx always appears after the cause by adding 1 ms
		err = achieveAchievement(tx, context.Player, context.Achievement, trip.SubmitTime.Add(1*time.Millisecond))
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

// Progress implements dataobjects.PPAchievementStrategy
func (s *VisitStationsAchievementStrategy) Progress(context *dataobjects.PPAchievementContext) (current, total int, err error) {
	tx, err := context.Node.Beginx()
	if err != nil {
		return 0, -1, err
	}
	defer tx.Commit() // read-only tx

	var config visitStationsConfig
	context.Achievement.UnmarshalConfig(&config)

	if config.SingleTrip {
		// achievements configured like this are all-or-nothing
		return 0, 0, nil
	}

	existingData, err := context.Player.Achievement(tx, context.Achievement.ID)
	if err != nil {
		// no existing data, player still has everything left to do
		return 0, len(config.Stations), nil
	}
	var extra visitStationsExtra
	existingData.UnmarshalExtra(&extra)

	return len(extra.VisitedStations), len(config.Stations), nil
}

// VisitThroughoutLineAchievementStrategy is an achievement strategy that rewards users when they visit one line from one end to another in a single trip
type VisitThroughoutLineAchievementStrategy struct {
	StubAchievementStrategy
}

type visitThroughoutLineConfig struct {
	Line      string `json:"line"`      // line ID or "*"
	Direction string `json:"direction"` // "ascending", "descending" or "*"
}

// ID returns the ID for this PPAchievementStrategy
func (s *VisitThroughoutLineAchievementStrategy) ID() string {
	return "visit_throughout_line"
}

// HandleTrip implements dataobjects.PPAchievementStrategy
func (s *VisitThroughoutLineAchievementStrategy) HandleTrip(context *dataobjects.PPAchievementContext, trip *dataobjects.Trip) error {
	tx, err := context.Node.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	existingData, err := context.Player.Achievement(tx, context.Achievement.ID)
	if err == nil && existingData.Achieved {
		// the player already has this achievement
		return tx.Commit()
	}

	var config visitThroughoutLineConfig
	context.Achievement.UnmarshalConfig(&config)

	var possibleLines []*dataobjects.Line
	if config.Line == "*" {
		possibleLines, err = dataobjects.GetLines(tx)
		if err != nil {
			return err
		}
	} else {
		line, err := dataobjects.GetLine(tx, config.Line)
		if err != nil {
			return err
		}
		possibleLines = []*dataobjects.Line{line}
	}
	possibleNeedles := []string{}
	for _, line := range possibleLines {
		stations, err := line.Stations(tx)
		if err != nil {
			return err
		}
		closed := make(map[string]bool)
		for _, station := range stations {
			closed[station.ID], err = station.Closed(tx)
			if err != nil {
				return err
			}
		}

		switch config.Direction {
		case "ascending", "up", "*":
			needle := ""
			for _, station := range stations {
				if !closed[station.ID] {
					needle += station.ID + "|"
				}
			}
			possibleNeedles = append(possibleNeedles, needle)
			if config.Direction == "*" {
				goto descending
			}
		descending:
			fallthrough
		case "descending", "down":
			stations = funk.Reverse(stations).([]*dataobjects.Station)
			needle := ""
			for _, station := range stations {
				if !closed[station.ID] {
					needle += station.ID + "|"
				}
			}
			possibleNeedles = append(possibleNeedles, needle)
		}
	}

	haystack := ""
	for _, uses := range trip.StationUses {
		haystack += uses.Station.ID + "|"
	}

	found := false
	for _, needle := range possibleNeedles {
		if strings.Contains(haystack, needle) {
			found = true
			break
		}
	}

	if found {
		// ensure the achievement reward tx always appears after the cause by adding 1 ms
		err = achieveAchievement(tx, context.Player, context.Achievement, trip.SubmitTime.Add(1*time.Millisecond))
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

// Progress implements dataobjects.PPAchievementStrategy
func (s *VisitThroughoutLineAchievementStrategy) Progress(context *dataobjects.PPAchievementContext) (current, total int, err error) {
	return 0, 0, nil
}

// SubmitAchievementStrategy is an achievement strategy that rewards users when they submit a trip, trip edit, or disturbance report
type SubmitAchievementStrategy struct {
	StubAchievementStrategy
}

type submitConfig struct {
	Type string `json:"type"` // "trip", "trip_edit" or "disturbance_report"
}

// ID returns the ID for this PPAchievementStrategy
func (s *SubmitAchievementStrategy) ID() string {
	return "submit"
}

// HandleTrip implements dataobjects.PPAchievementStrategy
func (s *SubmitAchievementStrategy) HandleTrip(context *dataobjects.PPAchievementContext, trip *dataobjects.Trip) error {
	var config submitConfig
	context.Achievement.UnmarshalConfig(&config)
	if config.Type != "trip" {
		return nil
	}

	tx, err := context.Node.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	existingData, err := context.Player.Achievement(tx, context.Achievement.ID)
	if err == nil && existingData.Achieved {
		// the player already has this achievement
		return tx.Commit()
	}

	if len(trip.StationUses) > 1 {
		// ensure the achievement reward tx always appears after the cause by adding 1 ms
		err = achieveAchievement(tx, context.Player, context.Achievement, trip.SubmitTime.Add(1*time.Millisecond))
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

// HandleTripEdit implements dataobjects.PPAchievementStrategy
func (s *SubmitAchievementStrategy) HandleTripEdit(context *dataobjects.PPAchievementContext, trip *dataobjects.Trip) error {
	var config submitConfig
	context.Achievement.UnmarshalConfig(&config)
	if config.Type != "trip_edit" {
		return nil
	}

	tx, err := context.Node.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	existingData, err := context.Player.Achievement(tx, context.Achievement.ID)
	if err == nil && existingData.Achieved {
		// the player already has this achievement
		return tx.Commit()
	}

	if len(trip.StationUses) > 1 {
		// ensure the achievement reward tx always appears after the cause by adding 1 ms
		err = achieveAchievement(tx, context.Player, context.Achievement, trip.SubmitTime.Add(1*time.Millisecond))
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

// HandleDisturbanceReport implements dataobjects.PPAchievementStrategy
func (s *SubmitAchievementStrategy) HandleDisturbanceReport(context *dataobjects.PPAchievementContext, report *dataobjects.LineDisturbanceReport) error {
	var config submitConfig
	context.Achievement.UnmarshalConfig(&config)
	if config.Type != "disturbance_report" {
		return nil
	}

	tx, err := context.Node.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	existingData, err := context.Player.Achievement(tx, context.Achievement.ID)
	if err == nil && existingData.Achieved {
		// the player already has this achievement
		return tx.Commit()
	}

	err = achieveAchievement(tx, context.Player, context.Achievement, report.Time())
	if err != nil {
		return err
	}

	return tx.Commit()
}
