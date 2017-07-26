package dataobjects

import (
	"errors"
	"fmt"
	"time"

	sq "github.com/gbl08ma/squirrel"
	"github.com/heetch/sqalx"
	"github.com/lib/pq"
	"github.com/satori/go.uuid"
)

// Trip represents a user-submitted subway trip
type Trip struct {
	ID            string
	StartTime     time.Time
	EndTime       time.Time
	Submitter     *APIPair
	SubmitTime    time.Time
	EditTime      time.Time
	Edited        bool
	UserConfirmed bool
	StationUses   []*StationUse
}

// GetTrips returns a slice with all registered trips
func GetTrips(node sqalx.Node) ([]*Trip, error) {
	s := sdb.Select().
		OrderBy("start_time ASC")
	return getTripsWithSelect(node, s)
}

// GetTripsForSubmitter returns a slice with all trips submitted by the specified submitter
func GetTripsForSubmitter(node sqalx.Node, submitter *APIPair) ([]*Trip, error) {
	s := sdb.Select().
		Where(sq.Eq{"submitter": submitter.Key}).
		OrderBy("start_time ASC")
	return getTripsWithSelect(node, s)
}

// getTripsWithSelect returns a slice with all trips that match the conditions in sbuilder
func getTripsWithSelect(node sqalx.Node, sbuilder sq.SelectBuilder) ([]*Trip, error) {
	trips := []*Trip{}

	tx, err := node.Beginx()
	if err != nil {
		return trips, err
	}
	defer tx.Commit() // read-only tx

	rows, err := sbuilder.Columns("trip.id", "trip.start_time", "trip.end_time",
		"trip.submitter", "trip.submit_time", "trip.edit_time", "trip.user_confirmed").
		From("trip").
		RunWith(tx).Query()
	if err != nil {
		return trips, fmt.Errorf("getTripsWithSelect: %s", err)
	}

	submitters := []string{}
	for rows.Next() {
		var trip Trip
		var timeEdit pq.NullTime
		var submitter string
		err := rows.Scan(
			&trip.ID,
			&trip.StartTime,
			&trip.EndTime,
			&submitter,
			&trip.SubmitTime,
			&timeEdit,
			&trip.UserConfirmed)
		if err != nil {
			rows.Close()
			return trips, fmt.Errorf("getTripsWithSelect: %s", err)
		}
		trip.EditTime = timeEdit.Time
		trip.Edited = timeEdit.Valid

		trips = append(trips, &trip)
		submitters = append(submitters, submitter)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return trips, fmt.Errorf("getTripsWithSelect: %s", err)
	}
	rows.Close()

	for i := range trips {
		trips[i].Submitter, err = GetPair(tx, submitters[i])
		if err != nil {
			return trips, fmt.Errorf("getTripsWithSelect: %s", err)
		}

		s := sdb.Select().
			Where(sq.Eq{"trip_id": trips[i].ID})

		trips[i].StationUses, err = getStationUsesWithSelect(tx, s)
		if err != nil {
			return trips, fmt.Errorf("getTripsWithSelect: %s", err)
		}
	}
	return trips, nil
}

// GetTrip returns the Trip with the given ID
func GetTrip(node sqalx.Node, id string) (*Trip, error) {
	s := sdb.Select().
		Where(sq.Eq{"id": id})
	trips, err := getTripsWithSelect(node, s)
	if err != nil {
		return nil, err
	}
	if len(trips) == 0 {
		return nil, errors.New("Trip not found")
	}
	return trips[0], nil
}

// Update adds or updates the trip
func (trip *Trip) Update(node sqalx.Node) error {
	if _, err := uuid.FromString(trip.ID); err != nil || len(trip.ID) != 36 {
		return errors.New("AddTrip: invalid trip ID")
	}

	if len(trip.StationUses) == 1 && trip.StationUses[0].Type != Visit {
		return errors.New("AddTrip: trip only has one station use, but is not a visit")
	}

	tx, err := node.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	timeEdit := pq.NullTime{
		Time:  trip.EditTime,
		Valid: trip.Edited,
	}

	_, err = sdb.Insert("trip").
		Columns("id", "start_time", "end_time", "submitter", "submit_time", "edit_time", "user_confirmed").
		Values(trip.ID, trip.StartTime, trip.EndTime, trip.Submitter.Key, trip.SubmitTime, timeEdit, trip.UserConfirmed).
		Suffix("ON CONFLICT (id) DO UPDATE SET start_time = ?, end_time = ?, submitter = ?, submit_time = ?, edit_time = ?, user_confirmed = ?",
			trip.StartTime, trip.EndTime, trip.Submitter.Key, trip.SubmitTime, timeEdit, trip.UserConfirmed).
		RunWith(tx).Exec()
	if err != nil {
		return errors.New("AddTrip: " + err.Error())
	}

	var nextAdmissibleEntryTime time.Time
	for i := range trip.StationUses {
		if i != 0 && trip.StationUses[i].EntryTime.Before(nextAdmissibleEntryTime) {
			return errors.New("AddTrip: station uses are not ordered or overlap")
		}
		err = trip.StationUses[i].Update(tx, trip.ID)
		if err != nil {
			return errors.New("AddTrip: " + err.Error())
		}
		nextAdmissibleEntryTime = trip.StationUses[i].LeaveTime
	}
	return tx.Commit()
}

// Delete deletes the trip
func (trip *Trip) Delete(node sqalx.Node) error {
	tx, err := node.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for i := range trip.StationUses {
		err = trip.StationUses[i].Delete(tx, trip.ID)
		if err != nil {
			return errors.New("RemoveTrip: " + err.Error())
		}
	}

	_, err = sdb.Delete("trip").
		Where(sq.Eq{"id": trip.ID}).RunWith(tx).Exec()
	if err != nil {
		return fmt.Errorf("RemoveTrip: %s", err)
	}
	return tx.Commit()
}
