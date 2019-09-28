package dataobjects

import (
	"errors"
	"fmt"
	"time"

	"github.com/gbl08ma/sqalx"
	sq "github.com/gbl08ma/squirrel"
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

// GetTripsForSubmitterBetween returns a slice with trips submitted by the specified submitter made in the specified interval
func GetTripsForSubmitterBetween(node sqalx.Node, submitter *APIPair, start time.Time, end time.Time) ([]*Trip, error) {
	s := sdb.Select().
		Where(sq.Eq{"submitter": submitter.Key}).
		Where(sq.Or{
			sq.Expr("start_time BETWEEN ? AND ?",
				start, end),
			sq.Expr("end_time BETWEEN ? AND ?",
				start, end),
		}).
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
			Where(sq.Eq{"trip_id": trips[i].ID}).
			OrderBy("entry_time, leave_time ASC")

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

// getTripIDsWithSelect returns a slice with the IDs of all trips that match the conditions in sbuilder
func getTripIDsWithSelect(node sqalx.Node, sbuilder sq.SelectBuilder) ([]string, error) {
	tx, err := node.Beginx()
	if err != nil {
		return []string{}, err
	}
	defer tx.Commit() // read-only tx

	rows, err := sbuilder.Columns("trip.id").
		From("trip").
		RunWith(tx).Query()
	if err != nil {
		return []string{}, fmt.Errorf("GetTripIDs: %s", err)
	}

	ids := []string{}
	for rows.Next() {
		var id string
		err := rows.Scan(&id)
		if err != nil {
			rows.Close()
			return ids, fmt.Errorf("GetTripIDs: %s", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return ids, fmt.Errorf("GetTripIDs: %s", err)
	}
	rows.Close()
	return ids, nil
}

// GetTripIDs returns a slice containing the IDs of all the trips in the database
func GetTripIDs(node sqalx.Node) ([]string, error) {
	s := sdb.Select().
		OrderBy("start_time ASC")
	return getTripIDsWithSelect(node, s)
}

// GetTripIDsBetween returns a slice containing the IDs of the trips in the specified interval
func GetTripIDsBetween(node sqalx.Node, start time.Time, end time.Time) ([]string, error) {
	s := sdb.Select().
		Where(sq.Or{
			sq.Expr("start_time BETWEEN ? AND ?",
				start, end),
			sq.Expr("end_time BETWEEN ? AND ?",
				start, end),
		}).
		OrderBy("start_time ASC")
	return getTripIDsWithSelect(node, s)
}

// CountTripsByDay counts trips by day between the specified dates
func CountTripsByDay(node sqalx.Node, start time.Time, end time.Time) ([]time.Time, []int, []int, error) {
	tx, err := node.Beginx()
	if err != nil {
		return []time.Time{}, []int{}, []int{}, err
	}
	defer tx.Commit() // read-only tx

	rows, err := tx.Query("SELECT curd, COUNT(id) FILTER (WHERE user_confirmed = false), COUNT(id) FILTER (WHERE user_confirmed = true) "+
		"FROM generate_series(($2 at time zone $1)::date, ($3 at time zone $1)::date, '1 day') AS curd "+
		"LEFT OUTER JOIN trip ON curd = (start_time at time zone $1)::date "+
		"GROUP BY curd ORDER BY curd;",
		start.Location().String(), start, end)
	if err != nil {
		return []time.Time{}, []int{}, []int{}, fmt.Errorf("CountTripsByDay: %s", err)
	}
	defer rows.Close()

	var dates []time.Time
	var unconfirmedCounts []int
	var confirmedCounts []int
	for rows.Next() {
		var date time.Time
		var count, confirmedCount int
		err := rows.Scan(&date, &count, &confirmedCount)
		if err != nil {
			return dates, unconfirmedCounts, confirmedCounts, fmt.Errorf("CountTripsByDay: %s", err)
		}
		dates = append(dates, date)
		unconfirmedCounts = append(unconfirmedCounts, count)
		confirmedCounts = append(confirmedCounts, confirmedCount)
	}
	if err := rows.Err(); err != nil {
		return dates, unconfirmedCounts, confirmedCounts, fmt.Errorf("CountTripsByDay: %s", err)
	}
	return dates, unconfirmedCounts, confirmedCounts, nil
}

// AverageSpeed computes the average speed (km/h), total distance (m) and total duration of this trip
func (trip *Trip) AverageSpeed(node sqalx.Node) (speed float64, totalDistance int64, totalDuration time.Duration, err error) {
	tx, err := node.Beginx()
	if err != nil {
		return 0, 0, 0, err
	}
	defer tx.Commit() // read-only tx

	if len(trip.StationUses) <= 1 {
		// station visit or invalid trip
		// can't extract any data about connections
		return 0, 0, 0, fmt.Errorf("Trip not suitable for computing average speed")
	}

	var startTime, endTime time.Time
	for useIdx := 0; useIdx < len(trip.StationUses)-1; useIdx++ {
		sourceUse := trip.StationUses[useIdx]

		if sourceUse.Manual {
			// manual path extensions don't contain valid time data
			// skip
			continue
		}

		if sourceUse.Type == Interchange ||
			sourceUse.Type == Visit {
			continue
		}

		targetUse := trip.StationUses[useIdx+1]

		if targetUse.Manual {
			// manual path extensions don't contain valid time data
			// skip
			continue
		}

		connection, err := GetConnection(tx, sourceUse.Station.ID, targetUse.Station.ID, true)
		if err != nil {
			// connection might no longer exist (closed stations, etc.)
			// move on
			return 0, 0, 0, fmt.Errorf("Trip contains now-invalid connection from %s to %s", sourceUse.Station.ID, targetUse.Station.ID)
		}

		totalDistance += int64(connection.WorldLength)
		if startTime.IsZero() {
			startTime = sourceUse.LeaveTime
		}
		endTime = targetUse.EntryTime
	}
	totalDuration = endTime.Sub(startTime)
	km := float64(totalDistance) / 1000
	return km / totalDuration.Hours(), totalDistance, totalDuration, nil
}

// SimultaneousTrips returns a slice containing trips that took place alongside this one
func (trip *Trip) SimultaneousTrips(node sqalx.Node, excludeLongerThan time.Duration) ([]*Trip, error) {
	s := sdb.Select().
		Where(sq.NotEq{"id": trip.ID}).
		Where(sq.LtOrEq{"end_time - start_time": Duration(excludeLongerThan)}).
		Where(sq.Expr("(start_time, end_time) OVERLAPS (?, ?)", trip.StartTime, trip.EndTime)).
		OrderBy("start_time ASC")
	return getTripsWithSelect(node, s)
}

// SimultaneousTripIDs returns a slice containing IDs of the trips that took place alongside this one
func (trip *Trip) SimultaneousTripIDs(node sqalx.Node, excludeLongerThan time.Duration) ([]string, error) {
	s := sdb.Select().
		Where(sq.NotEq{"id": trip.ID}).
		Where(sq.LtOrEq{"end_time - start_time": Duration(excludeLongerThan)}).
		Where(sq.Expr("(start_time, end_time) OVERLAPS (?, ?)", trip.StartTime, trip.EndTime)).
		OrderBy("start_time ASC")
	return getTripIDsWithSelect(node, s)
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
		// fix invalid times produced by old app versions as best as we can
		if i != 0 && trip.StationUses[i].EntryTime.Before(nextAdmissibleEntryTime) {
			trip.StationUses[i].EntryTime = nextAdmissibleEntryTime
		}
		if trip.StationUses[i].LeaveTime.Before(trip.StationUses[i].EntryTime) {
			trip.StationUses[i].LeaveTime = trip.StationUses[i].EntryTime
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
