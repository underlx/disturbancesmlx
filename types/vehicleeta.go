package types

import (
	"errors"
	"strconv"
	"time"

	"github.com/gbl08ma/sqalx"
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
	TransportUnits   int // number of train cars or equivalent. 0 is unknown or N/A
	VehicleServiceID string
	Platform         string
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

// VehicleIDisSpecial returns whether the vehicle ID of this ETA corresponds to a special service
func (eta *VehicleETA) VehicleIDisSpecial() bool {
	v := eta.VehicleServiceID
	return len(v) < 1 || (v[0] < '0' && v[0] > '9')
}

// VehicleIDgetLine returns the service line of the vehicle for this ETA
func (eta *VehicleETA) VehicleIDgetLine(node sqalx.Node) (*Line, error) {
	id, err := eta.VehicleIDgetLineString()
	if err != nil {
		return nil, err
	}
	return GetLineWithExternalID(node, id)
}

// VehicleIDgetLineString returns the service line of the vehicle for this ETA
func (eta *VehicleETA) VehicleIDgetLineString() (string, error) {
	v := eta.VehicleServiceID
	if len(v) == 0 {
		return "", errors.New("Empty vehicle ID")
	}
	id := ""
	if eta.VehicleIDisSpecial() {
		id = v[0:1]
	} else {
		id = v[len(v)-1 : len(v)]
	}
	return id, nil
}

// VehicleIDgetNumber returns the service number of the vehicle for this ETA
func (eta *VehicleETA) VehicleIDgetNumber() int {
	v := eta.VehicleServiceID
	if len(v) == 0 {
		return 0
	}
	if eta.VehicleIDisSpecial() {
		n, _ := strconv.Atoi(v[1:len(v)])
		return n
	}
	n, _ := strconv.Atoi(v[0 : len(v)-1])
	return n
}

// VehicleIDLessFunc is a function for comparing vehicle ETAs when sorting by vehicle ID
func VehicleIDLessFunc(vi, vj *VehicleETA) bool {
	if vi.VehicleIDisSpecial() && !vj.VehicleIDisSpecial() {
		return true
	}
	if !vi.VehicleIDisSpecial() && vj.VehicleIDisSpecial() {
		return false
	}
	li, _ := vi.VehicleIDgetLineString()
	lj, _ := vj.VehicleIDgetLineString()
	if li < lj {
		return true
	}
	if li > lj {
		return false
	}
	return vi.VehicleIDgetNumber() < vj.VehicleIDgetNumber()
}

// VehicleIDLessFuncString is a function for comparing vehicle IDs when sorting
func VehicleIDLessFuncString(vi, vj string) bool {
	return VehicleIDLessFunc(&VehicleETA{
		VehicleServiceID: vi,
	},
		&VehicleETA{
			VehicleServiceID: vj,
		})
}

// durationMax is math.Max for time.Duration
func durationMax(a, b time.Duration) time.Duration {
	if a > b {
		return a
	}
	return b
}
