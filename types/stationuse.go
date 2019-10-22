package types

import (
	"errors"
	"fmt"
	"time"

	"database/sql"

	"github.com/gbl08ma/sqalx"
	sq "github.com/Masterminds/squirrel"
)

// StationUse represents the stationUse of a Line at a certain point in time
type StationUse struct {
	Station    *Station
	EntryTime  time.Time
	LeaveTime  time.Time
	Type       StationUseType
	Manual     bool
	SourceLine *Line
	TargetLine *Line
}

// StationUseType corresponds to a type of station use (i.e. "how" the station was used)
type StationUseType string

const (
	// NetworkEntry is a type of station use reserved for the first station in a trip
	NetworkEntry StationUseType = "NETWORK_ENTRY"
	// NetworkExit is a type of station use reserved for the last station in a trip
	NetworkExit StationUseType = "NETWORK_EXIT"
	// Interchange is a type of station use reserved for a line change in a trip
	Interchange StationUseType = "INTERCHANGE"
	// GoneThrough is a type of station use reserved for when an user simply goes through a station on his way to somewhere
	GoneThrough StationUseType = "GONE_THROUGH"
	// Visit is a type of station use reserved for trips with a single station use
	Visit StationUseType = "VISIT"
)

// GetStationUses returns a slice with all registered stationUses
func GetStationUses(node sqalx.Node) ([]*StationUse, error) {
	s := sdb.Select().
		OrderBy("entry_time ASC")
	return getStationUsesWithSelect(node, s)
}

// getStationUsesWithSelect returns a slice with all station uses that match the conditions in sbuilder
func getStationUsesWithSelect(node sqalx.Node, sbuilder sq.SelectBuilder) ([]*StationUse, error) {
	stationUses := []*StationUse{}

	tx, err := node.Beginx()
	if err != nil {
		return stationUses, err
	}
	defer tx.Commit() // read-only tx

	rows, err := sbuilder.Columns("station_use.station_id", "station_use.entry_time", "station_use.leave_time",
		"station_use.type", "station_use.manual", "station_use.source_line", "station_use.target_line").
		From("station_use").
		RunWith(tx).Query()
	if err != nil {
		return stationUses, fmt.Errorf("getStationUsesWithSelect: %s", err)
	}

	stations := []string{}
	sourceLines := []string{}
	targetLines := []string{}
	for rows.Next() {
		var stationUse StationUse
		var station string
		var sourceLine sql.NullString
		var targetLine sql.NullString

		err := rows.Scan(
			&station,
			&stationUse.EntryTime,
			&stationUse.LeaveTime,
			&stationUse.Type,
			&stationUse.Manual,
			&sourceLine,
			&targetLine)
		if err != nil {
			rows.Close()
			return stationUses, fmt.Errorf("getStationUsesWithSelect: %s", err)
		}

		stationUses = append(stationUses, &stationUse)
		stations = append(stations, station)
		sourceLines = append(sourceLines, sourceLine.String)
		targetLines = append(targetLines, targetLine.String)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return stationUses, fmt.Errorf("getStationUsesWithSelect: %s", err)
	}
	rows.Close()

	for i := range stationUses {
		stationUses[i].Station, err = GetStation(tx, stations[i])
		if err != nil {
			return stationUses, fmt.Errorf("getStationUsesWithSelect: %s", err)
		}

		if sourceLines[i] != "" {
			stationUses[i].SourceLine, err = GetLine(tx, sourceLines[i])
			if err != nil {
				return stationUses, fmt.Errorf("getStationUsesWithSelect: %s", err)
			}
		}
		if targetLines[i] != "" {
			stationUses[i].TargetLine, err = GetLine(tx, targetLines[i])
			if err != nil {
				return stationUses, fmt.Errorf("getStationUsesWithSelect: %s", err)
			}
		}
	}
	return stationUses, nil
}

// Update adds or updates the stationUse
func (stationUse *StationUse) Update(node sqalx.Node, tripID string) error {
	sourceLine, targetLine, err := stationUse.checkValidAndLines()
	if err != nil {
		return err
	}

	tx, err := node.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = sdb.Insert("station_use").
		Columns("trip_id", "station_id", "entry_time", "leave_time", "type", "manual", "source_line", "target_line").
		Values(tripID, stationUse.Station.ID, stationUse.EntryTime, stationUse.LeaveTime, stationUse.Type, stationUse.Manual, sourceLine, targetLine).
		Suffix("ON CONFLICT (trip_id, station_id, entry_time) DO UPDATE SET leave_time = ?, type = ?, manual = ?, source_line = ?, target_line = ?",
			stationUse.LeaveTime, stationUse.Type, stationUse.Manual, sourceLine, targetLine).
		RunWith(tx).Exec()

	if err != nil {
		return errors.New("AddStationUse: " + err.Error())
	}
	return tx.Commit()
}

func (stationUse *StationUse) checkValidAndLines() (sourceLine, targetLine sql.NullString, err error) {
	if stationUse.LeaveTime.Before(stationUse.EntryTime) {
		return sql.NullString{}, sql.NullString{}, errors.New("AddStationUse: leave time before entry time")
	}

	if stationUse.Type != NetworkEntry && stationUse.Type != NetworkExit &&
		stationUse.Type != Interchange && stationUse.Type != GoneThrough &&
		stationUse.Type != Visit {
		return sql.NullString{}, sql.NullString{}, errors.New("AddStationUse: invalid type")
	}

	if stationUse.Type == Interchange &&
		(stationUse.SourceLine == nil || stationUse.TargetLine == nil) {
		return sql.NullString{}, sql.NullString{}, errors.New("AddStationUse: interchange use missing source or target lines")
	}

	if stationUse.Type != Interchange &&
		(stationUse.SourceLine != nil || stationUse.TargetLine != nil) {
		return sql.NullString{}, sql.NullString{}, errors.New("AddStationUse: non-interchange use contains source or target lines")
	}

	sourceLine = sql.NullString{
		Valid: stationUse.SourceLine != nil,
	}
	if sourceLine.Valid {
		sourceLine.String = stationUse.SourceLine.ID
	}

	targetLine = sql.NullString{
		Valid: stationUse.TargetLine != nil,
	}
	if targetLine.Valid {
		targetLine.String = stationUse.TargetLine.ID
	}

	if sourceLine.Valid && targetLine.Valid && sourceLine.String == targetLine.String {
		return sql.NullString{}, sql.NullString{}, errors.New("AddStationUse: interchange has the same source and target")
	}

	return sourceLine, targetLine, nil
}

// Delete deletes the stationUse
func (stationUse *StationUse) Delete(node sqalx.Node, tripID string) error {
	tx, err := node.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = sdb.Delete("station_use").
		Where(sq.Eq{"trip_id": tripID, "station_id": stationUse.Station.ID, "entry_time": stationUse.EntryTime}).RunWith(tx).Exec()
	if err != nil {
		return fmt.Errorf("RemoveStationUse: %s", err)
	}
	return tx.Commit()
}
