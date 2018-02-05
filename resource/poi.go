package resource

import (
	"github.com/gbl08ma/disturbancesmlx/dataobjects"
	"github.com/heetch/sqalx"
	"github.com/yarf-framework/yarf"
)

// POI composites resource
type POI struct {
	resource
}

type apiPOI struct {
	ID         string            `msgpack:"id" json:"id"`
	Type       string            `msgpack:"type" json:"type"`
	WorldCoord [2]float64        `msgpack:"worldCoord" json:"worldCoord"`
	URL        string            `msgpack:"webURL" json:"webURL"`
	MainLocale string            `msgpack:"mainLocale" json:"mainLocale"`
	Names      map[string]string `msgpack:"names" json:"names"`
}

func (r *POI) WithNode(node sqalx.Node) *POI {
	r.node = node
	return r
}

func (n *POI) Get(c *yarf.Context) error {
	tx, err := n.Beginx()
	if err != nil {
		return err
	}
	defer tx.Commit() // read-only tx

	if c.Param("id") != "" {
		poi, err := dataobjects.GetPOI(tx, c.Param("id"))
		if err != nil {
			return err
		}
		RenderData(c, apiPOI(*poi))
	} else {
		pois, err := dataobjects.GetPOIs(tx)
		if err != nil {
			return err
		}
		apipois := make([]apiPOI, len(pois))
		for i := range pois {
			apipois[i] = apiPOI(*pois[i])
		}
		RenderData(c, apipois)
	}
	return nil
}
