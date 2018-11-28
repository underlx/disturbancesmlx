package compute

import (
	"math"
	"time"

	"github.com/gbl08ma/sqalx"
	"github.com/underlx/disturbancesmlx/dataobjects"
)

// UpdateTypicalSeconds calculates and updates the TypicalSeconds
// for all the Connections and Transfers where that can be done using the registered
// Trips from the past month.
// Current TypicalSeconds are ignored and discarded.
func UpdateTypicalSeconds(node sqalx.Node, yieldFor time.Duration) error {
	tx, err := node.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	startTime := time.Now().AddDate(0, -1, 0)
	tripIDs, err := dataobjects.GetTripIDsBetween(tx, startTime, time.Now())
	if err != nil {
		return err
	}

	mainLog.Printf("UpdateTypicalSeconds: %d trip IDs\n", len(tripIDs))

	// we can use pointers as keys in the following maps because dataobjects implements an internal cache
	// that ensures the pointers to the transfers stay the same throughout this transaction
	// (i.e. only one instance of each transfer is brought into memory)
	transferAvgNumerator := make(map[*dataobjects.Transfer]float64)
	transferAvgDenominator := make(map[*dataobjects.Transfer]float64)

	// we can use pointers as keys in the following maps because dataobjects implements an internal cache
	// that ensures the pointers to the transfers stay the same throughout this transaction
	// (i.e. only one instance of each connection is brought into memory)
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
				transfer, err := dataobjects.GetTransfer(tx, sourceUse.Station.ID, sourceUse.SourceLine.ID, sourceUse.TargetLine.ID)
				if err != nil {
					// transfer might no longer exist (closed stations, etc.)
					// move on
					mainLog.Printf("%s: Transfer on %s from %s to %s skipped\n", trip.ID, sourceUse.Station.ID, sourceUse.SourceLine.ID, sourceUse.TargetLine.ID)
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

			connection, err := dataobjects.GetConnection(tx, sourceUse.Station.ID, targetUse.Station.ID)
			if err != nil {
				// connection might no longer exist (closed stations, etc.)
				// move on
				mainLog.Printf("%s: Connection from %s to %s skipped\n", trip.ID, sourceUse.Station.ID, targetUse.Station.ID)
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

		if yieldFor > 0 {
			time.Sleep(yieldFor)
		}
	}

	for connection, denominator := range connectionAvgDenominator {
		if denominator < 2 {
			// data is not significant enough
			continue
		}
		average := connectionAvgNumerator[connection] / denominator
		connection.TypicalSeconds = int(math.Round(average))
		mainLog.Printf("Updating connection from %s to %s with %d (%f)\n", connection.From.ID, connection.To.ID, connection.TypicalSeconds, denominator)
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
		mainLog.Printf("Updating connection from %s to %s with stop %d (%f)\n", connection.From.ID, connection.To.ID, connection.TypicalStopSeconds, denominator)
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
		mainLog.Printf("Updating connection from %s to %s with wait %d (%f)\n", connection.From.ID, connection.To.ID, connection.TypicalWaitingSeconds, denominator)
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
		mainLog.Printf("Updating transfer from %s to %s with %d (%f)\n", transfer.From.ID, transfer.To.ID, transfer.TypicalSeconds, denominator)
		err := transfer.Update(tx)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}
