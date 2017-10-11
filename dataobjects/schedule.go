package dataobjects

import (
	"errors"
	"fmt"

	sq "github.com/gbl08ma/squirrel"
	"github.com/heetch/sqalx"
)

// Schedule is a Lobby schedule
type Schedule struct {
	Lobby        *Lobby
	Holiday      bool
	Day          int
	Open         bool
	OpenTime     Time
	OpenDuration Duration
}

// GetSchedules returns a slice with all registered schedules
func GetSchedules(node sqalx.Node) ([]*Schedule, error) {
	return getSchedulesWithSelect(node, sdb.Select())
}

func getSchedulesWithSelect(node sqalx.Node, sbuilder sq.SelectBuilder) ([]*Schedule, error) {
	schedules := []*Schedule{}

	tx, err := node.Beginx()
	if err != nil {
		return schedules, err
	}
	defer tx.Commit() // read-only tx

	rows, err := sbuilder.Columns("lobby_id", "holiday", "day", "open", "open_time", "open_duration").
		From("station_lobby_schedule").
		RunWith(tx).Query()
	if err != nil {
		return schedules, fmt.Errorf("getSchedulesWithSelect: %s", err)
	}
	defer rows.Close()

	var lobbyIDs []string
	for rows.Next() {
		var schedule Schedule
		var lobbyID string
		err := rows.Scan(
			&lobbyID,
			&schedule.Holiday,
			&schedule.Day,
			&schedule.Open,
			&schedule.OpenTime,
			&schedule.OpenDuration)
		if err != nil {
			return schedules, fmt.Errorf("getSchedulesWithSelect: %s", err)
		}
		schedules = append(schedules, &schedule)
		lobbyIDs = append(lobbyIDs, lobbyID)
	}
	if err := rows.Err(); err != nil {
		return schedules, fmt.Errorf("getSchedulesWithSelect: %s", err)
	}
	for i := range lobbyIDs {
		schedules[i].Lobby, err = GetLobby(tx, lobbyIDs[i])
		if err != nil {
			return schedules, fmt.Errorf("getSchedulesWithSelect: %s", err)
		}
	}
	return schedules, nil
}

// Compare checks if two schedules are the same regardless of their day
func (schedule *Schedule) Compare(s2 *Schedule) bool {
	return schedule.Open == s2.Open && schedule.OpenTime == s2.OpenTime && schedule.OpenDuration == s2.OpenDuration
}

// Update adds or updates the schedule
func (schedule *Schedule) Update(node sqalx.Node) error {
	tx, err := node.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	err = schedule.Lobby.Update(tx)
	if err != nil {
		return errors.New("AddSchedule: " + err.Error())
	}

	_, err = sdb.Insert("station_lobby_schedule").
		Columns("lobby_id", "holiday", "day", "open", "open_time", "open_duration").
		Values(schedule.Lobby.ID, schedule.Holiday, schedule.Day, schedule.Open, schedule.OpenTime, schedule.OpenDuration).
		Suffix("ON CONFLICT (lobby_id, holiday, day) DO UPDATE SET open = ?, open_time = ?, open_duration = ?",
			schedule.Open, schedule.OpenTime, schedule.OpenDuration).
		RunWith(tx).Exec()

	if err != nil {
		return errors.New("AddSchedule: " + err.Error())
	}
	return tx.Commit()
}

// Delete deletes the schedule
func (schedule *Schedule) Delete(node sqalx.Node) error {
	tx, err := node.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = sdb.Delete("station_lobby_schedule").
		Where(sq.Eq{"lobby_id": schedule.Lobby.ID},
			sq.Eq{"holiday": schedule.Holiday},
			sq.Eq{"day": schedule.Day}).
		RunWith(tx).Exec()
	if err != nil {
		return fmt.Errorf("RemoveSchedule: %s", err)
	}
	return tx.Commit()
}
