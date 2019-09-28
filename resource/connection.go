package resource

import (
	"github.com/gbl08ma/sqalx"
	"github.com/ulule/deepcopier"
	"github.com/underlx/disturbancesmlx/dataobjects"
	"github.com/yarf-framework/yarf"
)

// Connection composites resource
type Connection struct {
	resource
}

type apiConnection struct {
	From                  *dataobjects.Station `msgpack:"-" json:"-"`
	To                    *dataobjects.Station `msgpack:"-" json:"-"`
	TypicalWaitingSeconds int                  `msgpack:"typWaitS" json:"typWaitS"`
	TypicalStopSeconds    int                  `msgpack:"typStopS" json:"typStopS"`
	TypicalSeconds        int                  `msgpack:"typS" json:"typS"`
	WorldLength           int                  `msgpack:"worldLength" json:"worldLength"`
}

type apiConnectionWrapper struct {
	apiConnection `msgpack:",inline"`
	FromID        string `msgpack:"from" json:"from"`
	ToID          string `msgpack:"to" json:"to"`
}

// WithNode associates a sqalx Node with this resource
func (r *Connection) WithNode(node sqalx.Node) *Connection {
	r.node = node
	return r
}

// Get serves HTTP GET requests on this resource
func (r *Connection) Get(c *yarf.Context) error {
	tx, err := r.Beginx()
	if err != nil {
		return err
	}
	defer tx.Commit() // read-only tx

	compat := c.Request.URL.Query().Get("closedcompat") != "false"

	if c.Param("from") != "" && c.Param("to") != "" {
		connection, err := dataobjects.GetConnection(tx, c.Param("from"), c.Param("to"), compat)
		if err != nil {
			return err
		}
		ac := &apiConnection{}
		deepcopier.Copy(*connection).To(ac)
		data := apiConnectionWrapper{
			apiConnection: *ac,
			FromID:        connection.From.ID,
			ToID:          connection.To.ID,
		}

		RenderData(c, data, "s-maxage=10")
	} else {
		connections, err := dataobjects.GetConnections(tx, compat)
		if err != nil {
			return err
		}
		apiconnections := make([]apiConnectionWrapper, len(connections))
		for i := range connections {
			ac := &apiConnection{}
			deepcopier.Copy(*connections[i]).To(ac)
			apiconnections[i] = apiConnectionWrapper{
				apiConnection: *ac,
				FromID:        connections[i].From.ID,
				ToID:          connections[i].To.ID,
			}
		}
		RenderData(c, apiconnections, "s-maxage=10")
	}
	return nil
}
