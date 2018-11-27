package dataobjects

import (
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"time"

	"github.com/gbl08ma/sqalx"
	sq "github.com/gbl08ma/squirrel"
	"github.com/lib/pq"
)

// Disturbance represents a disturbance
type Disturbance struct {
	ID          string
	Official    bool
	OStartTime  time.Time
	OEndTime    time.Time
	OEnded      bool
	UStartTime  time.Time
	UEndTime    time.Time
	UEnded      bool
	Line        *Line
	Description string
	Notes       string
	Statuses    []*Status
}

// DisturbanceCategory is a disturbance category
type DisturbanceCategory string

const (
	// SignalFailureCategory is attributed to disturbances involving signal failures
	SignalFailureCategory DisturbanceCategory = "SIGNAL_FAILURE"
	// TrainFailureCategory is attributed to disturbances involving train failures
	TrainFailureCategory DisturbanceCategory = "TRAIN_FAILURE"
	// PowerOutageCategory is attributed to disturbances involving power outages
	PowerOutageCategory DisturbanceCategory = "POWER_OUTAGE"
	// ThirdPartyFaultCategory is attributed to disturbances involving 3rd party causes
	ThirdPartyFaultCategory DisturbanceCategory = "3RD_PARTY_FAULT"
	// PassengerIncidentCategory is attributed to disturbances involving incidents with passengers
	PassengerIncidentCategory DisturbanceCategory = "PASSENGER_INCIDENT"
	// StationAnomalyCategory is attributed to disturbances involving station anomalies
	StationAnomalyCategory DisturbanceCategory = "STATION_ANOMALY"
	// CommunityReportedCategory is attributed to disturbances reported by the community
	CommunityReportedCategory DisturbanceCategory = "COMMUNITY_REPORTED"
)

// GetDisturbances returns a slice with all registered disturbances
func GetDisturbances(node sqalx.Node) ([]*Disturbance, error) {
	s := sdb.Select().
		OrderBy("time_start ASC")
	return getDisturbancesWithSelect(node, s)
}

// GetLatestNDisturbances returns up to `limit` most recent disturbances
func GetLatestNDisturbances(node sqalx.Node, limit uint64) ([]*Disturbance, error) {
	s := sdb.Select().
		OrderBy("time_start DESC").
		Limit(limit)
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

	rows, err := sbuilder.Columns("line_disturbance.id",
		"line_disturbance.time_start", "line_disturbance.time_end",
		"line_disturbance.otime_start", "line_disturbance.otime_end",
		"line_disturbance.mline", "line_disturbance.description", "line_disturbance.notes").
		From("line_disturbance").
		RunWith(tx).Query()
	if err != nil {
		return disturbances, fmt.Errorf("getDisturbancesWithSelect: %s", err)
	}

	lineIDs := []string{}
	for rows.Next() {
		var disturbance Disturbance
		var timeEnd pq.NullTime
		var otimeStart pq.NullTime
		var otimeEnd pq.NullTime
		var notes sql.NullString
		var lineID string
		err := rows.Scan(
			&disturbance.ID,
			&disturbance.UStartTime,
			&timeEnd,
			&otimeStart,
			&otimeEnd,
			&lineID,
			&disturbance.Description,
			&notes)
		if err != nil {
			rows.Close()
			return disturbances, fmt.Errorf("getDisturbancesWithSelect: %s", err)
		}
		disturbance.UEndTime = timeEnd.Time
		disturbance.UEnded = timeEnd.Valid
		disturbance.OStartTime = otimeStart.Time
		disturbance.Official = otimeStart.Valid
		disturbance.OEndTime = otimeEnd.Time
		disturbance.OEnded = otimeEnd.Valid
		disturbance.Notes = notes.String

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

		sort.SliceStable(disturbances[i].Statuses, func(x, y int) bool {
			return disturbances[i].Statuses[x].Time.Before(disturbances[i].Statuses[y].Time)
		})
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

var mlCompositeMessageMatcher = regexp.MustCompile("^ML_([0-9A-Z]+)_([0-9A-Z]+)_([0-9A-Z]+)$")

// Categories returns the categories for this disturbance
func (disturbance *Disturbance) Categories() []DisturbanceCategory {
	categories := []DisturbanceCategory{}
	deduplicator := make(map[DisturbanceCategory]bool)
	for _, status := range disturbance.Statuses {
		matches := mlCompositeMessageMatcher.FindStringSubmatch(string(status.MsgType))
		if len(matches) == 4 {
			var possibleCategory DisturbanceCategory
			switch matches[1] {
			case "SIGNAL":
				possibleCategory = SignalFailureCategory
			case "TRAIN":
				possibleCategory = TrainFailureCategory
			case "POWER":
				possibleCategory = PowerOutageCategory
			case "3RDPARTY":
				possibleCategory = ThirdPartyFaultCategory
			case "PASSENGER":
				possibleCategory = PassengerIncidentCategory
			case "STATION":
				possibleCategory = StationAnomalyCategory
			}
			if len(possibleCategory) > 0 && !deduplicator[possibleCategory] {
				categories = append(categories, possibleCategory)
				deduplicator[possibleCategory] = true
			}
			continue
		}
		switch status.MsgType {
		case ReportBeginMessage, ReportConfirmMessage, ReportReconfirmMessage, ReportSolvedMessage:
			if !deduplicator[CommunityReportedCategory] {
				categories = append(categories, CommunityReportedCategory)
				deduplicator[CommunityReportedCategory] = true
			}
		}
	}
	return categories
}

// Update adds or updates the disturbance
func (disturbance *Disturbance) Update(node sqalx.Node) error {
	tx, err := node.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	timeEnd := pq.NullTime{
		Time:  disturbance.UEndTime,
		Valid: disturbance.UEnded,
	}

	otimeStart := pq.NullTime{
		Time:  disturbance.OStartTime,
		Valid: disturbance.Official,
	}

	otimeEnd := pq.NullTime{
		Time:  disturbance.OEndTime,
		Valid: disturbance.Official && disturbance.OEnded,
	}

	notes := sql.NullString{
		String: disturbance.Notes,
		Valid:  len(disturbance.Notes) > 0,
	}

	_, err = sdb.Insert("line_disturbance").
		Columns("id", "time_start", "time_end", "otime_start", "otime_end", "mline", "description", "notes").
		Values(disturbance.ID, disturbance.UStartTime, timeEnd, otimeStart, otimeEnd, disturbance.Line.ID, disturbance.Description, notes).
		Suffix("ON CONFLICT (id) DO UPDATE SET time_start = ?, time_end = ?, otime_start = ?, otime_end = ?, mline = ?, description = ?, notes = ?",
			disturbance.UStartTime, timeEnd, otimeStart, otimeEnd, disturbance.Line.ID, disturbance.Description, notes).
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
		if err != nil {
			return errors.New("AddDisturbance: " + err.Error())
		}
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
