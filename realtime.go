package main

import (
	"time"

	"github.com/gbl08ma/disturbancesmlx/dataobjects"
	"github.com/pkg/math"
)

var vehicleHandler = new(VehicleHandler)

// VehicleHandler implements resource.RealtimeVehicleHandler
type VehicleHandler struct {
	readings []PassengerReading
}

type PassengerReading struct {
	Time        time.Time
	StationID   string
	DirectionID string
}

func (h *VehicleHandler) RegisterTrainPassenger(currentStation *dataobjects.Station, direction *dataobjects.Station) {
	h.readings = append(h.readings, PassengerReading{
		Time:        time.Now(),
		StationID:   currentStation.ID,
		DirectionID: direction.ID,
	})

	// preserve last 100 entries
	h.readings = h.readings[math.Max(0, len(h.readings)-100):len(h.readings)]
}

func (h *VehicleHandler) GetReadings() []PassengerReading {
	return h.readings
}
