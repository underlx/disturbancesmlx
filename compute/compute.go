package compute

import (
	"log"
	"math"
	"sort"
	"sync"
	"time"

	"github.com/gbl08ma/sqalx"
	"github.com/underlx/disturbancesmlx/types"
)

var rootSqalxNode sqalx.Node
var mainLog *log.Logger

// Initialize initializes the package
func Initialize(snode sqalx.Node, log *log.Logger) {
	rootSqalxNode = snode
	mainLog = log
}

// TripsScatterplotNumTripsVsAvgSpeedPoint represents a datapoint for the TripsScatterplotNumTripsVsAvgSpeed scatterplot
type TripsScatterplotNumTripsVsAvgSpeedPoint struct {
	DayOfWeek    time.Weekday
	Hour         int
	NumUsers     int
	AverageSpeed float64
}

// TripsScatterplotNumTripsVsAvgSpeed returns data for a scatterplot showing possible relations between the number of users in the network and the average trip speed
func TripsScatterplotNumTripsVsAvgSpeed(node sqalx.Node, fromTime time.Time, toTime time.Time, threads int) ([]TripsScatterplotNumTripsVsAvgSpeedPoint, error) {
	tx, err := node.Beginx()
	if err != nil {
		return []TripsScatterplotNumTripsVsAvgSpeedPoint{}, err
	}
	defer tx.Commit() // read-only tx

	tripIDs, err := types.GetTripIDsBetween(tx, fromTime, toTime)
	if err != nil {
		return []TripsScatterplotNumTripsVsAvgSpeedPoint{}, err
	}

	if len(tripIDs) == 0 {
		return []TripsScatterplotNumTripsVsAvgSpeedPoint{}, nil
	}
	points := []TripsScatterplotNumTripsVsAvgSpeedPoint{}

	processTrips := func(tripIDsPart []string) ([]TripsScatterplotNumTripsVsAvgSpeedPoint, error) {
		tx, err := rootSqalxNode.Beginx()
		if err != nil {
			return []TripsScatterplotNumTripsVsAvgSpeedPoint{}, err
		}
		defer tx.Commit() // read-only tx
		// instantiate each trip from DB individually
		// (instead of using types.GetTrips)
		// to reduce memory usage
		thisPoints := []TripsScatterplotNumTripsVsAvgSpeedPoint{}
		for _, tripID := range tripIDsPart {
			trip, err := types.GetTrip(tx, tripID)
			if err != nil {
				return []TripsScatterplotNumTripsVsAvgSpeedPoint{}, err
			}
			if trip.EndTime.Sub(trip.StartTime) > 2*time.Hour {
				continue
			}

			point := TripsScatterplotNumTripsVsAvgSpeedPoint{
				DayOfWeek: trip.StartTime.Weekday(),
				Hour:      trip.StartTime.Hour(),
			}

			simTrips, err := trip.SimultaneousTripIDs(tx, 2*time.Hour)
			if err != nil {
				return []TripsScatterplotNumTripsVsAvgSpeedPoint{}, err
			}
			point.NumUsers = len(simTrips)
			if err != nil {
				return []TripsScatterplotNumTripsVsAvgSpeedPoint{}, err
			}
			// round to multiples of 5 so there aren't too many "buckets" on the axis
			point.NumUsers = int(math.Round(float64(point.NumUsers)/5) * 5)

			point.AverageSpeed, _, _, err = trip.AverageSpeed(tx)
			if err != nil || point.AverageSpeed > 50 || point.AverageSpeed < 10 || math.IsNaN(point.AverageSpeed) {
				continue
			}
			// round to units so there aren't too many "buckets" on the axis
			point.AverageSpeed = math.Round(point.AverageSpeed)
			thisPoints = append(thisPoints, point)
		}
		return thisPoints, nil
	}

	var wg sync.WaitGroup
	var appendMutex sync.Mutex
	launched := 0
	for index := 0; index < len(tripIDs); index += 1000 {
		wg.Add(1)
		go func(start, end int) {
			if end > len(tripIDs) {
				end = len(tripIDs)
			}
			p, err := processTrips(tripIDs[start:end])
			if err != nil {
				mainLog.Fatalln(err)
			}
			appendMutex.Lock()
			points = append(points, p...)
			appendMutex.Unlock()
			mainLog.Println("TripsScatterplotNumTripsVsAvgSpeed processed", start, "to", end-1)
			wg.Done()
		}(index, index+1000)
		launched++
		if launched >= threads {
			wg.Wait()
			launched = 0
		}
	}
	wg.Wait()

	sort.Slice(points, func(i, j int) bool {
		if points[i].DayOfWeek == points[j].DayOfWeek {
			if points[i].Hour == points[j].Hour {
				return points[i].NumUsers < points[j].NumUsers
			}
			return points[i].Hour < points[j].Hour
		}
		return points[i].DayOfWeek < points[j].DayOfWeek
	})
	return points, nil
}
