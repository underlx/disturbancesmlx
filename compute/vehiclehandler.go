package compute

import (
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/gbl08ma/sqalx"
	"github.com/underlx/disturbancesmlx/dataobjects"

	altmath "github.com/pkg/math"
)

// VehicleHandler implements resource.RealtimeVehicleHandler
type VehicleHandler struct {
	readings                      []PassengerReading
	presenceByStationAndDirection map[string]time.Time
	connectionDurationCache       map[string]int
}

// NewVehicleHandler returns a new, initialized VehicleHandler
func NewVehicleHandler() *VehicleHandler {
	vh := new(VehicleHandler)
	vh.presenceByStationAndDirection = make(map[string]time.Time)
	vh.connectionDurationCache = make(map[string]int)
	return vh
}

// PassengerReading represents a datapoint of real-time information as submitted by a user
type PassengerReading struct {
	Time        time.Time
	StationID   string
	DirectionID string
}

// ClearTypicalSecondsCache clears a cache used for ETA computation
func (h *VehicleHandler) ClearTypicalSecondsCache() {
	h.connectionDurationCache = make(map[string]int)
}

// RegisterTrainPassenger registers the presence of a user in a station
func (h *VehicleHandler) RegisterTrainPassenger(currentStation *dataobjects.Station, direction *dataobjects.Station) {
	h.presenceByStationAndDirection[h.getMapKey(currentStation, direction)] = time.Now()

	h.readings = append(h.readings, PassengerReading{
		Time:        time.Now(),
		StationID:   currentStation.ID,
		DirectionID: direction.ID,
	})

	// preserve last 100 entries
	h.readings = h.readings[altmath.Max(0, len(h.readings)-100):len(h.readings)]
}

// Readings returns the currently stored PassengerReadings
func (h *VehicleHandler) Readings() []PassengerReading {
	return h.readings
}

// NextTrainETA makes a best-effort calculation of the ETA to the next train at the specified station going in the specified direction
func (h *VehicleHandler) NextTrainETA(node sqalx.Node, station *dataobjects.Station, direction *dataobjects.Station) (time.Duration, error) {
	tx, err := node.Beginx()
	if err != nil {
		return 0, err
	}
	defer tx.Commit() // read-only tx

	lines, err := station.Lines(tx)
	if err != nil {
		return 0, err
	}
	thisLineStations := []*dataobjects.Station{}
	// whether, in thisLineStations, the current direction is the last index (true) or zero (false)
	// i.e., whether the caller is asking for the ETA in the direction that corresponds to moving "up" in the slice
	var movingUp bool
	for _, line := range lines {
		lineStations, err := line.Stations(tx)
		if err != nil {
			return 0, err
		}
		if lineStations[0].ID == direction.ID {
			thisLineStations = lineStations
			movingUp = false
		} else if lineStations[len(lineStations)-1].ID == direction.ID {
			thisLineStations = lineStations
			movingUp = true
		}
	}
	if len(thisLineStations) == 0 {
		return 0, errors.New("Invalid direction for the specified station")
	}

	cursor := 0
	for ; cursor < len(thisLineStations); cursor++ {
		if thisLineStations[cursor].ID == station.ID {
			break
		}
	}
	userAtIdx := cursor

	getConnectionDuration := func(from, to string) int {
		if s, present := h.connectionDurationCache[from+"#"+to]; present {
			return s
		}
		connection, err := dataobjects.GetConnection(tx, from, to)
		if err != nil {
			return 0
		}
		s := connection.TypicalSeconds + connection.TypicalStopSeconds
		h.connectionDurationCache[from+"#"+to] = s
		return s
	}

	getTimeDistance := func(trainAtIdx int, movingUp, trainMovingUp bool) int {
		totalSeconds := 0
		if trainMovingUp {
			for i := trainAtIdx; (i <= userAtIdx-1 || trainMovingUp != movingUp) && i < len(thisLineStations)-1; i++ {
				totalSeconds += getConnectionDuration(thisLineStations[i].ID, thisLineStations[i+1].ID)
			}
		} else {
			for i := trainAtIdx; (i >= userAtIdx+1 || trainMovingUp != movingUp) && i > 0; i-- {
				totalSeconds += getConnectionDuration(thisLineStations[i].ID, thisLineStations[i-1].ID)
			}
		}

		if trainMovingUp != movingUp {
			// the next train is still on the opposite direction
			trainMovingUp = !trainMovingUp

			totalSeconds += 120 // TODO calculate inversion time

			if trainMovingUp {
				for i := 0; i <= userAtIdx-1 && i < len(thisLineStations)-1; i++ {
					totalSeconds += getConnectionDuration(thisLineStations[i].ID, thisLineStations[i+1].ID)
				}
			} else {
				for i := len(thisLineStations) - 1; i >= userAtIdx+1 && i > 0; i-- {
					totalSeconds += getConnectionDuration(thisLineStations[i].ID, thisLineStations[i-1].ID)
				}
			}
		}
		return totalSeconds
	}

	// let's find at which station the next train is right now (or from which station it just departed)
	trainAtSeconds := int(^uint(0) >> 1) // max signed int
	minDistance := int(^uint(0) >> 1)    // max signed int
	trainAtIdx := -1
	var trainMovingUp bool
	curTime := time.Now()
	foundTrain := false
	if movingUp {
		// look for trains going "up" before this station
		for ; cursor >= 0; cursor-- {
			t, present := h.presenceByStationAndDirection[h.getMapKey(thisLineStations[cursor], direction)]
			tInt := int(math.Round(curTime.Sub(t).Seconds()))
			tDistance := getTimeDistance(cursor, movingUp, true) - tInt
			if present && tDistance < minDistance && tDistance >= -30 {
				trainAtSeconds = tInt
				minDistance = tDistance
				trainAtIdx = cursor
				trainMovingUp = true
				foundTrain = true
			} else if present && tDistance > minDistance {
				foundTrain = true
				break
			}
		}
		if !foundTrain {
			// invert direction and begin looking for trains that are going "down"
			oppositeDirection := thisLineStations[0]
			for cursor++; cursor < len(thisLineStations); cursor++ {
				t, present := h.presenceByStationAndDirection[h.getMapKey(thisLineStations[cursor], oppositeDirection)]
				tInt := int(math.Round(curTime.Sub(t).Seconds()))
				tDistance := getTimeDistance(cursor, movingUp, false) - tInt
				if present && tDistance < minDistance && tDistance >= -30 {
					trainAtSeconds = tInt
					minDistance = tDistance
					trainAtIdx = cursor
					trainMovingUp = false
					foundTrain = true
				} else if present && tDistance > minDistance {
					foundTrain = true
					break
				}
			}
		}
	} else {
		// look for trains going "down" before this station
		for ; cursor < len(thisLineStations); cursor++ {
			t, present := h.presenceByStationAndDirection[h.getMapKey(thisLineStations[cursor], direction)]
			tInt := int(math.Round(curTime.Sub(t).Seconds()))
			tDistance := getTimeDistance(cursor, movingUp, false) - tInt
			if present && tDistance < minDistance && tDistance >= -30 {
				trainAtSeconds = tInt
				minDistance = tDistance
				trainAtIdx = cursor
				trainMovingUp = false
				foundTrain = true
			} else if present && tDistance > minDistance {
				foundTrain = true
				break
			}
		}
		if !foundTrain {
			// invert direction and begin looking for trains that are going "up"
			oppositeDirection := thisLineStations[len(thisLineStations)-1]
			for cursor--; cursor >= 0; cursor-- {
				t, present := h.presenceByStationAndDirection[h.getMapKey(thisLineStations[cursor], oppositeDirection)]
				tInt := int(math.Round(curTime.Sub(t).Seconds()))
				tDistance := getTimeDistance(cursor, movingUp, true) - tInt
				if present && tDistance < minDistance && tDistance >= -30 {
					trainAtSeconds = tInt
					minDistance = tDistance
					trainAtIdx = cursor
					trainMovingUp = true
					foundTrain = true
				} else if present && tDistance > minDistance {
					foundTrain = true
					break
				}
			}
		}
	}

	if !foundTrain {
		// not yet sure in which conditions this could happen, probably when there is too little data?
		return 0, errors.New("Could not find next train")
	}

	// now compute the time for the next train to travel from the station where it currently is
	// then maybe subtract trainAtSeconds but add the avg stop time for the station where it currently is
	// will certainly need manual adjustment/"magic constant"

	totalSeconds := 0
	if trainMovingUp {
		for cursor = trainAtIdx; (cursor <= userAtIdx-1 || trainMovingUp != movingUp) && cursor < len(thisLineStations)-1; cursor++ {
			totalSeconds += getConnectionDuration(thisLineStations[cursor].ID, thisLineStations[cursor+1].ID)
		}
	} else {
		for cursor = trainAtIdx; (cursor >= userAtIdx+1 || trainMovingUp != movingUp) && cursor > 0; cursor-- {
			totalSeconds += getConnectionDuration(thisLineStations[cursor].ID, thisLineStations[cursor-1].ID)
		}
	}

	if trainMovingUp != movingUp {
		// the next train is still on the opposite direction
		trainMovingUp = !trainMovingUp

		totalSeconds += 120 // TODO calculate inversion time

		if trainMovingUp {
			for cursor = 0; cursor <= userAtIdx-1 && cursor < len(thisLineStations)-1; cursor++ {
				totalSeconds += getConnectionDuration(thisLineStations[cursor].ID, thisLineStations[cursor+1].ID)
			}
		} else {
			for cursor = len(thisLineStations) - 1; cursor >= userAtIdx+1 && cursor > 0; cursor-- {
				totalSeconds += getConnectionDuration(thisLineStations[cursor].ID, thisLineStations[cursor-1].ID)
			}
		}
	}

	totalSeconds -= trainAtSeconds

	return time.Duration(totalSeconds) * time.Second, nil
}

// TrainETA contains information about the estimated arrival time of a train for a station and direction
type TrainETA struct {
	Station   *dataobjects.Station
	Direction *dataobjects.Station
	ETA       string
}

// AllTrainETAs computes the train ETA for all stations in all directions
func (h *VehicleHandler) AllTrainETAs(node sqalx.Node) ([]TrainETA, error) {
	tx, err := node.Beginx()
	if err != nil {
		return []TrainETA{}, err
	}
	defer tx.Commit() // read-only tx

	stations, err := dataobjects.GetStations(tx)
	if err != nil {
		return []TrainETA{}, err
	}
	trainETAs := []TrainETA{}
	for _, station := range stations {
		directions, err := station.Directions(tx)
		if err != nil {
			return trainETAs, err
		}
		for _, direction := range directions {
			eta, err := h.NextTrainETA(tx, station, direction)
			if err != nil {
				trainETAs = append(trainETAs, TrainETA{station, direction, err.Error()})
			} else if eta.Seconds() < 0 {
				trainETAs = append(trainETAs, TrainETA{station, direction, fmt.Sprintf("probably arrived %s ago", (-eta).String())})
			} else {
				trainETAs = append(trainETAs, TrainETA{station, direction, eta.String()})
			}
		}
	}
	return trainETAs, nil
}

func (h *VehicleHandler) getMapKey(station *dataobjects.Station, direction *dataobjects.Station) string {
	return station.ID + "#" + direction.ID
}
