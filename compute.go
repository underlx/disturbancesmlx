package main

import (
	"fmt"
	"time"

	"github.com/gbl08ma/disturbancesmlx/dataobjects"
	"github.com/heetch/sqalx"
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

	processTransfer := func(transfer *dataobjects.Transfer, use *dataobjects.StationUse) error {
		seconds := use.LeaveTime.Sub(use.EntryTime).Seconds()
		transferAvgNumerator[transfer] += seconds
		transferAvgDenominator[transfer]++
		return nil
	}

	processConnection := func(connection *dataobjects.Connection, sourceUse *dataobjects.StationUse, targetUse *dataobjects.StationUse) error {
		seconds := targetUse.EntryTime.Sub(sourceUse.LeaveTime).Seconds()
		connectionAvgNumerator[connection] += seconds
		connectionAvgDenominator[connection]++
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
					fmt.Printf("Transfer on %s from %s to %s skipped\n", sourceUse.Station.ID, sourceUse.SourceLine.ID, sourceUse.TargetLine.ID)
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
				fmt.Printf("Connection from %s to %s skipped\n", sourceUse.Station.ID, targetUse.Station.ID)
				return nil
			}
			if useIdx+2 < len(trip.StationUses) && trip.StationUses[useIdx+2].EntryTime.Sub(targetUse.EntryTime) < 1*time.Second {
				// this station use is certainly a forced extension to make up for a station the client did not capture correct times for
				// skip
				return nil
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

	for transfer, denominator := range transferAvgDenominator {
		if denominator < 2 {
			// data is not significant enough
			continue
		}
		average := transferAvgNumerator[transfer] / denominator
		// TODO: add math.Round to int cast once Go 1.10 is released
		transfer.TypicalSeconds = int(average)
		fmt.Printf("Updating transfer from %s to %s with %d (%f)\n", transfer.From.ID, transfer.To.ID, transfer.TypicalSeconds, denominator)
		err := transfer.Update(tx)
		if err != nil {
			return err
		}
	}

	for connection, denominator := range connectionAvgDenominator {
		if denominator < 2 {
			// data is not significant enough
			continue
		}
		average := connectionAvgNumerator[connection] / denominator
		// TODO: add math.Round to int cast once Go 1.10 is released
		connection.TypicalSeconds = int(average)
		fmt.Printf("Updating connection from %s to %s with %d (%f)\n", connection.From.ID, connection.To.ID, connection.TypicalSeconds, denominator)
		err := connection.Update(tx)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}
