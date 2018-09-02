package dataobjects

import (
	"errors"
	"fmt"
	"time"

	sq "github.com/gbl08ma/squirrel"
	"github.com/gbl08ma/sqalx"
)

// Status represents the status of a Line at a certain point in time
type Status struct {
	ID         string
	Time       time.Time
	Line       *Line
	IsDowntime bool
	Status     string
	Source     *Source
	MsgType    StatusMessageType
}

// StatusMessageType indicates the type of the status message (to help with e.g. translation and disturbance categorization)
type StatusMessageType string

const (
	// RawMessage is an untranslatable message
	RawMessage StatusMessageType = "RAW"
	// ReportBeginMessage is the message for when users begin reporting disturbances
	ReportBeginMessage StatusMessageType = "REPORT_BEGIN"
	// ReportConfirmMessage is the message for when users confirm disturbance reports
	ReportConfirmMessage StatusMessageType = "REPORT_CONFIRM"
	// ReportReconfirmMessage is the message for when users confirm disturbance reports shortly after a disturbance ended
	ReportReconfirmMessage StatusMessageType = "REPORT_RECONFIRM"
	// ReportSolvedMessage is the message for when reports of disturbances are gone
	ReportSolvedMessage StatusMessageType = "REPORT_SOLVED"
)

// GetStatuses returns a slice with all registered statuses
func GetStatuses(node sqalx.Node) ([]*Status, error) {
	return getStatusesWithSelect(node, sdb.Select())
}

// GetStatus returns the Status with the given ID
func GetStatus(node sqalx.Node, id string) (*Status, error) {
	s := sdb.Select().
		Where(sq.Eq{"id": id})
	statuses, err := getStatusesWithSelect(node, s)
	if err != nil {
		return nil, err
	}
	if len(statuses) == 0 {
		return nil, errors.New("Status not found")
	}
	return statuses[0], nil
}

func getStatusesWithSelect(node sqalx.Node, sbuilder sq.SelectBuilder) ([]*Status, error) {
	statuss := []*Status{}

	tx, err := node.Beginx()
	if err != nil {
		return statuss, err
	}
	defer tx.Commit() // read-only tx

	rows, err := sbuilder.Columns("id", "timestamp", "mline", "downtime", "status", "source", "msgtype").
		From("line_status").
		OrderBy("timestamp ASC").
		RunWith(tx).Query()
	if err != nil {
		return statuss, fmt.Errorf("getStatusesWithSelect: %s", err)
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
			&sourceID,
			&status.MsgType)
		if err != nil {
			return statuss, fmt.Errorf("getStatusesWithSelect: %s", err)
		}
		statuss = append(statuss, &status)
		lineIDs = append(lineIDs, lineID)
		sourceIDs = append(sourceIDs, sourceID)
	}
	if err := rows.Err(); err != nil {
		return statuss, fmt.Errorf("getStatusesWithSelect: %s", err)
	}
	for i := range lineIDs {
		statuss[i].Line, err = GetLine(tx, lineIDs[i])
		if err != nil {
			return statuss, fmt.Errorf("getStatusesWithSelect: %s", err)
		}
		statuss[i].Source, err = GetSource(tx, sourceIDs[i])
		if err != nil {
			return statuss, fmt.Errorf("getStatusesWithSelect: %s", err)
		}
	}
	return statuss, nil
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

	if status.MsgType == "" {
		status.MsgType = RawMessage
	}

	_, err = sdb.Insert("line_status").
		Columns("id", "timestamp", "mline", "downtime", "status", "source", "msgtype").
		Values(status.ID, status.Time, status.Line.ID, status.IsDowntime, status.Status, status.Source.ID, status.MsgType).
		Suffix("ON CONFLICT (id) DO UPDATE SET timestamp = ?, mline = ?, downtime = ?, status = ?, source = ?, msgtype = ?",
			status.Time, status.Line.ID, status.IsDowntime, status.Status, status.Source.ID, status.MsgType).
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
