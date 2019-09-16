package mqttgateway

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/gbl08ma/gmqtt"
	"github.com/gbl08ma/gmqtt/pkg/packets"
	"github.com/gbl08ma/sqalx"
	"github.com/underlx/disturbancesmlx/dataobjects"
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
	Order     uint   `msgpack:"order" json:"order"`
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

func buildVehicleETATimestampStruct(direction string, made time.Time, validFor time.Duration, eta time.Time, order uint) vehicleETASingleValue {
	return vehicleETASingleValue{
		vehicleETA: vehicleETA{
			Direction: direction,
			Made:      made.Unix(),
			ValidFor:  uint(validFor.Seconds()),
			Type:      vehicleETATypeTimestamp,
			Order:     order,
			Units:     vehicleETAUnitSeconds,
		},
		Value: uint(eta.Unix()),
	}
}

func buildVehicleETAExactStruct(direction string, made time.Time, validFor time.Duration, eta time.Duration, precise bool, order uint) vehicleETASingleValue {
	data := vehicleETASingleValue{
		vehicleETA: vehicleETA{
			Direction: direction,
			Made:      made.Unix(),
			ValidFor:  uint(validFor.Seconds()),
			Type:      vehicleETATypeExact,
			Order:     order,
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

func buildVehicleETALessThanStruct(direction string, made time.Time, validFor time.Duration, eta time.Duration, precise bool, order uint) vehicleETASingleValue {
	data := vehicleETASingleValue{
		vehicleETA: vehicleETA{
			Direction: direction,
			Made:      made.Unix(),
			ValidFor:  uint(validFor.Seconds()),
			Type:      vehicleETATypeLessThan,
			Order:     order,
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

func buildVehicleETAMoreThanStruct(direction string, made time.Time, validFor time.Duration, eta time.Duration, precise bool, order uint) vehicleETASingleValue {
	data := vehicleETASingleValue{
		vehicleETA: vehicleETA{
			Direction: direction,
			Made:      made.Unix(),
			ValidFor:  uint(validFor.Seconds()),
			Type:      vehicleETATypeMoreThan,
			Order:     order,
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

func buildVehicleETAIntervalStruct(direction string, made time.Time, validFor time.Duration, lower, upper time.Duration, precise bool, order uint) vehicleETAInterval {
	data := vehicleETAInterval{
		vehicleETA: vehicleETA{
			Direction: direction,
			Made:      made.Unix(),
			ValidFor:  uint(validFor.Seconds()),
			Type:      vehicleETATypeInterval,
			Order:     order,
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

func buildVehicleETAJSONPayload(structs ...interface{}) []byte {
	encoded, err := json.Marshal(structs)
	if err != nil {
		return []byte{}
	}
	return encoded
}

// SendVehicleETAs publishes vehicle ETAs for all stations and directions in the respective topics
func (g *MQTTGateway) SendVehicleETAs() error {
	if g.etaAvailability == "none" || g.etaAvailability == "" {
		return nil
	}
	tx, err := g.Node.Beginx()
	if err != nil {
		return err
	}
	defer tx.Commit() // read-only tx

	stations, err := dataobjects.GetStations(tx)
	if err != nil {
		return err
	}

	for _, station := range stations {
		structs, err := g.buildStructsForStation(tx, station, 1)
		if err != nil {
			return err
		}
		if len(structs) != 0 {
			g.sendStructsAccordingToAvailability(station, structs, false)
		}

		structsAll, err := g.buildStructsForStation(tx, station, 3)
		if err != nil {
			return err
		}

		if len(structsAll) != 0 {
			g.sendStructsAccordingToAvailability(station, structsAll, true)
		}

	}
	return nil
}

func (g *MQTTGateway) sendStructsAccordingToAvailability(station *dataobjects.Station, structs []interface{}, isAll bool) {
	topicSuffix := ""
	if isAll {
		topicSuffix = "/all"
	}
	payload := buildVehicleETAPayload(structs...)
	g.server.Publish(&packets.Publish{
		Qos:       packets.QOS_0,
		TopicName: []byte(fmt.Sprintf("dev-msgpack/vehicleeta/%s/%s%s", station.Network.ID, station.ID, topicSuffix)),
		Payload:   payload,
	})

	if g.etaAvailability == "all" {
		g.server.Publish(&packets.Publish{
			Qos:       packets.QOS_0,
			TopicName: []byte(fmt.Sprintf("msgpack/vehicleeta/%s/%s%s", station.Network.ID, station.ID, topicSuffix)),
			Payload:   payload,
		})

		jsonPayload := buildVehicleETAJSONPayload(structs...)
		g.server.Publish(&packets.Publish{
			Qos:       packets.QOS_0,
			TopicName: []byte(fmt.Sprintf("json/vehicleeta/%s/%s%s", station.Network.ID, station.ID, topicSuffix)),
			Payload:   jsonPayload,
		})
	}
}

func (g *MQTTGateway) buildStructsForStation(tx sqalx.Node, station *dataobjects.Station, numVehicles int) ([]interface{}, error) {
	structs := []interface{}{}

	directions, err := station.Directions(tx, true)
	if err != nil {
		return structs, err
	}

	for _, direction := range directions {
		etas := g.vehicleETAhandler.VehicleETAs(station, direction, numVehicles)
		for _, eta := range etas {
			structs = append(structs, g.vehicleETAtoStruct(eta))
		}
	}
	return structs, nil
}

func (g *MQTTGateway) vehicleETAtoStruct(eta *dataobjects.VehicleETA) interface{} {
	precise := eta.Precision < 30*time.Second
	switch eta.Type {
	case dataobjects.Absolute:
		return buildVehicleETATimestampStruct(eta.Direction.ID, eta.Computed,
			eta.RemainingValidity(), eta.AbsoluteETA, uint(eta.ArrivalOrder))
	case dataobjects.RelativeExact:
		return buildVehicleETAExactStruct(eta.Direction.ID, eta.Computed,
			eta.RemainingValidity(), eta.LiveETA(), precise, uint(eta.ArrivalOrder))
	case dataobjects.RelativeMinimum:
		return buildVehicleETAMoreThanStruct(eta.Direction.ID, eta.Computed,
			eta.RemainingValidity(), eta.LiveETA(), precise, uint(eta.ArrivalOrder))
	case dataobjects.RelativeMaximum:
		return buildVehicleETALessThanStruct(eta.Direction.ID, eta.Computed,
			eta.RemainingValidity(), eta.LiveETA(), precise, uint(eta.ArrivalOrder))
	default:
		return nil
	}
}

// SendVehicleETAForStationToClient publishes, to the given client, vehicle ETAs for the specified station
func (g *MQTTGateway) SendVehicleETAForStationToClient(client *gmqtt.Client, topicID, networkID, stationID string) error {
	tx, err := g.Node.Beginx()
	if err != nil {
		return err
	}
	defer tx.Commit() // read-only tx

	station, err := dataobjects.GetStation(tx, stationID)
	if err != nil {
		return err
	}

	if station.Network.ID != networkID {
		return errors.New("Mismatch between expected network ID and station ID")
	}

	numVehicles := 1
	if strings.HasSuffix(topicID, "/all") {
		numVehicles = 3
	}

	structs, err := g.buildStructsForStation(tx, station, numVehicles)
	if err != nil || len(structs) == 0 {
		return err
	}

	var payload []byte
	if strings.Contains(strings.Split(topicID, "/")[0], "msgpack") {
		payload = buildVehicleETAPayload(structs...)
	} else {
		payload = buildVehicleETAJSONPayload(structs...)
	}

	g.server.Publish(&packets.Publish{
		Qos:       packets.QOS_0,
		TopicName: []byte(topicID),
		Payload:   payload,
	}, client.ClientOptions().ClientID)

	return nil
}
