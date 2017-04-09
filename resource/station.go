package resource

import (
	"github.com/heetch/sqalx"
	"github.com/yarf-framework/yarf"
	"github.com/gbl08ma/disturbancesmlx/interfaces"
)

// Station composites resource
type Station struct {
	resource
}

type apiStation struct {
	ID      string              `msgpack:"id" json:"id"`
	Name    string              `msgpack:"name" json:"name"`
	Network *interfaces.Network `msgpack:"-" json:"-"`
}

type apiStationWrapper struct {
	apiStation `msgpack:",inline"`
	NetworkID  string   `msgpack:"network" json:"network"`
	Lines      []string `msgpack:"lines" json:"lines"`
	WiFiAPs    []string `msgpack:"wiFiAPs" json:"wiFiAPs"`
}

func (r *Station) WithNode(node sqalx.Node) *Station {
	r.node = node
	return r
}

func (n *Station) Get(c *yarf.Context) error {
	tx, err := n.Beginx()
	if err != nil {
		return err
	}
	defer tx.Commit() // read-only tx

	if c.Param("id") != "" {
		station, err := interfaces.GetStation(tx, c.Param("id"))
		if err != nil {
			return err
		}
		data := apiStationWrapper{
			apiStation: apiStation(*station),
			NetworkID:  station.Network.ID,
		}

		data.Lines = []string{}
		lines, err := station.Lines(tx)
		if err != nil {
			return err
		}
		for _, line := range lines {
			data.Lines = append(data.Lines, line.ID)
		}

		data.WiFiAPs = []string{}
		wiFiAPs, err := station.WiFiAPs(tx)
		if err != nil {
			return err
		}
		for _, ap := range wiFiAPs {
			data.WiFiAPs = append(data.WiFiAPs, ap.BSSID)
		}

		RenderData(c, data)
	} else {
		stations, err := interfaces.GetStations(tx)
		if err != nil {
			return err
		}
		apistations := make([]apiStationWrapper, len(stations))
		for i := range stations {
			apistations[i] = apiStationWrapper{
				apiStation: apiStation(*stations[i]),
				NetworkID:  stations[i].Network.ID,
			}

			apistations[i].Lines = []string{}
			lines, err := stations[i].Lines(tx)
			if err != nil {
				return err
			}
			for _, line := range lines {
				apistations[i].Lines = append(apistations[i].Lines, line.ID)
			}

			apistations[i].WiFiAPs = []string{}
			wiFiAPs, err := stations[i].WiFiAPs(tx)
			if err != nil {
				return err
			}
			for _, ap := range wiFiAPs {
				apistations[i].WiFiAPs = append(apistations[i].WiFiAPs, ap.BSSID)
			}
		}
		RenderData(c, apistations)
	}
	return nil
}
