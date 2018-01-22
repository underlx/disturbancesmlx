package resource

import (
	"time"

	"github.com/gbl08ma/disturbancesmlx/dataobjects"
	"github.com/heetch/sqalx"
	"github.com/yarf-framework/yarf"
)

type RealtimeStatsHandler interface {
	RegisterActivity(network *dataobjects.Network, user *dataobjects.APIPair, expectedDuration time.Duration)
}

// Realtime composites resource, handles real-time location submissions
type Realtime struct {
	resource
	statsHandler RealtimeStatsHandler
}

func (r *Realtime) WithNode(node sqalx.Node) *Realtime {
	r.node = node
	return r
}

func (r *Realtime) WithHashKey(key []byte) *Realtime {
	r.hashKey = key
	return r
}

func (r *Realtime) WithStatsHandler(handler RealtimeStatsHandler) *Realtime {
	r.statsHandler = handler
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

	if request.DirectionID == "" {
		// user just entered the network, is going to wait for a vehicle
		r.statsHandler.RegisterActivity(station.Network, request.Submitter, 8*time.Minute)
	} else if lines, err := station.Lines(tx); err != nil && len(lines) > 1 {
		// user might change lines and will need to wait for a vehicle
		r.statsHandler.RegisterActivity(station.Network, request.Submitter, 8*time.Minute)
	} else {
		r.statsHandler.RegisterActivity(station.Network, request.Submitter, 4*time.Minute)
	}

	return nil
}
