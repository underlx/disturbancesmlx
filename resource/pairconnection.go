package resource

import (
	"net/http"
	"time"

	"github.com/gbl08ma/sqalx"
	"github.com/underlx/disturbancesmlx/dataobjects"
	"github.com/yarf-framework/yarf"
)

// PairConnectionHandler handles connections of APIPairs with external first-party services and subsystems
type PairConnectionHandler interface {
	ID() string
	TryCreateConnection(node sqalx.Node, code, deviceName string, pair *dataobjects.APIPair) bool
	GetConnectionsForPair(node sqalx.Node, pair *dataobjects.APIPair) ([]dataobjects.PairConnection, error)
	DisplayName() string
}

var pairConnectionHandlers = []PairConnectionHandler{}

// RegisterPairConnectionHandler registers a pair connection handler
func RegisterPairConnectionHandler(handler PairConnectionHandler) {
	pairConnectionHandlers = append(pairConnectionHandlers, handler)
}

// PairConnection composites resource
type PairConnection struct {
	resource
}

type apiPairConnectionRequest struct {
	Code       string `msgpack:"code" json:"code"`
	DeviceName string `msgpack:"deviceName" json:"deviceName"`
}

type apiPairConnectionResponse struct {
	Result      string `msgpack:"result" json:"result"`
	ServiceName string `msgpack:"serviceName" json:"serviceName"`
}

type apiPairConnection struct {
	Service      string      `msgpack:"service" json:"service"`
	ServiceName  string      `msgpack:"serviceName" json:"serviceName"`
	CreationTime time.Time   `msgpack:"creationTime" json:"creationTime"`
	Extra        interface{} `msgpack:"extra" json:"extra"`
}

// WithNode associates a sqalx Node with this resource
func (r *PairConnection) WithNode(node sqalx.Node) *PairConnection {
	r.node = node
	return r
}

// WithHashKey associates a HMAC key with this resource so it can participate in authentication processes
func (r *PairConnection) WithHashKey(key []byte) *PairConnection {
	r.hashKey = key
	return r
}

// Get serves HTTP GET requests on this resource
func (r *PairConnection) Get(c *yarf.Context) error {
	pair, err := r.AuthenticateClient(c)
	if err != nil {
		RenderUnauthorized(c)
		return nil
	}

	tx, err := r.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback() // read-only tx, but this ensures that pairConnectionHandlers can't write even if they wanted

	connections := []dataobjects.PairConnection{}
	services := []PairConnectionHandler{}
	for _, handler := range pairConnectionHandlers {
		c, err := handler.GetConnectionsForPair(tx, pair)
		if err != nil {
			return err
		}
		connections = append(connections, c...)
		for range c {
			services = append(services, handler)
		}
	}

	apiConnections := []apiPairConnection{}
	for i, connection := range connections {
		apiConnections = append(apiConnections, apiPairConnection{
			Service:      services[i].ID(),
			ServiceName:  services[i].DisplayName(),
			CreationTime: connection.Created(),
			Extra:        connection.Extra(),
		})
	}

	RenderData(c, apiConnections, "no-cache, no-store, must-revalidate")

	return nil
}

// Post serves HTTP POST requests on this resource
func (r *PairConnection) Post(c *yarf.Context) error {
	pair, err := r.AuthenticateClient(c)
	if err != nil {
		RenderUnauthorized(c)
		return nil
	}

	tx, err := r.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var connectionRequest apiPairConnectionRequest
	err = r.DecodeRequest(c, &connectionRequest)
	if err != nil {
		return err
	}

	for _, handler := range pairConnectionHandlers {
		if handler.TryCreateConnection(tx, connectionRequest.Code, connectionRequest.DeviceName, pair) {
			RenderData(c, apiPairConnectionResponse{
				Result:      "connected",
				ServiceName: handler.DisplayName(),
			}, "no-cache, no-store, must-revalidate")
			return tx.Commit()
		}
	}

	c.Response.WriteHeader(http.StatusNotFound)
	RenderData(c, apiPairConnectionResponse{
		Result: "failure",
	}, "no-cache, no-store, must-revalidate")

	return nil
}
