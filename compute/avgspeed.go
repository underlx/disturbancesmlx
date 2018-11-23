package compute

import (
	"errors"
	"runtime"
	"time"

	"github.com/gbl08ma/sqalx"
	"github.com/underlx/disturbancesmlx/dataobjects"
)

// ErrInfoNotReady is returned when the requested information is not yet available
var ErrInfoNotReady = errors.New("information not ready")

type avgSpeedCacheKey struct {
	From int64
	To   int64
}

var avgSpeedCache map[avgSpeedCacheKey]float64
var avgSpeedComputeInProgress map[avgSpeedCacheKey]bool

func init() {
	avgSpeedCache = make(map[avgSpeedCacheKey]float64)
	avgSpeedComputeInProgress = make(map[avgSpeedCacheKey]bool)
}

// AverageSpeed returns the average service speed in km/h
// based on the trips in the specified time range
func AverageSpeed(node sqalx.Node, fromTime time.Time, toTime time.Time, yieldFor time.Duration) (float64, error) {
	return AverageSpeedFilter(node, fromTime, toTime, yieldFor, func(trip *dataobjects.Trip) bool { return true })
}

// AverageSpeedFilter returns the average service speed in km/h
// based on the trips in the specified time range that match the provided filter
func AverageSpeedFilter(node sqalx.Node, fromTime time.Time, toTime time.Time, yieldFor time.Duration, filter func(trip *dataobjects.Trip) bool) (float64, error) {
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

	var totalTime time.Duration
	var totalDistance int64

	processTrip := func(trip *dataobjects.Trip) {
		if !filter(trip) {
			return
		}
		_, distance, duration, err := trip.AverageSpeed(tx)
		if err != nil {
			return
		}
		totalDistance += distance
		totalTime += duration
	}

	// instantiate each trip from DB individually
	// (instead of using dataobjects.GetTrips)
	// to reduce memory usage
	for _, tripID := range tripIDs {
		trip, err := dataobjects.GetTrip(tx, tripID)
		if err != nil {
			return 0, err
		}

		processTrip(trip)

		if yieldFor > 0 {
			time.Sleep(yieldFor)
		}
	}

	km := float64(totalDistance) / 1000
	hours := totalTime.Hours()

	return km / hours, nil
}

// AverageSpeedCached returns the average speed of trips within the specified period,
// if it has been computed already, or begins computing it, if it has not. Returns
// ErrInfoNotReady in the latter case, or the average speed in the former case.
func AverageSpeedCached(node sqalx.Node, fromTime time.Time, toTime time.Time) (float64, error) {
	if val, ok := avgSpeedCache[avgSpeedCacheKey{fromTime.Unix(), toTime.Unix()}]; ok {
		return val, nil
	}

	if !avgSpeedComputeInProgress[avgSpeedCacheKey{fromTime.Unix(), toTime.Unix()}] {
		go func() {
			mainLog.Println("Now computing average speed between " + fromTime.String() + " and " + toTime.String())
			avgSpeedComputeInProgress[avgSpeedCacheKey{fromTime.Unix(), toTime.Unix()}] = true
			val, err := AverageSpeed(rootSqalxNode, fromTime, toTime, 5*time.Millisecond)
			if err != nil {
				mainLog.Println("Error computing average speed between " + fromTime.String() + " and " + toTime.String() + ": " + err.Error())
				return
			}
			avgSpeedCache[avgSpeedCacheKey{fromTime.Unix(), toTime.Unix()}] = val
			avgSpeedComputeInProgress[avgSpeedCacheKey{fromTime.Unix(), toTime.Unix()}] = false
			mainLog.Println("Average speed between " + fromTime.String() + " and " + toTime.String() + " computed")
		}()
		runtime.Gosched()
	}

	return 0, ErrInfoNotReady
}
