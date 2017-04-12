package resource

import (
	"github.com/gbl08ma/disturbancesmlx/interfaces"
	"github.com/heetch/sqalx"
	"github.com/yarf-framework/yarf"
)

// Transfer composites resource
type Transfer struct {
	resource
}

type apiTransfer struct {
	Station        *interfaces.Station `msgpack:"-" json:"-"`
	From           *interfaces.Line    `msgpack:"-" json:"-"`
	To             *interfaces.Line    `msgpack:"-" json:"-"`
	TypicalSeconds int                 `msgpack:"typS" json:"typS"`
}

type apiTransferWrapper struct {
	apiTransfer `msgpack:",inline"`
	StationID   string `msgpack:"station" json:"station"`
	FromID      string `msgpack:"from" json:"from"`
	ToID        string `msgpack:"to" json:"to"`
}

func (r *Transfer) WithNode(node sqalx.Node) *Transfer {
	r.node = node
	return r
}

func (n *Transfer) Get(c *yarf.Context) error {
	tx, err := n.Beginx()
	if err != nil {
		return err
	}
	defer tx.Commit() // read-only tx

	if c.Param("station") != "" && c.Param("from") != "" && c.Param("to") != "" {
		transfer, err := interfaces.GetTransfer(tx, c.Param("station"), c.Param("from"), c.Param("to"))
		if err != nil {
			return err
		}
		data := apiTransferWrapper{
			apiTransfer: apiTransfer(*transfer),
			StationID:   transfer.Station.ID,
			FromID:      transfer.From.ID,
			ToID:        transfer.To.ID,
		}

		RenderData(c, data)
	} else {
		transfers, err := interfaces.GetTransfers(tx)
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
		RenderData(c, apitransfers)
	}
	return nil
}
