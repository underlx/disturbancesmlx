package dataobjects

import (
	"errors"
	"fmt"
	"time"

	sq "github.com/gbl08ma/squirrel"
	"github.com/heetch/sqalx"
	"github.com/lib/pq"
)

// Disturbance represents a disturbance
type Disturbance struct {
	ID          string
	StartTime   time.Time
	EndTime     time.Time
	Ended       bool
	Line        *Line
	Description string
	Statuses    []*Status
}

// GetDisturbances returns a slice with all registered disturbances
func GetDisturbances(node sqalx.Node) ([]*Disturbance, error) {
	s := sdb.Select().
		OrderBy("time_start ASC")
	return getDisturbancesWithSelect(node, s)
}

// GetOngoingDisturbances returns a slice with all ongoing disturbances
func GetOngoingDisturbances(node sqalx.Node) ([]*Disturbance, error) {
	s := sdb.Select().
		Where("time_end IS NULL").
		OrderBy("time_start ASC")
	return getDisturbancesWithSelect(node, s)
}

// GetDisturbancesBetween returns a slice with disturbances affecting the specified interval
func GetDisturbancesBetween(node sqalx.Node, start time.Time, end time.Time) ([]*Disturbance, error) {
	s := sdb.Select().
		Where(sq.Or{
			sq.Expr("time_start BETWEEN ? AND ?",
				start, end),
			sq.Expr("time_end BETWEEN ? AND ?",
				start, end),
		}).
		OrderBy("time_start ASC")
	return getDisturbancesWithSelect(node, s)
}

// getDisturbancesWithSelect returns a slice with all disturbances that match the conditions in sbuilder
func getDisturbancesWithSelect(node sqalx.Node, sbuilder sq.SelectBuilder) ([]*Disturbance, error) {
	disturbances := []*Disturbance{}

	tx, err := node.Beginx()
	if err != nil {
		return disturbances, err
	}
	defer tx.Commit() // read-only tx

	rows, err := sbuilder.Columns("line_disturbance.id", "line_disturbance.time_start",
		"line_disturbance.time_end", "line_disturbance.mline", "line_disturbance.description").
		From("line_disturbance").
		RunWith(tx).Query()
	if err != nil {
		return disturbances, fmt.Errorf("getDisturbancesWithSelect: %s", err)
	}

	lineIDs := []string{}
	for rows.Next() {
		var disturbance Disturbance
		var timeEnd pq.NullTime
		var lineID string
		err := rows.Scan(
			&disturbance.ID,
			&disturbance.StartTime,
			&timeEnd,
			&lineID,
			&disturbance.Description)
		if err != nil {
			rows.Close()
			return disturbances, fmt.Errorf("getDisturbancesWithSelect: %s", err)
		}
		disturbance.EndTime = timeEnd.Time
		disturbance.Ended = timeEnd.Valid

		disturbances = append(disturbances, &disturbance)
		lineIDs = append(lineIDs, lineID)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return disturbances, fmt.Errorf("getDisturbancesWithSelect: %s", err)
	}
	rows.Close()

	for i := range disturbances {
		disturbances[i].Line, err = GetLine(tx, lineIDs[i])
		if err != nil {
			return disturbances, fmt.Errorf("getDisturbancesWithSelect: %s", err)
		}
		rows, err := sdb.Select("status_id").
			From("line_disturbance_has_status").
			Where(sq.Eq{"disturbance_id": disturbances[i].ID}).
			RunWith(tx).Query()
		if err != nil {
			return disturbances, fmt.Errorf("getDisturbancesWithSelect: %s", err)
		}

		statusIDs := []string{}
		for rows.Next() {
			var statusID string
			err := rows.Scan(&statusID)
			if err != nil {
				rows.Close()
				return disturbances, fmt.Errorf("getDisturbancesWithSelect: %s", err)
			}
			statusIDs = append(statusIDs, statusID)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return disturbances, fmt.Errorf("getDisturbancesWithSelect: %s", err)
		}
		rows.Close()

		for j := range statusIDs {
			status, err := GetStatus(tx, statusIDs[j])
			if err != nil {
				return disturbances, fmt.Errorf("getDisturbancesWithSelect: %s", err)
			}
			disturbances[i].Statuses = append(disturbances[i].Statuses, status)
		}
	}
	return disturbances, nil
}

// GetDisturbance returns the Disturbance with the given ID
func GetDisturbance(node sqalx.Node, id string) (*Disturbance, error) {
	s := sdb.Select().
		Where(sq.Eq{"id": id})
	disturbances, err := getDisturbancesWithSelect(node, s)
	if err != nil {
		return nil, err
	}
	if len(disturbances) == 0 {
		return nil, errors.New("Disturbance not found")
	}
	return disturbances[0], nil
}

// LatestStatus returns the most recent status of this disturbance
func (disturbance *Disturbance) LatestStatus() *Status {
	var latest *Status
	for _, status := range disturbance.Statuses {
		if latest == nil || status.Time.After(latest.Time) {
			latest = status
		}
	}
	return latest
}

// Update adds or updates the disturbance
func (disturbance *Disturbance) Update(node sqalx.Node) error {
	tx, err := node.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	timeEnd := pq.NullTime{
		Time:  disturbance.EndTime,
		Valid: disturbance.Ended,
	}

	_, err = sdb.Insert("line_disturbance").
		Columns("id", "time_start", "time_end", "mline", "description").
		Values(disturbance.ID, disturbance.StartTime, timeEnd, disturbance.Line.ID, disturbance.Description).
		Suffix("ON CONFLICT (id) DO UPDATE SET time_start = ?, time_end = ?, mline = ?, description = ?",
			disturbance.StartTime, timeEnd, disturbance.Line.ID, disturbance.Description).
		RunWith(tx).Exec()
	if err != nil {
		return errors.New("AddDisturbance: " + err.Error())
	}

	_, err = sdb.Delete("line_disturbance_has_status").
		Where(sq.Eq{"disturbance_id": disturbance.ID}).RunWith(tx).Exec()
	if err != nil {
		return fmt.Errorf("AddDisturbance: %s", err)
	}

	for i := range disturbance.Statuses {
		err = disturbance.Statuses[i].Update(tx)
		if err != nil {
			return errors.New("AddDisturbance: " + err.Error())
		}

		_, err = sdb.Insert("line_disturbance_has_status").
			Columns("disturbance_id", "status_id").
			Values(disturbance.ID, disturbance.Statuses[i].ID).
			RunWith(tx).Exec()
	}
	return tx.Commit()
}

// Delete deletes the disturbance
func (disturbance *Disturbance) Delete(node sqalx.Node) error {
	tx, err := node.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = sdb.Delete("line_disturbance_has_status").
		Where(sq.Eq{"disturbance_id": disturbance.ID}).RunWith(tx).Exec()
	if err != nil {
		return fmt.Errorf("RemoveDisturbance: %s", err)
	}

	_, err = sdb.Delete("line_disturbance").
		Where(sq.Eq{"id": disturbance.ID}).RunWith(tx).Exec()
	if err != nil {
		return fmt.Errorf("RemoveDisturbance: %s", err)
	}
	return tx.Commit()
}
