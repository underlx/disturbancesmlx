package dataobjects

import (
	"errors"
	"fmt"

	"github.com/gbl08ma/sqalx"
	sq "github.com/Masterminds/squirrel"
)

// LobbySchedule is a Lobby schedule
type LobbySchedule struct {
	Lobby        *Lobby
	Holiday      bool
	Day          int
	Open         bool
	OpenTime     Time
	OpenDuration Duration
}

// GetLobbySchedules returns a slice with all registered schedules
func GetLobbySchedules(node sqalx.Node) ([]*LobbySchedule, error) {
	return getLobbySchedulesWithSelect(node, sdb.Select())
}

func getLobbySchedulesWithSelect(node sqalx.Node, sbuilder sq.SelectBuilder) ([]*LobbySchedule, error) {
	schedules := []*LobbySchedule{}

	tx, err := node.Beginx()
	if err != nil {
		return schedules, err
	}
	defer tx.Commit() // read-only tx

	rows, err := sbuilder.Columns("lobby_id", "holiday", "day", "open", "open_time", "open_duration").
		From("station_lobby_schedule").
		RunWith(tx).Query()
	if err != nil {
		return schedules, fmt.Errorf("getLobbySchedulesWithSelect: %s", err)
	}
	defer rows.Close()

	var lobbyIDs []string
	for rows.Next() {
		var schedule LobbySchedule
		var lobbyID string
		err := rows.Scan(
			&lobbyID,
			&schedule.Holiday,
			&schedule.Day,
			&schedule.Open,
			&schedule.OpenTime,
			&schedule.OpenDuration)
		if err != nil {
			return schedules, fmt.Errorf("getLobbySchedulesWithSelect: %s", err)
		}
		schedules = append(schedules, &schedule)
		lobbyIDs = append(lobbyIDs, lobbyID)
	}
	if err := rows.Err(); err != nil {
		return schedules, fmt.Errorf("getLobbySchedulesWithSelect: %s", err)
	}
	for i := range lobbyIDs {
		schedules[i].Lobby, err = GetLobby(tx, lobbyIDs[i])
		if err != nil {
			return schedules, fmt.Errorf("getLobbySchedulesWithSelect: %s", err)
		}
	}
	return schedules, nil
}

// Compare checks if two schedules are the same regardless of their day
func (schedule *LobbySchedule) Compare(s2 *LobbySchedule) bool {
	return (!schedule.Open && !s2.Open) ||
		(schedule.Open == s2.Open &&
			schedule.OpenTime == s2.OpenTime &&
			schedule.OpenDuration == s2.OpenDuration)
}

// Update adds or updates the schedule
func (schedule *LobbySchedule) Update(node sqalx.Node) error {
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
		return errors.New("AddLobbySchedule: " + err.Error())
	}
	tx.Delete(getCacheKey("station-closed", schedule.Lobby.Station.ID))
	return tx.Commit()
}

// Delete deletes the schedule
func (schedule *LobbySchedule) Delete(node sqalx.Node) error {
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
		return fmt.Errorf("RemoveLobbySchedule: %s", err)
	}
	tx.Delete(getCacheKey("station-closed", schedule.Lobby.Station.ID))
	return tx.Commit()
}
