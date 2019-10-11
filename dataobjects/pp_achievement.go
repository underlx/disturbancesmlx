package dataobjects

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/gbl08ma/sqalx"
	sq "github.com/Masterminds/squirrel"
	"github.com/lib/pq"
)

// PPAchievementContext contains the context necessary to process an achievement-producing event
type PPAchievementContext struct {
	Node             sqalx.Node
	Achievement      *PPAchievement
	Player           *PPPlayer
	StrategyOwnCache *sync.Map
}

// PPAchievementStrategy is a strategy/driver for handing a PosPlay achievement
type PPAchievementStrategy interface {
	ID() string
	HandleTrip(context *PPAchievementContext, trip *Trip) error
	HandleTripEdit(context *PPAchievementContext, trip *Trip) error
	HandleDisturbanceReport(context *PPAchievementContext, report *LineDisturbanceReport) error
	HandleXPTransaction(context *PPAchievementContext, transaction *PPXPTransaction, actualValueDiff int) error
	Progress(context *PPAchievementContext) (current, total int, err error)
	ProgressHTML(context *PPAchievementContext) string
	CriteriaHTML(context *PPAchievementContext) string
}

var achievementStrategies sync.Map

// RegisterPPAchievementStrategy makes this package aware of an available achievement strategy
// All PPAchievements that can be possibly found in the database must have a corresponding strategy
func RegisterPPAchievementStrategy(s PPAchievementStrategy) {
	achievementStrategies.Store(s.ID(), s)
}

// UnregisterPPAchievementStrategy makes this package unaware of a PPAchievementStrategy
func UnregisterPPAchievementStrategy(s PPAchievementStrategy) {
	achievementStrategies.Delete(s.ID())
}

func getAchievementStrategy(id string) PPAchievementStrategy {
	value, ok := achievementStrategies.Load(id)
	if !ok {
		return nil
	}
	return value.(PPAchievementStrategy)
}

// PPAchievement is a PosPlay achievement
type PPAchievement struct {
	ID           string
	Strategy     PPAchievementStrategy
	Config       string
	MainLocale   string
	Names        map[string]string
	Descriptions map[string]string
	Icon         string
	XPReward     int
}

// GetPPAchievements returns a slice with all registered transactions
func GetPPAchievements(node sqalx.Node) ([]*PPAchievement, error) {
	return getPPAchievementsWithSelect(node, sdb.Select())
}

func getPPAchievementsWithSelect(node sqalx.Node, sbuilder sq.SelectBuilder) ([]*PPAchievement, error) {
	achievements := []*PPAchievement{}
	achievementMap := make(map[string]*PPAchievement)

	tx, err := node.Beginx()
	if err != nil {
		return achievements, err
	}
	defer tx.Commit() // read-only tx

	rows, err := sbuilder.Columns("pp_achievement.id", "pp_achievement.strategy",
		"pp_achievement.config", "pp_achievement.icon", "pp_achievement.xp_reward").
		From("pp_achievement").
		RunWith(tx).Query()
	if err != nil {
		return achievements, fmt.Errorf("getPPAchievementsWithSelect: %s", err)
	}
	defer rows.Close()

	for rows.Next() {
		var achievement PPAchievement
		var strategyID string
		err := rows.Scan(
			&achievement.ID,
			&strategyID,
			&achievement.Config,
			&achievement.Icon,
			&achievement.XPReward)
		if err != nil {
			return achievements, fmt.Errorf("getPPAchievementsWithSelect: %s", err)
		}
		achievement.Strategy = getAchievementStrategy(strategyID)
		if achievement.Strategy == nil {
			return achievements, fmt.Errorf("getPPAchievementsWithSelect: strategy with ID %s not found", strategyID)
		}
		achievements = append(achievements, &achievement)
		achievementMap[achievement.ID] = &achievement
		achievementMap[achievement.ID].Names = make(map[string]string)
		achievementMap[achievement.ID].Descriptions = make(map[string]string)
	}
	if err := rows.Err(); err != nil {
		return achievements, fmt.Errorf("getPPAchievementsWithSelect: %s", err)
	}

	// get MainLocale for each achievement
	rows2, err := sbuilder.Columns("pp_achievement.id", "pp_achievement_name.lang").
		From("pp_achievement").
		Join("pp_achievement_name ON pp_achievement.id = pp_achievement_name.id AND pp_achievement_name.main = true").
		RunWith(tx).Query()
	if err != nil {
		return achievements, fmt.Errorf("getPPAchievementsWithSelect: %s", err)
	}
	defer rows2.Close()

	for rows2.Next() {
		var id, lang string
		err := rows2.Scan(&id, &lang)
		if err != nil {
			return achievements, fmt.Errorf("getPPAchievementsWithSelect: %s", err)
		}
		achievementMap[id].MainLocale = lang
	}
	if err := rows2.Err(); err != nil {
		return achievements, fmt.Errorf("getPPAchievementsWithSelect: %s", err)
	}

	// get localized name map for each line
	rows3, err := sbuilder.Columns("pp_achievement.id", "pp_achievement_name.lang", "pp_achievement_name.name", "pp_achievement_name.description").
		From("pp_achievement").
		Join("pp_achievement_name ON pp_achievement.id = pp_achievement_name.id").
		RunWith(tx).Query()
	if err != nil {
		return achievements, fmt.Errorf("getPPAchievementsWithSelect: %s", err)
	}
	defer rows3.Close()

	for rows3.Next() {
		var id, lang, name, description string
		err := rows3.Scan(&id, &lang, &name, &description)
		if err != nil {
			return achievements, fmt.Errorf("getPPAchievementsWithSelect: %s", err)
		}
		achievementMap[id].Names[lang] = name
		achievementMap[id].Descriptions[lang] = description
	}
	if err := rows3.Err(); err != nil {
		return achievements, fmt.Errorf("getPPAchievementsWithSelect: %s", err)
	}
	return achievements, nil
}

// GetPPAchievement returns the achievement with the given ID
func GetPPAchievement(node sqalx.Node, id string) (*PPAchievement, error) {
	s := sdb.Select().
		Where(sq.Eq{"pp_achievement.id": id})
	achievements, err := getPPAchievementsWithSelect(node, s)
	if err != nil {
		return nil, err
	}
	if len(achievements) == 0 {
		return nil, errors.New("PPAchievement not found")
	}
	return achievements[0], nil
}

// CountAchieved returns the number of players who have achieved this achievement
func (achievement *PPAchievement) CountAchieved(node sqalx.Node) (int, error) {
	tx, err := node.Beginx()
	if err != nil {
		return 0, err
	}
	defer tx.Commit() // read-only tx

	var count int
	err = sdb.Select("COUNT(*)").
		From("pp_player_has_achievement").
		Where(sq.Eq{"achievement_id": achievement.ID}).
		Where("achieved IS NOT NULL").
		RunWith(tx).
		Scan(&count)
	return count, err
}

// FirstAchieved returns the first time anyone achieved this achievement
func (achievement *PPAchievement) FirstAchieved(node sqalx.Node) (time.Time, error) {
	tx, err := node.Beginx()
	if err != nil {
		return time.Time{}, err
	}
	defer tx.Commit() // read-only tx

	var achieved pq.NullTime
	err = sdb.Select("MIN(achieved)").
		From("pp_player_has_achievement").
		Where(sq.Eq{"achievement_id": achievement.ID}).
		Where("achieved IS NOT NULL").
		RunWith(tx).
		Scan(&achieved)
	return achieved.Time, err
}

// LastAchieved returns the last time anyone achieved this achievement
func (achievement *PPAchievement) LastAchieved(node sqalx.Node) (time.Time, error) {
	tx, err := node.Beginx()
	if err != nil {
		return time.Time{}, err
	}
	defer tx.Commit() // read-only tx

	var achieved pq.NullTime
	err = sdb.Select("MAX(achieved)").
		From("pp_player_has_achievement").
		Where(sq.Eq{"achievement_id": achievement.ID}).
		Where("achieved IS NOT NULL").
		RunWith(tx).
		Scan(&achieved)
	return achieved.Time, err
}

// UnmarshalConfig decodes the Config field for this transaction as JSON
func (achievement *PPAchievement) UnmarshalConfig(v interface{}) error {
	return json.Unmarshal([]byte(achievement.Config), &v)
}

// MarshalConfig encodes the parameter as JSON and places the result in the Config field
func (achievement *PPAchievement) MarshalConfig(v interface{}) error {
	b, err := json.Marshal(v)
	if err == nil {
		achievement.Config = string(b)
	}
	return err
}

// Update adds or updates the PPAchievement. Does not supporting updating names, descriptions or MainLocale
func (achievement *PPAchievement) Update(node sqalx.Node) error {
	tx, err := node.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = sdb.Insert("pp_achievement").
		Columns("id", "strategy", "config", "icon", "xp_reward").
		Values(achievement.ID, achievement.Strategy.ID(), achievement.Config, achievement.Icon, achievement.XPReward).
		Suffix("ON CONFLICT (id) DO UPDATE SET strategy = ?, config = ?, icon = ?, xp_reward = ?",
			achievement.Strategy.ID(), achievement.Config, achievement.Icon, achievement.XPReward).
		RunWith(tx).Exec()

	if err != nil {
		return errors.New("AddPPAchievement: " + err.Error())
	}
	return tx.Commit()
}

// Delete deletes the PPAchievement
func (achievement *PPAchievement) Delete(node sqalx.Node) error {
	tx, err := node.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = sdb.Delete("pp_achievement_name").
		Where(sq.Eq{"id": achievement.ID}).RunWith(tx).Exec()
	if err != nil {
		return fmt.Errorf("RemovePPAchievement: %s", err)
	}

	_, err = sdb.Delete("pp_achievement").
		Where(sq.Eq{"id": achievement.ID}).RunWith(tx).Exec()
	if err != nil {
		return fmt.Errorf("RemovePPAchievement: %s", err)
	}
	return tx.Commit()
}
