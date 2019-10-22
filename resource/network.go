package resource

import (
	"github.com/gbl08ma/sqalx"
	"github.com/underlx/disturbancesmlx/types"
	"github.com/yarf-framework/yarf"
)

// Network composites resource
type Network struct {
	resource
}

type apiNetwork struct {
	ID           string               `msgpack:"id" json:"id"`
	Name         string               `msgpack:"name" json:"name"`
	MainLocale   string               `msgpack:"mainLocale" json:"mainLocale"`
	Names        map[string]string    `msgpack:"names" json:"names"`
	TypicalCars  int                  `msgpack:"typCars" json:"typCars"`
	Holidays     []int64              `msgpack:"holidays" json:"holidays"`
	OpenTime     types.Time     `msgpack:"openTime" json:"openTime"`
	OpenDuration types.Duration `msgpack:"duration" json:"duration"`
	Timezone     string               `msgpack:"timezone" json:"timezone"`
	NewsURL      string               `msgpack:"newsURL" json:"newsURL"`
}

type apiNetworkSchedule struct {
	Network      *types.Network `msgpack:"-" json:"-"`
	Holiday      bool                 `msgpack:"holiday" json:"holiday"`
	Day          int                  `msgpack:"day" json:"day"`
	Open         bool                 `msgpack:"open" json:"open"`
	OpenTime     types.Time     `msgpack:"openTime" json:"openTime"`
	OpenDuration types.Duration `msgpack:"duration" json:"duration"`
}

type apiNetworkWrapper struct {
	apiNetwork `msgpack:",inline"`
	Lines      []string             `msgpack:"lines" json:"lines"`
	Stations   []string             `msgpack:"stations" json:"stations"`
	Schedule   []apiNetworkSchedule `msgpack:"schedule" json:"schedule"`
}

// WithNode associates a sqalx Node with this resource
func (r *Network) WithNode(node sqalx.Node) *Network {
	r.node = node
	return r
}

// Get serves HTTP GET requests on this resource
func (r *Network) Get(c *yarf.Context) error {
	tx, err := r.Beginx()
	if err != nil {
		return err
	}
	defer tx.Commit() // read-only tx

	if c.Param("id") != "" {
		network, err := types.GetNetwork(tx, c.Param("id"))
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

		data.Schedule = []apiNetworkSchedule{}
		schedules, err := network.Schedules(tx)
		if err != nil {
			return err
		}
		for _, s := range schedules {
			data.Schedule = append(data.Schedule, apiNetworkSchedule(*s))
		}
		RenderData(c, data, "s-maxage=10")
	} else {
		networks, err := types.GetNetworks(tx)
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

			apinetworks[i].Schedule = []apiNetworkSchedule{}
			schedules, err := networks[i].Schedules(tx)
			if err != nil {
				return err
			}
			for _, s := range schedules {
				apinetworks[i].Schedule = append(apinetworks[i].Schedule, apiNetworkSchedule(*s))
			}
		}
		RenderData(c, apinetworks, "s-maxage=10")
	}
	return nil
}
