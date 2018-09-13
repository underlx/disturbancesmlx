package resource

import (
	"net/http"

	"github.com/gbl08ma/sqalx"
	"github.com/underlx/disturbancesmlx/dataobjects"
	"github.com/yarf-framework/yarf"
)

// PairConnectionHandler handles connections of APIPairs with external first-party services and subsystems
type PairConnectionHandler interface {
	TryCreateConnection(node sqalx.Node, code, deviceName string, pair *dataobjects.APIPair) bool
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
			})
			return tx.Commit()
		}
	}

	c.Response.WriteHeader(http.StatusNotFound)
	RenderData(c, apiPairConnectionResponse{
		Result: "failure",
	})

	return nil
}
