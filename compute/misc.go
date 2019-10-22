package compute

import (
	"fmt"
	"io"
	"sort"
	"time"

	"github.com/gbl08ma/sqalx"
	"github.com/underlx/disturbancesmlx/types"
)

// SimulateRealtime looks at trips to compute a stream of entry beacons,
// as if they were being received in real time, and outputs a CSV to writer
func SimulateRealtime(node sqalx.Node, fromTime time.Time, toTime time.Time, writer io.Writer) error {
	tx, err := node.Beginx()
	if err != nil {
		return err
	}
	defer tx.Commit() // read-only tx

	tripIDs, err := types.GetTripIDsBetween(tx, fromTime, toTime)
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

	lines, err := types.GetLines(tx)
	if err != nil {
		return nil
	}

	processTrip := func(trip *types.Trip) error {
		nonManualCount := 0
		for useIdx := 0; useIdx < len(trip.StationUses); useIdx++ {
			curUse := trip.StationUses[useIdx]

			if curUse.Manual {
				// manual path extensions don't contain valid time data
				// skip
				continue
			}
			nonManualCount++

			if curUse.Type == types.Visit {
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

				connection, err := types.GetConnection(tx, prevUse.Station.ID, curUse.Station.ID, true)
				if err != nil {
					// connection might no longer exist (closed stations, etc.)
					// or it might be a transfer, or we messed up reading the station uses
					// in any case, the result is the same:
					continue
				}

				var direction *types.Station

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
	// (instead of using types.GetTrips)
	// to reduce memory usage
	for _, tripID := range tripIDs {
		trip, err := types.GetTrip(tx, tripID)
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
