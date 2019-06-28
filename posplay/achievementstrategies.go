package posplay

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/thoas/go-funk"

	"github.com/underlx/disturbancesmlx/dataobjects"
	"github.com/underlx/disturbancesmlx/website"
)

func init() {
	dataobjects.RegisterPPAchievementStrategy(new(ReachLevelAchievementStrategy))
	dataobjects.RegisterPPAchievementStrategy(new(DiscordEventParticipationAchievementStrategy))
	dataobjects.RegisterPPAchievementStrategy(new(VisitStationsAchievementStrategy))
	dataobjects.RegisterPPAchievementStrategy(new(VisitThroughoutLineAchievementStrategy))
	dataobjects.RegisterPPAchievementStrategy(new(SubmitAchievementStrategy))
	dataobjects.RegisterPPAchievementStrategy(new(TripDuringDisturbanceAchievementStrategy))
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
// If total == -1: this achievement is still locked for the user, show a censured version
// If total < -1: this achievement is still locked for the user, do not show it at all
func (s *StubAchievementStrategy) Progress(context *dataobjects.PPAchievementContext) (current, total int, err error) {
	return 0, -2, nil
}

// ProgressHTML implements dataobjects.PPAchievementStrategy
func (s *StubAchievementStrategy) ProgressHTML(context *dataobjects.PPAchievementContext) string {
	return ""
}

// CriteriaHTML implements dataobjects.PPAchievementStrategy
// context.Player may be nil when calling this function
func (s *StubAchievementStrategy) CriteriaHTML(context *dataobjects.PPAchievementContext) string {
	return ""
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
	} else if curLevel < achievementLevel-10 {
		// lock achievement
		achievementLevel = -2
	}
	return curLevel, achievementLevel, nil
}

// ProgressHTML implements dataobjects.PPAchievementStrategy
func (s *ReachLevelAchievementStrategy) ProgressHTML(context *dataobjects.PPAchievementContext) string {
	var config map[string]interface{}
	context.Achievement.UnmarshalConfig(&config)
	achievementLevel := int(config["level"].(float64))

	tx, err := context.Node.Beginx()
	if err != nil {
		return ""
	}
	defer tx.Commit() // read-only tx

	existingData, err := context.Player.Achievement(tx, context.Achievement.ID)
	if err == nil && existingData.Achieved {
		return ""
	}

	curXP, present := context.StrategyOwnCache.Load(context.Player.DiscordID)
	if !present {
		curXP, err = context.Player.XPBalance(tx)
		if err != nil {
			return ""
		}

		context.StrategyOwnCache.Store(context.Player.DiscordID, curXP.(int))
	}

	remaining := dataobjects.PosPlayLevelToXP(achievementLevel) - curXP.(int)

	return fmt.Sprintf("Falta-lhe ganhar %d XP para alcançar esta proeza.", remaining)
}

// CriteriaHTML implements dataobjects.PPAchievementStrategy
func (s *ReachLevelAchievementStrategy) CriteriaHTML(context *dataobjects.PPAchievementContext) string {
	var config map[string]interface{}
	context.Achievement.UnmarshalConfig(&config)
	achievementLevel := int(config["level"].(float64))

	return fmt.Sprintf(`
	<ul>
		<li>Ter um nível no PosPlay igual ou superior a %d
			<ul>
				<li>Equivalente a ter um total de XP igual ou superior a %d</li>
			</ul>
		</li>
	</ul>`, achievementLevel, dataobjects.PosPlayLevelToXP(achievementLevel))
}

// DiscordEventParticipationAchievementStrategy is an achievement strategy that rewards users when they participate in a set number of Discord events
type DiscordEventParticipationAchievementStrategy struct {
	StubAchievementStrategy
}

// ID returns the ID for this PPAchievementStrategy
func (s *DiscordEventParticipationAchievementStrategy) ID() string {
	return "discord_event_participation"
}

// HandleXPTransaction implements dataobjects.PPAchievementStrategy
func (s *DiscordEventParticipationAchievementStrategy) HandleXPTransaction(context *dataobjects.PPAchievementContext, transaction *dataobjects.PPXPTransaction, actualValueDiff int) error {
	var config map[string]interface{}
	context.Achievement.UnmarshalConfig(&config)
	includeReaction := true
	includeChallenge := true
	if config["eventType"] != nil {
		switch config["eventType"].(string) {
		case "reaction":
			if transaction.Type != "DISCORD_REACTION_EVENT" {
				return nil
			}
			includeChallenge = false
		case "challenge":
			if transaction.Type != "DISCORD_CHALLENGE_EVENT" {
				return nil
			}
			includeReaction = false
		}
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

	count := 0
	if includeReaction {
		c, err := context.Player.CountXPTransactionsWithType(tx, "DISCORD_REACTION_EVENT")
		if err != nil {
			return err
		}
		count += c
	}
	if includeChallenge {
		c, err := context.Player.CountXPTransactionsWithType(tx, "DISCORD_CHALLENGE_EVENT")
		if err != nil {
			return err
		}
		count += c
	}

	achievementCount := int(config["count"].(float64))

	if count >= achievementCount {
		// ensure the achievement reward tx always appears after the cause by adding 1 ms
		err = achieveAchievement(tx, context.Player, context.Achievement, transaction.Time.Add(1*time.Millisecond))
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

// Progress implements dataobjects.PPAchievementStrategy
func (s *DiscordEventParticipationAchievementStrategy) Progress(context *dataobjects.PPAchievementContext) (current, total int, err error) {
	var config map[string]interface{}
	context.Achievement.UnmarshalConfig(&config)
	includeReaction := true
	includeChallenge := true
	if config["eventType"] != nil {
		switch config["eventType"].(string) {
		case "reaction":
			includeChallenge = false
		case "challenge":
			includeReaction = false
		}
	}

	tx, err := context.Node.Beginx()
	if err != nil {
		return 0, 0, err
	}
	defer tx.Commit() // read-only tx

	count := 0
	if includeReaction {
		c, err := context.Player.CountXPTransactionsWithType(tx, "DISCORD_REACTION_EVENT")
		if err != nil {
			return 0, 0, err
		}
		count += c
	}
	if includeChallenge {
		c, err := context.Player.CountXPTransactionsWithType(tx, "DISCORD_CHALLENGE_EVENT")
		if err != nil {
			return 0, 0, err
		}
		count += c
	}

	achievementCount := int(config["count"].(float64))

	if count > achievementCount {
		return count, achievementCount, nil
	} else if count < achievementCount-50 {
		// lock achievement
		achievementCount = -2
	}
	return count, achievementCount, nil
}

// ProgressHTML implements dataobjects.PPAchievementStrategy
func (s *DiscordEventParticipationAchievementStrategy) ProgressHTML(context *dataobjects.PPAchievementContext) string {
	var config map[string]interface{}
	context.Achievement.UnmarshalConfig(&config)
	achievementCount := int(config["count"].(float64))

	includeReaction := true
	includeChallenge := true
	if config["eventType"] != nil {
		switch config["eventType"].(string) {
		case "reaction":
			includeChallenge = false
		case "challenge":
			includeReaction = false
		}
	}

	tx, err := context.Node.Beginx()
	if err != nil {
		return ""
	}
	defer tx.Commit() // read-only tx

	existingData, err := context.Player.Achievement(tx, context.Achievement.ID)
	if err == nil && existingData.Achieved {
		return ""
	}

	count := 0
	if includeReaction {
		c, err := context.Player.CountXPTransactionsWithType(tx, "DISCORD_REACTION_EVENT")
		if err != nil {
			return ""
		}
		count += c
	}
	if includeChallenge {
		c, err := context.Player.CountXPTransactionsWithType(tx, "DISCORD_CHALLENGE_EVENT")
		if err != nil {
			return ""
		}
		count += c
	}

	remaining := achievementCount - count

	plural := "s"
	if remaining == 1 {
		plural = ""
	}
	if includeChallenge && includeReaction {
		return fmt.Sprintf("Falta-lhe participar em %d evento%s ou desafio%s para alcançar esta proeza.", remaining, plural, plural)
	} else if includeChallenge {
		return fmt.Sprintf("Falta-lhe participar em %d desafio%s para alcançar esta proeza.", remaining, plural)
	} else {
		return fmt.Sprintf("Falta-lhe participar em %d evento%s para alcançar esta proeza.", remaining, plural)
	}
}

// CriteriaHTML implements dataobjects.PPAchievementStrategy
func (s *DiscordEventParticipationAchievementStrategy) CriteriaHTML(context *dataobjects.PPAchievementContext) string {
	var config map[string]interface{}
	context.Achievement.UnmarshalConfig(&config)
	achievementCount := int(config["count"].(float64))

	typeSingular := "evento ou desafio"
	typePlural := "eventos ou desafios"
	if config["eventType"] != nil {
		switch config["eventType"].(string) {
		case "reaction":
			typeSingular = "evento"
			typePlural = "eventos"
		case "challenge":
			typeSingular = "desafio"
			typePlural = "desafios"
		}
	}

	t := typePlural
	if achievementCount == 1 {
		t = typeSingular
	}

	return fmt.Sprintf(`
	<ul>
		<li>Participar em %d %s no servidor de Discord do UnderLX;</li>
		<li>Terá de participar nos %s antes da sua hora de fecho.</li>
	</ul>`, achievementCount, t, typePlural)
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
			station, err := dataobjects.GetStation(tx, x)
			if err != nil {
				return false
			}
			closed, err := station.Closed(context.Node)
			if err != nil {
				return false
			}
			return !funk.ContainsString(extra.VisitedStations, x) && !closed
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

	config.Stations = funk.FilterString(config.Stations, func(x string) bool {
		station, err := dataobjects.GetStation(tx, x)
		if err != nil {
			return false
		}
		closed, err := station.Closed(tx)
		if err != nil {
			return false
		}
		return !closed
	})

	existingData, err := context.Player.Achievement(tx, context.Achievement.ID)
	if err != nil {
		// no existing data, player still has everything left to do
		return 0, -1, nil
	}
	var extra visitStationsExtra
	existingData.UnmarshalExtra(&extra)

	extra.VisitedStations = funk.FilterString(extra.VisitedStations, func(x string) bool {
		station, err := dataobjects.GetStation(tx, x)
		if err != nil {
			return false
		}
		closed, err := station.Closed(tx)
		if err != nil {
			return false
		}
		return !closed
	})

	if float64(len(extra.VisitedStations))/float64(len(config.Stations)) < 0.3 {
		// lock achievement
		return len(extra.VisitedStations), -1, nil
	}

	return len(extra.VisitedStations), len(config.Stations), nil
}

// ProgressHTML implements dataobjects.PPAchievementStrategy
func (s *VisitStationsAchievementStrategy) ProgressHTML(context *dataobjects.PPAchievementContext) string {
	var config visitStationsConfig
	context.Achievement.UnmarshalConfig(&config)
	if config.SingleTrip {
		return ""
	}

	tx, err := context.Node.Beginx()
	if err != nil {
		return ""
	}
	defer tx.Commit() // read-only tx

	existingData, err := context.Player.Achievement(tx, context.Achievement.ID)
	if err != nil {
		// no existing data, player still has everything left to do
		return ""
	}
	var extra visitStationsExtra
	existingData.UnmarshalExtra(&extra)

	if len(extra.VisitedStations) == 0 || len(extra.VisitedStations) == len(config.Stations) {
		return ""
	}

	result := "Falta-lhe visitar as seguintes estações: <ul>"
	stationsLeftToVisit := funk.FilterString(config.Stations, func(x string) bool {
		return !funk.ContainsString(extra.VisitedStations, x)
	})
	stations := []*dataobjects.Station{}
	for _, s := range stationsLeftToVisit {
		station, err := dataobjects.GetStation(context.Node, s)
		if err != nil {
			continue
		}
		if closed, err := station.Closed(context.Node); err == nil && closed {
			continue
		}
		stations = append(stations, station)
	}
	sort.Slice(stations, func(i, j int) bool {
		return stations[i].Name < stations[j].Name
	})

	for _, station := range stations {
		result += "<li><a href=\"" + website.BaseURL() + "/s/" + station.ID + "\">" + station.Name + "</a></li>"
	}

	result += "</ul>"
	return result
}

// CriteriaHTML implements dataobjects.PPAchievementStrategy
func (s *VisitStationsAchievementStrategy) CriteriaHTML(context *dataobjects.PPAchievementContext) string {
	var config visitStationsConfig
	context.Achievement.UnmarshalConfig(&config)

	var result string
	if config.SingleTrip {
		result = "<ul><li>Visitar as seguintes estações numa só viagem:<ul>"
	} else {
		result = "<ul><li>Visitar as seguintes estações:<ul>"
	}

	stations := []*dataobjects.Station{}
	for _, s := range config.Stations {
		station, err := dataobjects.GetStation(context.Node, s)
		if err != nil {
			continue
		}
		if closed, err := station.Closed(context.Node); err == nil && closed {
			continue
		}
		stations = append(stations, station)
	}
	sort.Slice(stations, func(i, j int) bool {
		return stations[i].Name < stations[j].Name
	})

	for _, station := range stations {
		result += "<li><a href=\"" + website.BaseURL() + "/s/" + station.ID + "\">" + station.Name + "</a></li>"
	}

	result += "</ul></li>"
	if config.SingleTrip {
		result += "<li>A viagem terá que ser registada e submetida pela aplicação UnderLX associada à sua conta do PosPlay;</li>"
	} else {
		result += "<li>As estações podem ser visitadas ao longo de várias viagens separadas;</li>"
		result += "<li>A viagem ou viagens terão que ser registadas e submetidas pela aplicação UnderLX associada à sua conta do PosPlay;</li>"
	}
	result += "<li>Estações incluídas em correções manuais das viagens não irão contar para o progresso desta proeza.</li></ul>"

	return result
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
	for _, use := range trip.StationUses {
		if !use.Manual {
			haystack += use.Station.ID + "|"
		}
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

// CriteriaHTML implements dataobjects.PPAchievementStrategy
func (s *VisitThroughoutLineAchievementStrategy) CriteriaHTML(context *dataobjects.PPAchievementContext) string {
	var config visitThroughoutLineConfig
	context.Achievement.UnmarshalConfig(&config)

	var result string
	var line *dataobjects.Line
	if config.Line == "*" {
		result = "<ul><li>Percorrer qualquer linha"
	} else {
		line, _ = dataobjects.GetLine(context.Node, config.Line)
		result = "<ul><li>Percorrer a linha <a href=\"" + website.BaseURL() + "/l/" + line.ID + "\">" + line.Name + "</a>"
	}
	switch config.Direction {
	case "*":
		result += " em qualquer sentido"
	case "up", "ascending":
		if line != nil {
			stations, _ := line.Stations(context.Node)
			result += fmt.Sprintf(" no sentido %s → %s", stations[0].Name, stations[len(stations)-1].Name)
		} else {
			result += " no sentido ascendente"
		}
	case "down", "descending":
		if line != nil {
			stations, _ := line.Stations(context.Node)
			result += fmt.Sprintf(" no sentido %s → %s", stations[len(stations)-1].Name, stations[0].Name)
		} else {
			result += " no sentido descendente"
		}
	}
	result += " de uma ponta à outra;</li>"
	result += "<li>O percurso terá que ser realizado numa só viagem;</li>"
	result += "<li>A viagem terá que ser registada e submetida pela aplicação UnderLX associada à sua conta do PosPlay;</li>"
	result += "<li>Estações encerradas por tempo indeterminado não são tidas em conta;</li>"
	result += "<li>Estações incluídas numa correção manual da viagem não irão contar para o progresso desta proeza.</li></ul>"
	return result
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

// CriteriaHTML implements dataobjects.PPAchievementStrategy
func (s *SubmitAchievementStrategy) CriteriaHTML(context *dataobjects.PPAchievementContext) string {
	var config submitConfig
	context.Achievement.UnmarshalConfig(&config)

	result := "<ul>"
	switch config.Type {
	case "trip":
		result += "<li>Submeter um registo de viagem com duas ou mais estações;</li>"
	case "trip_edit":
		result += "<li>Submeter uma correção ou confirmação de registo de viagem;</li>"
	case "disturbance_report":
		result += "<li>Submeter uma comunicação de problemas na circulação;</li>"
	}
	result += "<li>A submissão terá que ser feita usando a aplicação UnderLX associada à sua conta do PosPlay.</li></ul>"
	return result
}

// TripDuringDisturbanceAchievementStrategy is an achievement strategy that rewards users when they submit a trip performed during a disturbance
type TripDuringDisturbanceAchievementStrategy struct {
	StubAchievementStrategy
}

type tripDuringDisturbanceConfig struct {
	Line         string `json:"line"`         // line ID or "*", in the latter case Network must be specified
	Network      string `json:"network"`      // network ID or "*"
	OfficialOnly bool   `json:"officialOnly"` // whether only official disturbances count
}

// ID returns the ID for this PPAchievementStrategy
func (s *TripDuringDisturbanceAchievementStrategy) ID() string {
	return "trip_during_disturbance"
}

// Progress implements dataobjects.PPAchievementStrategy
func (s *TripDuringDisturbanceAchievementStrategy) Progress(context *dataobjects.PPAchievementContext) (current, total int, err error) {
	return 0, 0, nil
}

// HandleTrip implements dataobjects.PPAchievementStrategy
func (s *TripDuringDisturbanceAchievementStrategy) HandleTrip(context *dataobjects.PPAchievementContext, trip *dataobjects.Trip) error {
	if len(trip.StationUses) < 2 {
		return nil
	}

	var config tripDuringDisturbanceConfig
	context.Achievement.UnmarshalConfig(&config)

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

	var line *dataobjects.Line
	var network *dataobjects.Network
	if (config.Line == "*" || config.Line == "") && (config.Network != "*" && config.Network != "") {
		network, err = dataobjects.GetNetwork(tx, config.Network)
	} else if config.Line != "*" && config.Line != "" {
		line, err = dataobjects.GetLine(tx, config.Line)
	}
	if err != nil {
		return err
	}

	hadDisturbances := false
	if line != nil {
		disturbances, err := line.DisturbancesBetween(tx, trip.StartTime, trip.EndTime, config.OfficialOnly)
		if err != nil {
			return err
		}
		hadDisturbances = len(disturbances) > 0
	} else if network != nil {
		lines, err := network.Lines(tx)
		if err != nil {
			return err
		}
		for _, line := range lines {
			disturbances, err := line.DisturbancesBetween(tx, trip.StartTime, trip.EndTime, config.OfficialOnly)
			if err != nil {
				return err
			}
			hadDisturbances = len(disturbances) > 0
			if hadDisturbances {
				break
			}
		}
	} else {
		// all lines of any network
		disturbances, err := dataobjects.GetDisturbancesBetween(tx, trip.StartTime, trip.EndTime, config.OfficialOnly)
		if err != nil {
			return err
		}
		hadDisturbances = len(disturbances) > 0
	}

	if !hadDisturbances {
		// not eligible
		return tx.Commit()
	}

	visitedAffected := false

	if line != nil {
		prevUseWasAffected := false
		for _, use := range trip.StationUses {
			lines, err := use.Station.Lines(tx)
			if err != nil {
				return err
			}
			thisUseWasAffected := false
			for _, sline := range lines {
				if line.ID == sline.ID {
					thisUseWasAffected = true
					break
				}
			}
			if thisUseWasAffected && prevUseWasAffected {
				visitedAffected = true
				break
			}
			prevUseWasAffected = thisUseWasAffected
		}
	} else if network != nil {
		prevUseWasAffected := false
		for _, use := range trip.StationUses {
			thisUseWasAffected := use.Station.Network.ID == network.ID

			if thisUseWasAffected && prevUseWasAffected {
				visitedAffected = true
				break
			}
			prevUseWasAffected = thisUseWasAffected
		}
	} else {
		// if it's any line, then it's immediately affected
		visitedAffected = true
	}

	if !visitedAffected {
		// not eligible
		return tx.Commit()
	}

	// ensure the achievement reward tx always appears after the cause by adding 1 ms
	err = achieveAchievement(tx, context.Player, context.Achievement, trip.SubmitTime.Add(1*time.Millisecond))
	if err != nil {
		return err
	}

	return tx.Commit()
}

// CriteriaHTML implements dataobjects.PPAchievementStrategy
func (s *TripDuringDisturbanceAchievementStrategy) CriteriaHTML(context *dataobjects.PPAchievementContext) string {
	var config tripDuringDisturbanceConfig
	context.Achievement.UnmarshalConfig(&config)

	result := "<ul>"
	var line *dataobjects.Line
	var network *dataobjects.Network
	var err error
	if (config.Line == "*" || config.Line == "") && (config.Network != "*" && config.Network != "") {
		network, err = dataobjects.GetNetwork(context.Node, config.Network)
	} else if config.Line != "*" && config.Line != "" {
		line, err = dataobjects.GetLine(context.Node, config.Line)
	}
	if err != nil {
		return ""
	}

	if line != nil {
		result += "<li>Viajar na linha <a href=\"" + website.BaseURL() + "/l/" + line.ID + "\">" + line.Name + "</a> enquanto decorre uma perturbação que a afecte;</li>"
	} else if network != nil {
		result += "<li>Viajar na rede " + network.Name + " enquanto decorre uma perturbação nesta rede;</li>"
	} else {
		result += "<li>Viajar em qualquer linha de qualquer rede enquanto decorre uma perturbação;</li>"
	}

	if config.OfficialOnly {
		result += "<li>Perturbações comunicadas pela comunidade de utilizadores não serão contabilizadas para esta proeza;</li>"
	}

	result += "<li>A viagem terá que ser registada e submetida pela aplicação UnderLX associada à sua conta do PosPlay.</li></ul>"
	return result
}
