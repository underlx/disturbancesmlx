package mqttgateway

import (
	"math"
	"time"

	"gopkg.in/vmihailenco/msgpack.v2"
)

const (
	vehicleETATypeNotAvailable = "n"
	vehicleETATypeExact        = "e"
	vehicleETATypeInterval     = "i"
	vehicleETATypeLessThan     = "l"
	vehicleETATypeMoreThan     = "m"
	vehicleETATypeTimestamp    = "t"
)

const (
	vehicleETAUnitSeconds = "s"
	vehicleETAUnitMinutes = "m"
)

type vehicleETA struct {
	Direction string `msgpack:"direction" json:"direction"`
	Made      int64  `msgpack:"made" json:"made"`         // always a unix timestamp (seconds)
	ValidFor  uint   `msgpack:"validFor" json:"validFor"` // always in seconds
	Type      string `msgpack:"type" json:"type"`
	Units     string `msgpack:"units" json:"units"`
}

type vehicleETASingleValue struct {
	vehicleETA `msgpack:",inline"`
	Value      uint `msgpack:"value" json:"value"`
}

type vehicleETAInterval struct {
	vehicleETA `msgpack:",inline"`
	Lower      uint `msgpack:"lower" json:"lower"`
	Upper      uint `msgpack:"upper" json:"upper"`
}

func buildVehicleETAExactStruct(direction string, validFor time.Duration, eta time.Duration, precise bool) vehicleETASingleValue {
	data := vehicleETASingleValue{
		vehicleETA: vehicleETA{
			Direction: direction,
			Made:      time.Now().Unix(),
			ValidFor:  uint(validFor.Seconds()),
			Type:      vehicleETATypeExact,
		},
	}

	if precise {
		data.Units = vehicleETAUnitSeconds
		data.Value = uint(eta.Seconds())
	} else {
		data.Units = vehicleETAUnitMinutes
		data.Value = uint(math.Round(eta.Minutes()))
	}

	return data
}

func buildVehicleETAIntervalStruct(direction string, validFor time.Duration, lower, upper time.Duration, precise bool) vehicleETAInterval {
	data := vehicleETAInterval{
		vehicleETA: vehicleETA{
			Direction: direction,
			Made:      time.Now().Unix(),
			ValidFor:  uint(validFor.Seconds()),
			Type:      vehicleETATypeInterval,
		},
	}

	if precise {
		data.Units = vehicleETAUnitSeconds
		data.Lower = uint(lower.Seconds())
		data.Upper = uint(upper.Seconds())
	} else {
		data.Units = vehicleETAUnitMinutes
		data.Lower = uint(math.Round(lower.Minutes()))
		data.Upper = uint(math.Round(upper.Minutes()))
	}

	return data
}

func buildVehicleETAPayload(structs ...interface{}) []byte {
	encoded, err := msgpack.Marshal(structs)
	if err != nil {
		return []byte{}
	}
	return encoded
}
