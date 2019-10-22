package resource

import (
	"github.com/gbl08ma/sqalx"
	"github.com/underlx/disturbancesmlx/types"
	"github.com/yarf-framework/yarf"
)

// Transfer composites resource
type Transfer struct {
	resource
}

type apiTransfer struct {
	Station        *types.Station `msgpack:"-" json:"-"`
	From           *types.Line    `msgpack:"-" json:"-"`
	To             *types.Line    `msgpack:"-" json:"-"`
	TypicalSeconds int                  `msgpack:"typS" json:"typS"`
}

type apiTransferWrapper struct {
	apiTransfer `msgpack:",inline"`
	StationID   string `msgpack:"station" json:"station"`
	FromID      string `msgpack:"from" json:"from"`
	ToID        string `msgpack:"to" json:"to"`
}

// WithNode associates a sqalx Node with this resource
func (r *Transfer) WithNode(node sqalx.Node) *Transfer {
	r.node = node
	return r
}

// Get serves HTTP GET requests on this resource
func (r *Transfer) Get(c *yarf.Context) error {
	tx, err := r.Beginx()
	if err != nil {
		return err
	}
	defer tx.Commit() // read-only tx

	if c.Param("station") != "" && c.Param("from") != "" && c.Param("to") != "" {
		transfer, err := types.GetTransfer(tx, c.Param("station"), c.Param("from"), c.Param("to"))
		if err != nil {
			return err
		}
		data := apiTransferWrapper{
			apiTransfer: apiTransfer(*transfer),
			StationID:   transfer.Station.ID,
			FromID:      transfer.From.ID,
			ToID:        transfer.To.ID,
		}

		RenderData(c, data, "s-maxage=10")
	} else {
		transfers, err := types.GetTransfers(tx)
		if err != nil {
			return err
		}
		apitransfers := make([]apiTransferWrapper, len(transfers))
		for i := range transfers {
			apitransfers[i] = apiTransferWrapper{
				apiTransfer: apiTransfer(*transfers[i]),
				StationID:   transfers[i].Station.ID,
				FromID:      transfers[i].From.ID,
				ToID:        transfers[i].To.ID,
			}
		}
		RenderData(c, apitransfers, "s-maxage=10")
	}
	return nil
}
