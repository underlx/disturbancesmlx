package dataobjects

import (
	"time"
)

// VehicleETAType is a type of VehicleETA
type VehicleETAType int

const (
	// Absolute is an ETA with a defined absolute time (e.g. for scheduled vehicles, like a train arriving at 22:43 on September 9 2020...)
	Absolute VehicleETAType = iota

	// RelativeExact is a relative ETA that is exact and known (e.g. the next bus will arrive in 5 minutes)
	RelativeExact

	// RelativeRange is a relative ETA that has a lower and upper bound (e.g. the bus will arrive within 3 to 6 minutes)
	RelativeRange

	// RelativeMinimum is a relative ETA that has a lower bound and no upper bound (e.g. the train will not arrive within the next hour)
	RelativeMinimum

	// RelativeMaximum is a relative ETA that has an upper bound only (e.g. the train will arrive within 10 minutes - but it could arrive much sooner)
	RelativeMaximum
)

// VehicleETA defines the estimated arrival time at Station for a vehicle going in Direction
type VehicleETA struct {
	Station          *Station
	Direction        *Station
	ArrivalOrder     int // 0 is N/A, 1 is the next vehicle, 2 is the vehicle after the next one, etc.
	VehicleServiceID string
	Computed         time.Time
	ValidFor         time.Duration
	Type             VehicleETAType

	// the following fields are (un)used depending on Type
	AbsoluteETA   time.Time
	eta           time.Duration
	etaLowerBound time.Duration
	etaUpperBound time.Duration

	Precision time.Duration
}

// RemainingValidity returns an adjusted ValidFor based on how much time elapsed since this ETA was computed/received
func (eta *VehicleETA) RemainingValidity() time.Duration {
	return durationMax(0, eta.ValidFor-time.Since(eta.Computed))
}

// SetETA sets the eta field and automatically sets the Type to RelativeExact
func (eta *VehicleETA) SetETA(d time.Duration) {
	eta.Type = RelativeExact
	eta.eta = d
}

// LiveETA returns an adjusted ETA based on how much time elapsed since this ETA was computed/received
func (eta *VehicleETA) LiveETA() time.Duration {
	// for this to work correctly, eta.Computed must be based in our system's clock
	return durationMax(0, eta.eta-time.Since(eta.Computed))
}

// SetETALowerBound sets the etaLowerBound field
func (eta *VehicleETA) SetETALowerBound(d time.Duration) {
	eta.etaLowerBound = d
}

// LiveETAlowerBound returns an adjusted ETAlowerBound based on how much time elapsed since this ETA was computed/received
func (eta *VehicleETA) LiveETAlowerBound() time.Duration {
	// for this to work correctly, eta.Computed must be based in our system's clock
	return durationMax(0, eta.etaLowerBound-time.Since(eta.Computed))
}

// SetETAUpperBound sets the etaUpperBound field
func (eta *VehicleETA) SetETAUpperBound(d time.Duration) {
	eta.etaUpperBound = d
}

// LiveETAupperBound returns an adjusted ETAupperBound based on how much time elapsed since this ETA was computed/received
func (eta *VehicleETA) LiveETAupperBound() time.Duration {
	// for this to work correctly, eta.Computed must be based in our system's clock
	return durationMax(0, eta.etaUpperBound-time.Since(eta.Computed))
}

// durationMax is math.Max for time.Duration
func durationMax(a, b time.Duration) time.Duration {
	if a > b {
		return a
	}
	return b
}
