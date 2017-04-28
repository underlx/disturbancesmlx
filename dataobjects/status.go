package dataobjects

import (
	"errors"
	"fmt"
	"time"

	sq "github.com/gbl08ma/squirrel"
	"github.com/heetch/sqalx"
)

// Status represents the status of a Line at a certain point in time
type Status struct {
	ID         string
	Time       time.Time
	Line       *Line
	IsDowntime bool
	Status     string
	Source     *Source
}

// GetStatuses returns a slice with all registered statuses
func GetStatuses(node sqalx.Node) ([]*Status, error) {
	statuss := []*Status{}

	tx, err := node.Beginx()
	if err != nil {
		return statuss, err
	}
	defer tx.Commit() // read-only tx

	rows, err := sdb.Select("id", "timestamp", "mline", "downtime", "status", "source").
		From("line_status").
		OrderBy("timestamp ASC").
		RunWith(tx).Query()
	if err != nil {
		return statuss, fmt.Errorf("GetStatuses: %s", err)
	}
	defer rows.Close()

	lineIDs := []string{}
	sourceIDs := []string{}
	for rows.Next() {
		var status Status
		var lineID, sourceID string
		err := rows.Scan(
			&status.ID,
			&status.Time,
			&lineID,
			&status.IsDowntime,
			&status.Status,
			&sourceID)
		if err != nil {
			return statuss, fmt.Errorf("GetStatuses: %s", err)
		}
		statuss = append(statuss, &status)
		lineIDs = append(lineIDs, lineID)
		sourceIDs = append(sourceIDs, sourceID)
	}
	if err := rows.Err(); err != nil {
		return statuss, fmt.Errorf("GetStatuses: %s", err)
	}
	for i := range lineIDs {
		statuss[i].Line, err = GetLine(tx, lineIDs[i])
		if err != nil {
			return statuss, fmt.Errorf("GetStatuses: %s", err)
		}
		statuss[i].Source, err = GetSource(tx, sourceIDs[i])
		if err != nil {
			return statuss, fmt.Errorf("GetStatuses: %s", err)
		}
	}
	return statuss, nil
}

// GetStatus returns the Status with the given ID
func GetStatus(node sqalx.Node, id string) (*Status, error) {
	var status Status
	tx, err := node.Beginx()
	if err != nil {
		return &status, err
	}
	defer tx.Commit() // read-only tx

	var lineID, sourceID string
	err = sdb.Select("id", "timestamp", "mline", "downtime", "status", "source").
		From("line_status").
		Where(sq.Eq{"id": id}).
		RunWith(tx).QueryRow().Scan(
		&status.ID,
		&status.Time,
		&lineID,
		&status.IsDowntime,
		&status.Status,
		&sourceID)
	if err != nil {
		return &status, errors.New("GetStatus: " + err.Error())
	}
	status.Line, err = GetLine(tx, lineID)
	if err != nil {
		return &status, fmt.Errorf("GetStatus: %s", err)
	}
	status.Source, err = GetSource(tx, sourceID)
	if err != nil {
		return &status, fmt.Errorf("GetStatus: %s", err)
	}
	return &status, nil
}

// Update adds or updates the status
func (status *Status) Update(node sqalx.Node) error {
	tx, err := node.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	err = status.Source.Update(tx)
	if err != nil {
		return errors.New("AddStatus: " + err.Error())
	}

	_, err = sdb.Insert("line_status").
		Columns("id", "timestamp", "mline", "downtime", "status", "source").
		Values(status.ID, status.Time, status.Line.ID, status.IsDowntime, status.Status, status.Source.ID).
		Suffix("ON CONFLICT (id) DO UPDATE SET timestamp = ?, mline = ?, downtime = ?, status = ?, source = ?",
			status.Time, status.Line.ID, status.IsDowntime, status.Status, status.Source.ID).
		RunWith(tx).Exec()

	if err != nil {
		return errors.New("AddStatus: " + err.Error())
	}
	return tx.Commit()
}

// Delete deletes the status
func (status *Status) Delete(node sqalx.Node) error {
	tx, err := node.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = sdb.Delete("line_status").
		Where(sq.Eq{"id": status.ID}).RunWith(tx).Exec()
	if err != nil {
		return fmt.Errorf("RemoveStatus: %s", err)
	}
	return tx.Commit()
}
