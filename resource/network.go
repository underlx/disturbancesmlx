package resource

import (
	"github.com/gbl08ma/disturbancesmlx/dataobjects"
	"github.com/heetch/sqalx"
	"github.com/yarf-framework/yarf"
)

// Network composites resource
type Network struct {
	resource
}

type apiNetwork struct {
	ID           string               `msgpack:"id" json:"id"`
	Name         string               `msgpack:"name" json:"name"`
	TypicalCars  int                  `msgpack:"typCars" json:"typCars"`
	Holidays     []int64              `msgpack:"holidays" json:"holidays"`
	OpenTime     dataobjects.Time     `msgpack:"openTime" json:"openTime"`
	OpenDuration dataobjects.Duration `msgpack:"duration" json:"duration"`
	Timezone     string               `msgpack:"timezone" json:"timezone"`
	NewsURL      string               `msgpack:"newsURL" json:"newsURL"`
}

type apiNetworkWrapper struct {
	apiNetwork `msgpack:",inline"`
	Lines      []string `msgpack:"lines" json:"lines"`
	Stations   []string `msgpack:"stations" json:"stations"`
}

func (r *Network) WithNode(node sqalx.Node) *Network {
	r.node = node
	return r
}

func (n *Network) Get(c *yarf.Context) error {
	tx, err := n.Beginx()
	if err != nil {
		return err
	}
	defer tx.Commit() // read-only tx

	if c.Param("id") != "" {
		network, err := dataobjects.GetNetwork(tx, c.Param("id"))
		if err != nil {
			return err
		}
		data := apiNetworkWrapper{
			apiNetwork: apiNetwork(*network),
		}
		data.Lines = []string{}
		lines, err := network.Lines(tx)
		if err != nil {
			return err
		}
		for _, line := range lines {
			data.Lines = append(data.Lines, line.ID)
		}

		data.Stations = []string{}
		stations, err := network.Stations(tx)
		if err != nil {
			return err
		}
		for _, station := range stations {
			data.Stations = append(data.Stations, station.ID)
		}
		RenderData(c, data)
	} else {
		networks, err := dataobjects.GetNetworks(tx)
		if err != nil {
			return err
		}
		apinetworks := make([]apiNetworkWrapper, len(networks))
		for i := range networks {
			apinetworks[i] = apiNetworkWrapper{
				apiNetwork: apiNetwork(*networks[i]),
			}
			apinetworks[i].Lines = []string{}
			lines, err := networks[i].Lines(tx)
			if err != nil {
				return err
			}
			for _, line := range lines {
				apinetworks[i].Lines = append(apinetworks[i].Lines, line.ID)
			}

			apinetworks[i].Stations = []string{}
			stations, err := networks[i].Stations(tx)
			if err != nil {
				return err
			}
			for _, station := range stations {
				apinetworks[i].Stations = append(apinetworks[i].Stations, station.ID)
			}
		}
		RenderData(c, apinetworks)
	}
	return nil
}
