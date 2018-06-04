package resource

import (
	"github.com/heetch/sqalx"
	"github.com/underlx/disturbancesmlx/dataobjects"
	"github.com/yarf-framework/yarf"
)

// RealtimeStatsHandler handles real-time network statistics such as the number of users in transit
type RealtimeStatsHandler interface {
	RegisterActivity(lines []*dataobjects.Line, user *dataobjects.APIPair, justEntered bool)
}

// RealtimeVehicleHandler handles real-time vehicle information such as the position of trains in a network
type RealtimeVehicleHandler interface {
	RegisterTrainPassenger(currentStation *dataobjects.Station, direction *dataobjects.Station)
}

// Realtime composites resource, handles real-time location submissions
type Realtime struct {
	resource
	statsHandler   RealtimeStatsHandler
	vehicleHandler RealtimeVehicleHandler
}

// WithNode associates a sqalx Node with this resource
func (r *Realtime) WithNode(node sqalx.Node) *Realtime {
	r.node = node
	return r
}

// WithHashKey associates a HMAC key with this resource so it can participate in authentication processes
func (r *Realtime) WithHashKey(key []byte) *Realtime {
	r.hashKey = key
	return r
}

// WithStatsHandler associates a RealtimeStatsHandler with this resource
func (r *Realtime) WithStatsHandler(handler RealtimeStatsHandler) *Realtime {
	r.statsHandler = handler
	return r
}

// WithVehicleHandler associates a RealtimeVehicleHandler with this resource
func (r *Realtime) WithVehicleHandler(handler RealtimeVehicleHandler) *Realtime {
	r.vehicleHandler = handler
	return r
}

// msgpack and json field names are very small to optimize bandwidth usage
// (in the subway network, the connection is certainly spotty)
type apiRealtimeLocation struct {
	StationID string `msgpack:"s" json:"s"`
	// DirectionID may be missing/empty if the user just entered the network
	DirectionID string               `msgpack:"d" json:"d"`
	Submitter   *dataobjects.APIPair `msgpack:"-" json:"-"`
}

// Post serves HTTP POST requests on this resource
func (r *Realtime) Post(c *yarf.Context) error {
	tx, err := r.Beginx()
	if err != nil {
		return err
	}
	defer tx.Commit() // read-only tx

	pair, err := r.AuthenticateClient(c)
	if err != nil {
		RenderUnauthorized(c)
		return nil
	}

	var request apiRealtimeLocation
	err = r.DecodeRequest(c, &request)
	if err != nil {
		return err
	}

	request.Submitter = pair

	station, err := dataobjects.GetStation(tx, request.StationID)
	if err != nil {
		return err
	}

	lines, err := station.Lines(tx)
	if err != nil {
		return err
	}

	if r.statsHandler != nil {
		if request.DirectionID == "" {
			r.statsHandler.RegisterActivity(lines, request.Submitter, true)
		} else {
			r.statsHandler.RegisterActivity(lines, request.Submitter, false)
		}
	}

	if r.vehicleHandler != nil {
		if request.DirectionID != "" {
			direction, err := dataobjects.GetStation(tx, request.DirectionID)
			if err != nil {
				return err
			}

			r.vehicleHandler.RegisterTrainPassenger(station, direction)
		}
	}

	return nil
}
