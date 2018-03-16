package dataobjects

import (
	"errors"
	"fmt"

	sq "github.com/gbl08ma/squirrel"
	"github.com/heetch/sqalx"
)

// NetworkSchedule is a Network schedule
type NetworkSchedule struct {
	Network      *Network
	Holiday      bool
	Day          int
	Open         bool
	OpenTime     Time
	OpenDuration Duration
}

// GetNetworkSchedules returns a slice with all registered schedules
func GetNetworkSchedules(node sqalx.Node) ([]*NetworkSchedule, error) {
	return getNetworkSchedulesWithSelect(node, sdb.Select())
}

func getNetworkSchedulesWithSelect(node sqalx.Node, sbuilder sq.SelectBuilder) ([]*NetworkSchedule, error) {
	schedules := []*NetworkSchedule{}

	tx, err := node.Beginx()
	if err != nil {
		return schedules, err
	}
	defer tx.Commit() // read-only tx

	rows, err := sbuilder.Columns("network_id", "holiday", "day", "open", "open_time", "open_duration").
		From("network_schedule").
		RunWith(tx).Query()
	if err != nil {
		return schedules, fmt.Errorf("getNetworkSchedulesWithSelect: %s", err)
	}
	defer rows.Close()

	var networkIDs []string
	for rows.Next() {
		var schedule NetworkSchedule
		var networkID string
		err := rows.Scan(
			&networkID,
			&schedule.Holiday,
			&schedule.Day,
			&schedule.Open,
			&schedule.OpenTime,
			&schedule.OpenDuration)
		if err != nil {
			return schedules, fmt.Errorf("getNetworkSchedulesWithSelect: %s", err)
		}
		schedules = append(schedules, &schedule)
		networkIDs = append(networkIDs, networkID)
	}
	if err := rows.Err(); err != nil {
		return schedules, fmt.Errorf("getNetworkSchedulesWithSelect: %s", err)
	}
	for i := range networkIDs {
		schedules[i].Network, err = GetNetwork(tx, networkIDs[i])
		if err != nil {
			return schedules, fmt.Errorf("getNetworkSchedulesWithSelect: %s", err)
		}
	}
	return schedules, nil
}

// Compare checks if two schedules are the same regardless of their day
func (schedule *NetworkSchedule) Compare(s2 *NetworkSchedule) bool {
	return (!schedule.Open && !s2.Open) ||
		(schedule.Open == s2.Open &&
			schedule.OpenTime == s2.OpenTime &&
			schedule.OpenDuration == s2.OpenDuration)
}

// Update adds or updates the schedule
func (schedule *NetworkSchedule) Update(node sqalx.Node) error {
	tx, err := node.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	err = schedule.Network.Update(tx)
	if err != nil {
		return errors.New("AddSchedule: " + err.Error())
	}

	_, err = sdb.Insert("network_schedule").
		Columns("network_id", "holiday", "day", "open", "open_time", "open_duration").
		Values(schedule.Network.ID, schedule.Holiday, schedule.Day, schedule.Open, schedule.OpenTime, schedule.OpenDuration).
		Suffix("ON CONFLICT (network_id, holiday, day) DO UPDATE SET open = ?, open_time = ?, open_duration = ?",
			schedule.Open, schedule.OpenTime, schedule.OpenDuration).
		RunWith(tx).Exec()

	if err != nil {
		return errors.New("AddNetworkSchedule: " + err.Error())
	}
	return tx.Commit()
}

// Delete deletes the schedule
func (schedule *NetworkSchedule) Delete(node sqalx.Node) error {
	tx, err := node.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = sdb.Delete("network_schedule").
		Where(sq.Eq{"network_id": schedule.Network.ID},
			sq.Eq{"holiday": schedule.Holiday},
			sq.Eq{"day": schedule.Day}).
		RunWith(tx).Exec()
	if err != nil {
		return fmt.Errorf("RemoveNetworkSchedule: %s", err)
	}
	return tx.Commit()
}
