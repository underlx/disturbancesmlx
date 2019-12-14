package mqttgateway

import (
	"encoding/json"

	"github.com/gbl08ma/gmqtt/pkg/packets"
	"github.com/gbl08ma/sqalx"
	"github.com/underlx/disturbancesmlx/types"
	"gopkg.in/vmihailenco/msgpack.v2"
)

type vehiclePosition struct {
	Vehicle     string `msgpack:"v" json:"vehicle"`
	Cars        uint   `msgpack:"u" json:"cars"`
	PrevStation string `msgpack:"p" json:"prevStation"`
	NextStation string `msgpack:"n" json:"nextStation"`
	Direction   string `msgpack:"d" json:"direction"`
	Platform    string `msgpack:"t" json:"platform"`
	Percent     uint   `msgpack:"c" json:"percent"`
	Made        int64  `msgpack:"m" json:"made"`     // always a unix timestamp (seconds)
	ValidFor    uint   `msgpack:"f" json:"validFor"` // always in seconds
}

func (g *MQTTGateway) buildVehiclePositionStruct(tx sqalx.Node, eta *types.VehicleETA) vehiclePosition {
	prevStation, pct := g.vehicleETAhandler.VehiclePosition(tx, eta)
	return vehiclePosition{
		Vehicle:     eta.VehicleServiceID,
		Cars:        uint(eta.TransportUnits),
		PrevStation: prevStation.ID,
		NextStation: eta.Station.ID,
		Direction:   eta.Direction.ID,
		Platform:    eta.Platform,
		Percent:     pct,
		Made:        eta.Computed.Unix(),
		ValidFor:    uint(eta.RemainingValidity().Seconds()),
	}
}

func buildVehiclePositionPayload(structs ...interface{}) []byte {
	encoded, err := msgpack.Marshal(structs)
	if err != nil {
		return []byte{}
	}
	return encoded
}

func buildVehiclePositionJSONPayload(structs ...interface{}) []byte {
	encoded, err := json.Marshal(structs)
	if err != nil {
		return []byte{}
	}
	return encoded
}

// SendVehiclePositions publishes all vehicle positions
func (g *MQTTGateway) SendVehiclePositions() error {
	if g.etaAvailability == "none" || g.etaAvailability == "" {
		return nil
	}
	tx, err := g.Node.Beginx()
	if err != nil {
		return err
	}
	defer tx.Commit() // read-only tx

	vehicles := g.vehicleETAhandler.TrainPositions()

	structs := []interface{}{}
	for _, eta := range vehicles {
		structs = append(structs, g.buildVehiclePositionStruct(tx, eta))
	}

	if len(structs) != 0 {
		g.sendVehiclePositionStructs(structs)
	}
	return nil
}

func (g *MQTTGateway) sendVehiclePositionStructs(structs []interface{}) {
	payload := buildVehicleETAPayload(structs...)
	g.server.Publish(&packets.Publish{
		Qos:       packets.QOS_0,
		TopicName: []byte("dev-msgpack/vehiclepos"),
		Payload:   payload,
	})

	g.server.Publish(&packets.Publish{
		Qos:       packets.QOS_0,
		TopicName: []byte("msgpack/vehiclepos"),
		Payload:   payload,
	})

	jsonPayload := buildVehicleETAJSONPayload(structs...)
	g.server.Publish(&packets.Publish{
		Qos:       packets.QOS_0,
		TopicName: []byte("json/vehiclepos"),
		Payload:   jsonPayload,
	})
}
