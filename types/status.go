package types

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gbl08ma/sqalx"
	sq "github.com/Masterminds/squirrel"
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

	// MLGenericMessage corresponds to the format "existem perturbações na circulação. O tempo de espera pode ser superior ao normal. Pedimos desculpa pelo incómodo causado"
	MLGenericMessage StatusMessageType = "ML_GENERIC"
	// MLSolvedMessage corresponds to the format "Circulação normal"
	MLSolvedMessage StatusMessageType = "ML_SOLVED"
	// MLClosedMessage corresponds to the format "Serviço encerrado"
	MLClosedMessage StatusMessageType = "ML_CLOSED"
	// MLSpecialServiceMessage corresponds to the format "Serviço especial$1" (only observed $1 so far is " de passagem de ano")
	MLSpecialServiceMessage StatusMessageType = "ML_SPECIAL"

	// MLCompositeMessage corresponds to the format:
	// "[d|D]evido a $1$2$3"
	// $1 may be one of:
	// - "avaria na sinalização" (SIGNAL)
	// - "avaria de comboio" (TRAIN)
	// - "falha de energia" (POWER)
	// - "causa alheia ao Metro" (3RDPARTY)
	// - "incidente com passageiro" (PASSENGER)
	// - "anomalia na estação" (STATION)
	// $2 may be one of:
	// - ", a circulação está interrompida desde as XX:XX." (SINCE)
	// - " está interrompida a circulação." (HALTED)
	// - " está interrompida a circulação na linha entre as estações YYY e ZZZ." (BETWEEN)
	// - ", a circulação encontra-se com perturbações." (DELAYED)
	// $3 may be one of:
	// - " Não é possível prever a duração da interrupção, que poderá ser prolongada. Pedimos desculpa pelo incómodo causado" (LONGHALT, typically used with HALTED and BETWEEN)
	// - " O tempo de espera pode ser superior ao normal. Pedimos desculpa pelo incómodo" (LONGWAIT, typically used with DELAYED)
	// - " Esperamos retomar a circulação dentro de instantes" (SOON, typically used with SINCE)
	// - " Esperamos retomar a circulação num período inferior a 15 minutos. Pedimos desculpa pelo incómodo" (UNDER15, typically used with SINCE)
	// - " O tempo de reposição poderá ser superior a 15 minutos. Pedimos desculpa pelo incómodo" (OVER15, typically used with SINCE)
	MLCompositeMessage StatusMessageType = "ML_%s_%s_%s"
	// See also mlCompositeMessageMatcher in disturbance.go
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

// ComputeMsgType analyses the status message to assign the correct MsgType
func (status *Status) ComputeMsgType() {
	switch {
	case strings.Contains(status.Status, "existem perturbações na circulação"):
		status.MsgType = MLGenericMessage
		return
	case strings.Contains(status.Status, "Circulação normal"):
		status.MsgType = MLSolvedMessage
		return
	case strings.Contains(status.Status, "Serviço encerrado"):
		status.MsgType = MLClosedMessage
		return
	case strings.Contains(status.Status, "Serviço especial"):
		status.MsgType = MLSpecialServiceMessage
		return
	case strings.Contains(status.Status, "Os utilizadores comunicaram problemas na circulação"):
		status.MsgType = ReportBeginMessage
		return
	case strings.Contains(status.Status, "Vários utilizadores confirmaram problemas na circulação"):
		status.MsgType = ReportConfirmMessage
		return
	case strings.Contains(status.Status, "Vários utilizadores confirmaram mais problemas na circulação"):
		status.MsgType = ReportReconfirmMessage
		return
	case strings.Contains(status.Status, "Já não existem relatos de problemas na circulação"):
		status.MsgType = ReportSolvedMessage
		return
	}

	cause := ""
	switch {
	case strings.Contains(status.Status, "avaria na sinalização"):
		cause = "SIGNAL"
	case strings.Contains(status.Status, "avaria de comboio"):
		cause = "TRAIN"
	case strings.Contains(status.Status, "falha de energia"):
		cause = "POWER"
	case strings.Contains(status.Status, "causa alheia ao Metro"):
		cause = "3RDPARTY"
	case strings.Contains(status.Status, "incidente com passageiro"):
		cause = "PASSENGER"
	case strings.Contains(status.Status, "anomalia na estação"):
		cause = "STATION"
	}

	state := ""
	switch {
	case strings.Contains(status.Status, "a circulação está interrompida desde as"):
		state = "SINCE"
	case strings.Contains(status.Status, "está interrompida a circulação."):
		state = "HALTED"
	case strings.Contains(status.Status, "está interrompida a circulação na linha entre as estações"):
		state = "BETWEEN"
	case strings.Contains(status.Status, "a circulação encontra-se com perturbações"):
		state = "DELAYED"
	}

	outlook := ""
	switch {
	case strings.Contains(status.Status, "Não é possível prever a duração da interrupção, que poderá ser prolongada"):
		outlook = "LONGHALT"
	case strings.Contains(status.Status, "O tempo de espera pode ser superior ao normal"):
		outlook = "LONGWAIT"
	case strings.Contains(status.Status, "Esperamos retomar a circulação dentro de instantes"):
		outlook = "SOON"
	case strings.Contains(status.Status, "Esperamos retomar a circulação num período inferior a 15 minutos"):
		outlook = "UNDER15"
	case strings.Contains(status.Status, "O tempo de reposição poderá ser superior a 15 minutos"):
		outlook = "OVER15"
	}

	if cause == "" || state == "" || outlook == "" {
		return
	}
	status.MsgType = StatusMessageType(fmt.Sprintf(string(MLCompositeMessage), cause, state, outlook))
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
