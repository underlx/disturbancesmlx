package dataobjects

import (
	"errors"
	"fmt"

	sq "github.com/gbl08ma/squirrel"
	"github.com/heetch/sqalx"
)

// LineSchedule is a Line schedule
type LineSchedule struct {
	Line         *Line
	Holiday      bool
	Day          int
	Open         bool
	OpenTime     Time
	OpenDuration Duration
}

// GetLineSchedules returns a slice with all registered schedules
func GetLineSchedules(node sqalx.Node) ([]*LineSchedule, error) {
	return getLineSchedulesWithSelect(node, sdb.Select())
}

func getLineSchedulesWithSelect(node sqalx.Node, sbuilder sq.SelectBuilder) ([]*LineSchedule, error) {
	schedules := []*LineSchedule{}

	tx, err := node.Beginx()
	if err != nil {
		return schedules, err
	}
	defer tx.Commit() // read-only tx

	rows, err := sbuilder.Columns("line_id", "holiday", "day", "open", "open_time", "open_duration").
		From("line_schedule").
		RunWith(tx).Query()
	if err != nil {
		return schedules, fmt.Errorf("getLineSchedulesWithSelect: %s", err)
	}
	defer rows.Close()

	var lineIDs []string
	for rows.Next() {
		var schedule LineSchedule
		var lineID string
		err := rows.Scan(
			&lineID,
			&schedule.Holiday,
			&schedule.Day,
			&schedule.Open,
			&schedule.OpenTime,
			&schedule.OpenDuration)
		if err != nil {
			return schedules, fmt.Errorf("getLineSchedulesWithSelect: %s", err)
		}
		schedules = append(schedules, &schedule)
		lineIDs = append(lineIDs, lineID)
	}
	if err := rows.Err(); err != nil {
		return schedules, fmt.Errorf("getLineSchedulesWithSelect: %s", err)
	}
	for i := range lineIDs {
		schedules[i].Line, err = GetLine(tx, lineIDs[i])
		if err != nil {
			return schedules, fmt.Errorf("getLineSchedulesWithSelect: %s", err)
		}
	}
	return schedules, nil
}

// Compare checks if two schedules are the same regardless of their day
func (schedule *LineSchedule) Compare(s2 *LineSchedule) bool {
	return (!schedule.Open && !s2.Open) ||
		(schedule.Open == s2.Open &&
			schedule.OpenTime == s2.OpenTime &&
			schedule.OpenDuration == s2.OpenDuration)
}

// Update adds or updates the schedule
func (schedule *LineSchedule) Update(node sqalx.Node) error {
	tx, err := node.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	err = schedule.Line.Update(tx)
	if err != nil {
		return errors.New("AddSchedule: " + err.Error())
	}

	_, err = sdb.Insert("line_schedule").
		Columns("line_id", "holiday", "day", "open", "open_time", "open_duration").
		Values(schedule.Line.ID, schedule.Holiday, schedule.Day, schedule.Open, schedule.OpenTime, schedule.OpenDuration).
		Suffix("ON CONFLICT (line_id, holiday, day) DO UPDATE SET open = ?, open_time = ?, open_duration = ?",
			schedule.Open, schedule.OpenTime, schedule.OpenDuration).
		RunWith(tx).Exec()

	if err != nil {
		return errors.New("AddLineSchedule: " + err.Error())
	}
	return tx.Commit()
}

// Delete deletes the schedule
func (schedule *LineSchedule) Delete(node sqalx.Node) error {
	tx, err := node.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = sdb.Delete("line_schedule").
		Where(sq.Eq{"line_id": schedule.Line.ID},
			sq.Eq{"holiday": schedule.Holiday},
			sq.Eq{"day": schedule.Day}).
		RunWith(tx).Exec()
	if err != nil {
		return fmt.Errorf("RemoveLineSchedule: %s", err)
	}
	return tx.Commit()
}
