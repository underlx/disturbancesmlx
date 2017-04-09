package resource

import (
	"github.com/heetch/sqalx"
	"github.com/yarf-framework/yarf"
	"github.com/gbl08ma/disturbancesmlx/interfaces"
)

// Network composites resource
type Network struct {
	resource
}

type apiNetwork struct {
	ID   string `msgpack:"id" json:"id"`
	Name string `msgpack:"name" json:"name"`
}

type apiNetworkWrapper struct {
	apiNetwork
	Lines    []string `msgpack:"lines" json:"lines"`
	Stations []string `msgpack:"stations" json:"stations"`
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
		network, err := interfaces.GetNetwork(tx, c.Param("id"))
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
		networks, err := interfaces.GetNetworks(tx)
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
