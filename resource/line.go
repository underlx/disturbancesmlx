package resource

import (
	"github.com/heetch/sqalx"
	"github.com/yarf-framework/yarf"
	"tny.im/disturbancesmlx/interfaces"
)

// Line composites resource
type Line struct {
	resource
}

type apiLine struct {
	ID      string              `msgpack:"id" json:"id"`
	Name    string              `msgpack:"name" json:"name"`
	Color   string              `msgpack:"color" json:"color"`
	Network *interfaces.Network `msgpack:"-" json:"-"`
}

type apiLineWrapper struct {
	apiLine   `msgpack:",inline"`
	NetworkID string   `msgpack:"network" json:"network"`
	Stations  []string `msgpack:"stations" json:"stations"`
}

func (r *Line) WithNode(node sqalx.Node) *Line {
	r.node = node
	return r
}

func (n *Line) Get(c *yarf.Context) error {
	tx, err := n.Beginx()
	if err != nil {
		return err
	}
	defer tx.Commit() // read-only tx

	if c.Param("id") != "" {
		line, err := interfaces.GetLine(tx, c.Param("id"))
		if err != nil {
			return err
		}
		data := apiLineWrapper{
			apiLine:   apiLine(*line),
			NetworkID: line.Network.ID,
		}

		data.Stations = []string{}
		stations, err := line.Stations(tx)
		if err != nil {
			return err
		}
		for _, station := range stations {
			data.Stations = append(data.Stations, station.ID)
		}

		RenderData(c, data)
	} else {
		lines, err := interfaces.GetLines(tx)
		if err != nil {
			return err
		}
		apilines := make([]apiLineWrapper, len(lines))
		for i := range lines {
			apilines[i] = apiLineWrapper{
				apiLine:   apiLine(*lines[i]),
				NetworkID: lines[i].Network.ID,
			}

			apilines[i].Stations = []string{}
			stations, err := lines[i].Stations(tx)
			if err != nil {
				return err
			}
			for _, station := range stations {
				apilines[i].Stations = append(apilines[i].Stations, station.ID)
			}
		}
		RenderData(c, apilines)
	}
	return nil
}
