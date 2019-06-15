package dataobjects

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/lib/pq"

	"github.com/gbl08ma/sqalx"
	sq "github.com/gbl08ma/squirrel"
)

// PPPlayerAchievement contains data about a PosPlay achievement for one user
type PPPlayerAchievement struct {
	DiscordID    uint64
	Achievement  *PPAchievement
	AchievedTime time.Time
	Achieved     bool
	Extra        string
}

// GetPPPlayerAchievements returns a slice with all registered transactions
func GetPPPlayerAchievements(node sqalx.Node) ([]*PPPlayerAchievement, error) {
	return getPPPlayerAchievementsWithSelect(node, sdb.Select())
}

// CountPPPlayerAchievementsAchieved returns the total of achieved achievements
func CountPPPlayerAchievementsAchieved(node sqalx.Node) (int, error) {
	tx, err := node.Beginx()
	if err != nil {
		return 0, err
	}
	defer tx.Commit() // read-only tx

	var count int
	err = sdb.Select("COUNT(*)").
		From("pp_player_has_achievement").
		Where("achieved IS NOT NULL").
		RunWith(tx).
		Scan(&count)
	return count, err
}

func getPPPlayerAchievementsWithSelect(node sqalx.Node, sbuilder sq.SelectBuilder) ([]*PPPlayerAchievement, error) {
	achievements := []*PPPlayerAchievement{}

	tx, err := node.Beginx()
	if err != nil {
		return achievements, err
	}
	defer tx.Commit() // read-only tx

	rows, err := sbuilder.Columns("pp_player_has_achievement.discord_id", "pp_player_has_achievement.achievement_id",
		"pp_player_has_achievement.achieved", "pp_player_has_achievement.extra").
		From("pp_player_has_achievement").
		RunWith(tx).Query()
	if err != nil {
		return achievements, fmt.Errorf("getPPPlayerAchievementsWithSelect: %s", err)
	}
	defer rows.Close()

	achievementIDs := []string{}
	for rows.Next() {
		var achievement PPPlayerAchievement
		var achievementID string
		var achieved pq.NullTime
		err := rows.Scan(
			&achievement.DiscordID,
			&achievementID,
			&achieved,
			&achievement.Extra)
		if err != nil {
			return achievements, fmt.Errorf("getPPPlayerAchievementsWithSelect: %s", err)
		}
		achievement.Achieved = achieved.Valid
		if achieved.Valid {
			achievement.AchievedTime = achieved.Time
		}
		achievements = append(achievements, &achievement)
		achievementIDs = append(achievementIDs, achievementID)
	}
	if err := rows.Err(); err != nil {
		return achievements, fmt.Errorf("getPPPlayerAchievementsWithSelect: %s", err)
	}
	for i := range achievementIDs {
		achievements[i].Achievement, err = GetPPAchievement(tx, achievementIDs[i])
		if err != nil {
			return achievements, fmt.Errorf("getPPPlayerAchievementsWithSelect: %s", err)
		}
	}
	return achievements, nil
}

// GetPPPlayerAchievement returns the achievement with the given ID
func GetPPPlayerAchievement(node sqalx.Node, discordID uint64, achievementID string) (*PPPlayerAchievement, error) {
	s := sdb.Select().
		Where(sq.Eq{"discord_id": discordID}, sq.Eq{"achievement_id": achievementID})
	achievements, err := getPPPlayerAchievementsWithSelect(node, s)
	if err != nil {
		return nil, err
	}
	if len(achievements) == 0 {
		return nil, errors.New("PPPlayerAchievement not found")
	}
	return achievements[0], nil
}

// UnmarshalExtra decodes the Extra field for this transaction as JSON
func (achievement *PPPlayerAchievement) UnmarshalExtra(v interface{}) error {
	return json.Unmarshal([]byte(achievement.Extra), &v)
}

// MarshalExtra encodes the parameter as JSON and places the result in the Extra field
func (achievement *PPPlayerAchievement) MarshalExtra(v interface{}) error {
	b, err := json.Marshal(v)
	if err == nil {
		achievement.Extra = string(b)
	}
	return err
}

// Update adds or updates the PPPlayerAchievement
func (achievement *PPPlayerAchievement) Update(node sqalx.Node) error {
	tx, err := node.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// TODO / rethink: should we also do achievement.Achievement.Update(tx) here?
	// (could be useful for achievements that update their global configuration when some user unlocks them - e.g. "first to do X" - though I suppose those are better implemented as challenges, and achievements can always update themselves separately?)

	t := pq.NullTime{
		Time:  achievement.AchievedTime,
		Valid: achievement.Achieved,
	}

	_, err = sdb.Insert("pp_player_has_achievement").
		Columns("discord_id", "achievement_id", "achieved", "extra").
		Values(achievement.DiscordID, achievement.Achievement.ID, t, achievement.Extra).
		Suffix("ON CONFLICT (discord_id, achievement_id) DO UPDATE SET achieved = ?, extra = ?",
			t, achievement.Extra).
		RunWith(tx).Exec()

	if err != nil {
		return errors.New("AddPPPlayerAchievement: " + err.Error())
	}
	return tx.Commit()
}

// Delete deletes the PPPlayerAchievement
func (achievement *PPPlayerAchievement) Delete(node sqalx.Node) error {
	tx, err := node.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = sdb.Delete("pp_player_has_achievement").
		Where(sq.Eq{"discord_id": achievement.DiscordID}, sq.Eq{"achievement_id": achievement.Achievement.ID}).
		RunWith(tx).Exec()
	if err != nil {
		return fmt.Errorf("RemovePPPlayerAchievement: %s", err)
	}
	return tx.Commit()
}
