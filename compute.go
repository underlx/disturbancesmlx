package main

import (
	"fmt"
	"io"
	"math"
	"sort"
	"time"

	"github.com/heetch/sqalx"
	"github.com/underlx/disturbancesmlx/dataobjects"
)

// ComputeTypicalSeconds calculates and updates the TypicalSeconds
// for all the Connections and Transfers where that can be done using the registered Trips
// Current TypicalSeconds are ignored and discarded.
func ComputeTypicalSeconds(node sqalx.Node) error {
	tx, err := node.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	tripIDs, err := dataobjects.GetTripIDs(tx)
	if err != nil {
		return err
	}

	fmt.Printf("%d trip IDs\n", len(tripIDs))

	type TransferKey struct {
		Station string
		From    string
		To      string
	}

	transferCache := make(map[TransferKey]*dataobjects.Transfer)
	getTransfer := func(station, from, to string) (*dataobjects.Transfer, error) {
		transfer, cached := transferCache[TransferKey{station, from, to}]
		if !cached {
			var err error
			transfer, err = dataobjects.GetTransfer(tx, station, from, to)
			if err != nil {
				return nil, err
			}
			transferCache[TransferKey{station, from, to}] = transfer
		}
		return transfer, nil
	}
	// we can use pointers as keys in the following maps because getTransfer and transferCache
	// make sure only one instance of each transfer is created throughout the execution of this function
	transferAvgNumerator := make(map[*dataobjects.Transfer]float64)
	transferAvgDenominator := make(map[*dataobjects.Transfer]float64)

	type ConnectionKey struct {
		From string
		To   string
	}

	connectionCache := make(map[ConnectionKey]*dataobjects.Connection)
	getConnection := func(from, to string) (*dataobjects.Connection, error) {
		connection, cached := connectionCache[ConnectionKey{from, to}]
		if !cached {
			var err error
			connection, err = dataobjects.GetConnection(tx, from, to)
			if err != nil {
				return nil, err
			}
			connectionCache[ConnectionKey{from, to}] = connection
		}
		return connection, nil
	}
	// we can use pointers as keys in the following maps because getConnection and connectionCache
	// make sure only one instance of each connection is created throughout the execution of this function
	connectionAvgNumerator := make(map[*dataobjects.Connection]float64)
	connectionAvgDenominator := make(map[*dataobjects.Connection]float64)
	connectionStopAvgNumerator := make(map[*dataobjects.Connection]float64)
	connectionStopAvgDenominator := make(map[*dataobjects.Connection]float64)
	connectionWaitAvgNumerator := make(map[*dataobjects.Connection]float64)
	connectionWaitAvgDenominator := make(map[*dataobjects.Connection]float64)

	processTransfer := func(transfer *dataobjects.Transfer, use *dataobjects.StationUse) error {
		seconds := use.LeaveTime.Sub(use.EntryTime).Seconds()
		// if going from one line to another took more than 15 minutes,
		// probably what really happened was that the client's clock was adjusted
		// in the meantime, OR the user decided to go shop or something at the station
		if seconds < 15*60 {
			transferAvgNumerator[transfer] += seconds - 20
			transferAvgDenominator[transfer]++
		}
		return nil
	}

	processConnection := func(connection *dataobjects.Connection, sourceUse *dataobjects.StationUse, targetUse *dataobjects.StationUse) error {
		seconds := targetUse.EntryTime.Sub(sourceUse.LeaveTime).Seconds()
		// if going from one station to another took more than 10 minutes,
		// probably what really happened was that the client's clock was adjusted
		// in the meantime
		if seconds < 10*60 {
			connectionAvgNumerator[connection] += seconds + 20
			connectionAvgDenominator[connection]++
		}

		waitSeconds := sourceUse.LeaveTime.Sub(sourceUse.EntryTime).Seconds()
		if sourceUse.Type == dataobjects.NetworkEntry && waitSeconds < 60*3 {
			connectionWaitAvgNumerator[connection] += waitSeconds - 20
			connectionWaitAvgDenominator[connection]++
		} else if sourceUse.Type == dataobjects.GoneThrough && waitSeconds < 60*3 {
			connectionStopAvgNumerator[connection] += waitSeconds - 20
			connectionStopAvgDenominator[connection]++
		}
		return nil
	}

	processTrip := func(trip *dataobjects.Trip) error {
		if len(trip.StationUses) <= 1 {
			// station visit or invalid trip
			// can't extract any data about connections
			return nil
		}

		for useIdx := 0; useIdx < len(trip.StationUses)-1; useIdx++ {
			sourceUse := trip.StationUses[useIdx]

			if sourceUse.Manual {
				// manual path extensions don't contain valid time data
				// skip
				continue
			}

			// if this is a transfer, process it
			if sourceUse.Type == dataobjects.Interchange {
				transfer, err := getTransfer(sourceUse.Station.ID, sourceUse.SourceLine.ID, sourceUse.TargetLine.ID)
				if err != nil {
					// transfer might no longer exist (closed stations, etc.)
					// move on
					fmt.Printf("%s: Transfer on %s from %s to %s skipped\n", trip.ID, sourceUse.Station.ID, sourceUse.SourceLine.ID, sourceUse.TargetLine.ID)
					return nil
				}

				if err = processTransfer(transfer, sourceUse); err != nil {
					return err
				}
			}

			targetUse := trip.StationUses[useIdx+1]

			if targetUse.Manual {
				// manual path extensions don't contain valid time data
				// skip
				continue
			}

			connection, err := getConnection(sourceUse.Station.ID, targetUse.Station.ID)
			if err != nil {
				// connection might no longer exist (closed stations, etc.)
				// move on
				fmt.Printf("%s: Connection from %s to %s skipped\n", trip.ID, sourceUse.Station.ID, targetUse.Station.ID)
				continue
			}
			if useIdx+2 < len(trip.StationUses) && trip.StationUses[useIdx+2].EntryTime.Sub(targetUse.EntryTime) < 1*time.Second {
				// this station use is certainly a forced extension to make up for a station the client did not capture correct times for
				// skip
				continue
			}

			if err = processConnection(connection, sourceUse, targetUse); err != nil {
				return err
			}
		}
		return nil
	}

	// instantiate each trip from DB individually
	// (instead of using dataobjects.GetTrips)
	// to reduce memory usage
	for _, tripID := range tripIDs {
		trip, err := dataobjects.GetTrip(tx, tripID)
		if err != nil {
			return err
		}

		if err = processTrip(trip); err != nil {
			return err
		}
	}

	for connection, denominator := range connectionAvgDenominator {
		if denominator < 2 {
			// data is not significant enough
			continue
		}
		average := connectionAvgNumerator[connection] / denominator
		connection.TypicalSeconds = int(math.Round(average))
		fmt.Printf("Updating connection from %s to %s with %d (%f)\n", connection.From.ID, connection.To.ID, connection.TypicalSeconds, denominator)
		err := connection.Update(tx)
		if err != nil {
			return err
		}
	}

	for connection, denominator := range connectionStopAvgDenominator {
		if denominator < 2 {
			// data is not significant enough
			continue
		}
		average := connectionStopAvgNumerator[connection] / denominator
		connection.TypicalStopSeconds = int(math.Round(average))
		fmt.Printf("Updating connection from %s to %s with stop %d (%f)\n", connection.From.ID, connection.To.ID, connection.TypicalStopSeconds, denominator)
		err := connection.Update(tx)
		if err != nil {
			return err
		}
	}

	for connection, denominator := range connectionWaitAvgDenominator {
		if denominator < 2 {
			// data is not significant enough
			continue
		}
		average := connectionWaitAvgNumerator[connection] / denominator
		connection.TypicalWaitingSeconds = int(math.Round(average))
		fmt.Printf("Updating connection from %s to %s with wait %d (%f)\n", connection.From.ID, connection.To.ID, connection.TypicalWaitingSeconds, denominator)
		err := connection.Update(tx)
		if err != nil {
			return err
		}
	}

	for transfer, denominator := range transferAvgDenominator {
		if denominator < 2 {
			// data is not significant enough
			continue
		}
		average := transferAvgNumerator[transfer] / denominator

		// subtract average of stop times, because the pathfinding algos can't
		// deal with edges that have different weights depending on where one
		// "comes from"

		outgoingConnections := []*dataobjects.Connection{}
		for connection := range connectionStopAvgDenominator {
			if connection.From.ID == transfer.Station.ID {
				outgoingConnections = append(outgoingConnections, connection)
			}
		}

		outgoingDestConnections := []*dataobjects.Connection{}
		for _, connection := range outgoingConnections {
			lines, err := connection.To.Lines(tx)
			if err != nil {
				return err
			}
			for _, line := range lines {
				if line.ID == transfer.To.ID {
					outgoingDestConnections = append(outgoingDestConnections, connection)
					break
				}
			}
		}

		avgStopTime := 0
		for _, connection := range outgoingDestConnections {
			avgStopTime += connection.TypicalStopSeconds
		}
		if len(outgoingDestConnections) > 0 {
			average -= float64(avgStopTime) / float64(len(outgoingDestConnections))
		}

		transfer.TypicalSeconds = int(math.Round(average))
		fmt.Printf("Updating transfer from %s to %s with %d (%f)\n", transfer.From.ID, transfer.To.ID, transfer.TypicalSeconds, denominator)
		err := transfer.Update(tx)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

// ComputeAverageSpeed returns the average service speed in km/h
// based on the trips in the specified time range
func ComputeAverageSpeed(node sqalx.Node, fromTime time.Time, toTime time.Time) (float64, error) {
	tx, err := node.Beginx()
	if err != nil {
		return 0, err
	}
	defer tx.Commit() // read-only tx

	tripIDs, err := dataobjects.GetTripIDsBetween(tx, fromTime, toTime)
	if err != nil {
		return 0, err
	}

	if len(tripIDs) == 0 {
		return 0, nil
	}

	type ConnectionKey struct {
		From string
		To   string
	}

	connectionCache := make(map[ConnectionKey]*dataobjects.Connection)
	getConnection := func(from, to string) (*dataobjects.Connection, error) {
		connection, cached := connectionCache[ConnectionKey{from, to}]
		if !cached {
			var err error
			connection, err = dataobjects.GetConnection(tx, from, to)
			if err != nil {
				return nil, err
			}
			connectionCache[ConnectionKey{from, to}] = connection
		}
		return connection, nil
	}

	var totalTime time.Duration
	var totalDistance int64

	processTrip := func(trip *dataobjects.Trip) error {
		if len(trip.StationUses) <= 1 {
			// station visit or invalid trip
			// can't extract any data about connections
			return nil
		}

		var startTime, endTime time.Time
		for useIdx := 0; useIdx < len(trip.StationUses)-1; useIdx++ {
			sourceUse := trip.StationUses[useIdx]

			if sourceUse.Manual {
				// manual path extensions don't contain valid time data
				// skip
				continue
			}

			if sourceUse.Type == dataobjects.Interchange ||
				sourceUse.Type == dataobjects.Visit {
				continue
			}

			targetUse := trip.StationUses[useIdx+1]

			if targetUse.Manual {
				// manual path extensions don't contain valid time data
				// skip
				continue
			}

			connection, err := getConnection(sourceUse.Station.ID, targetUse.Station.ID)
			if err != nil {
				// connection might no longer exist (closed stations, etc.)
				// move on
				fmt.Printf("Connection from %s to %s skipped\n", sourceUse.Station.ID, targetUse.Station.ID)
				return nil
			}

			totalDistance += int64(connection.WorldLength)
			if startTime.IsZero() {
				startTime = sourceUse.LeaveTime
			}
			endTime = targetUse.EntryTime
		}
		totalTime += endTime.Sub(startTime)
		return nil
	}

	// instantiate each trip from DB individually
	// (instead of using dataobjects.GetTrips)
	// to reduce memory usage
	for _, tripID := range tripIDs {
		trip, err := dataobjects.GetTrip(tx, tripID)
		if err != nil {
			return 0, err
		}

		if err = processTrip(trip); err != nil {
			return 0, err
		}
	}

	km := float64(totalDistance) / 1000
	hours := totalTime.Hours()

	return km / hours, nil
}

// ComputeAverageSpeedCached returns the average service speed in km/h
// based on the trips in the specified time range, with cache in front
// to minimize computational cost
type avgSpeedCacheKey struct {
	From int64
	To   int64
}

var avgSpeedCache map[avgSpeedCacheKey]float64

func ComputeAverageSpeedCached(node sqalx.Node, fromTime time.Time, toTime time.Time) (float64, error) {
	if val, ok := avgSpeedCache[avgSpeedCacheKey{fromTime.Unix(), toTime.Unix()}]; ok {
		return val, nil
	}
	val, err := ComputeAverageSpeed(node, fromTime, toTime)
	if err != nil {
		return val, err
	}
	avgSpeedCache[avgSpeedCacheKey{fromTime.Unix(), toTime.Unix()}] = val
	return val, nil
}

// ComputeSimulatedRealtime looks at trips to compute a stream of entry beacons,
// as if they were being received in real time
func ComputeSimulatedRealtime(node sqalx.Node, fromTime time.Time, toTime time.Time, writer io.Writer) error {
	tx, err := node.Beginx()
	if err != nil {
		return err
	}
	defer tx.Commit() // read-only tx

	tripIDs, err := dataobjects.GetTripIDsBetween(tx, fromTime, toTime)
	if err != nil {
		return err
	}

	if len(tripIDs) == 0 {
		return nil
	}

	const NoDirectionID string = "NULL"
	type RealTimeEntry struct {
		Time        time.Time
		StationID   string
		DirectionID string
	}
	entries := []RealTimeEntry{}

	lines, err := dataobjects.GetLines(tx)
	if err != nil {
		return nil
	}

	type ConnectionKey struct {
		From string
		To   string
	}

	connectionCache := make(map[ConnectionKey]*dataobjects.Connection)
	getConnection := func(from, to string) (*dataobjects.Connection, error) {
		connection, cached := connectionCache[ConnectionKey{from, to}]
		if !cached {
			var err error
			connection, err = dataobjects.GetConnection(tx, from, to)
			if err != nil {
				return nil, err
			}
			connectionCache[ConnectionKey{from, to}] = connection
		}
		return connection, nil
	}

	processTrip := func(trip *dataobjects.Trip) error {
		nonManualCount := 0
		for useIdx := 0; useIdx < len(trip.StationUses); useIdx++ {
			curUse := trip.StationUses[useIdx]

			if curUse.Manual {
				// manual path extensions don't contain valid time data
				// skip
				continue
			}
			nonManualCount++

			if curUse.Type == dataobjects.Visit {
				entries = append(entries, RealTimeEntry{
					Time:        curUse.EntryTime,
					StationID:   curUse.Station.ID,
					DirectionID: NoDirectionID,
				})
				continue
			}

			if useIdx > 0 && nonManualCount > 1 {
				prevUse := trip.StationUses[useIdx-1]

				if useIdx+1 < len(trip.StationUses) && trip.StationUses[useIdx+1].EntryTime.Sub(curUse.EntryTime) < 1*time.Second {
					// this station use is certainly a forced extension to make up for a station the client did not capture correct times for
					// skip
					continue
				}

				connection, err := getConnection(prevUse.Station.ID, curUse.Station.ID)
				if err != nil {
					// connection might no longer exist (closed stations, etc.)
					// or it might be a transfer, or we messed up reading the station uses
					// in any case, the result is the same:
					continue
				}

				var direction *dataobjects.Station

				for _, line := range lines {
					dir, err := line.GetDirectionForConnection(tx, connection)
					if err == nil {
						direction = dir
						break
					}
				}

				if direction == nil {
					// we messed up reading the station uses...
				} else {
					entries = append(entries, RealTimeEntry{
						Time:        curUse.EntryTime,
						StationID:   curUse.Station.ID,
						DirectionID: direction.ID,
					})
				}
			} else {
				entries = append(entries, RealTimeEntry{
					Time:        curUse.EntryTime,
					StationID:   curUse.Station.ID,
					DirectionID: NoDirectionID,
				})
			}
		}
		return nil
	}

	// instantiate each trip from DB individually
	// (instead of using dataobjects.GetTrips)
	// to reduce memory usage
	for _, tripID := range tripIDs {
		trip, err := dataobjects.GetTrip(tx, tripID)
		if err != nil {
			return err
		}

		if err = processTrip(trip); err != nil {
			return err
		}
	}

	sort.Slice(entries, func(iidx, jidx int) bool {
		i := entries[iidx]
		j := entries[jidx]
		// i < j ?
		return i.Time.Before(j.Time)
	})

	writer.Write([]byte("time,station,direction\n"))
	for _, entry := range entries {
		writer.Write([]byte(fmt.Sprintf("%d,%s,%s\n", entry.Time.Unix(), entry.StationID, entry.DirectionID)))
	}
	return nil
}

func init() {
	avgSpeedCache = make(map[avgSpeedCacheKey]float64)
}
