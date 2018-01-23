package resource

import (
	"os"

	"github.com/gbl08ma/disturbancesmlx/dataobjects"
	"github.com/heetch/sqalx"
	"github.com/yarf-framework/yarf"
)

// Station composites resource
type Station struct {
	resource
}

type apiStation struct {
	ID       string                `msgpack:"id" json:"id"`
	Name     string                `msgpack:"name" json:"name"`
	AltNames []string              `msgpack:"altNames" json:"altNames"`
	Features *dataobjects.Features `msgpack:"-" json:"-"`
	Network  *dataobjects.Network  `msgpack:"-" json:"-"`
}

type wifiWrapper struct {
	BSSID string `msgpack:"bssid" json:"bssid"`
	Line  string `msgpack:"line" json:"line"`
}

type apiFeatures struct {
	StationID string `msgpack:"-" json:"-"`
	Lift      bool   `msgpack:"lift" json:"lift"`
	Bus       bool   `msgpack:"bus" json:"bus"`
	Boat      bool   `msgpack:"boat" json:"boat"`
	Train     bool   `msgpack:"train" json:"train"`
	Airport   bool   `msgpack:"airport" json:"airport"`
}

type apiStationWrapper struct {
	apiStation     `msgpack:",inline"`
	NetworkID      string                       `msgpack:"network" json:"network"`
	Lines          []string                     `msgpack:"lines" json:"lines"`
	Features       apiFeatures                  `msgpack:"features" json:"features"`
	Lobbies        []string                     `msgpack:"lobbies" json:"lobbies"`
	WiFiAPs        []wifiWrapper                `msgpack:"wiFiAPs" json:"wiFiAPs"`
	TriviaURLs     map[string]string            `msgpack:"triviaURLs" json:"triviaURLs"`
	ConnectionURLs map[string]map[string]string `msgpack:"connURLs" json:"connURLs"`
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
		station, err := dataobjects.GetStation(tx, c.Param("id"))
		if err != nil {
			return err
		}
		data := apiStationWrapper{
			apiStation: apiStation(*station),
			Features:   apiFeatures(*station.Features),
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

		data.Lobbies = []string{}
		lobbies, err := station.Lobbies(tx)
		if err != nil {
			return err
		}
		for _, lobby := range lobbies {
			data.Lobbies = append(data.Lobbies, lobby.ID)
		}

		data.WiFiAPs = []wifiWrapper{}
		wiFiAPs, err := station.WiFiAPs(tx)
		if err != nil {
			return err
		}
		for _, ap := range wiFiAPs {
			data.WiFiAPs = append(data.WiFiAPs, wifiWrapper{
				BSSID: ap.BSSID,
				Line:  ap.Line,
			})
		}

		data.TriviaURLs = ComputeStationTriviaURLs(station)

		RenderData(c, data)
	} else {
		stations, err := dataobjects.GetStations(tx)
		if err != nil {
			return err
		}
		apistations := make([]apiStationWrapper, len(stations))
		for i := range stations {
			apistations[i] = apiStationWrapper{
				apiStation: apiStation(*stations[i]),
				Features:   apiFeatures(*stations[i].Features),
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

			apistations[i].Lobbies = []string{}
			lobbies, err := stations[i].Lobbies(tx)
			if err != nil {
				return err
			}
			for _, lobby := range lobbies {
				apistations[i].Lobbies = append(apistations[i].Lobbies, lobby.ID)
			}

			apistations[i].WiFiAPs = []wifiWrapper{}
			wiFiAPs, err := stations[i].WiFiAPs(tx)
			if err != nil {
				return err
			}
			for _, ap := range wiFiAPs {
				apistations[i].WiFiAPs = append(apistations[i].WiFiAPs, wifiWrapper{
					BSSID: ap.BSSID,
					Line:  ap.Line,
				})
			}
			apistations[i].TriviaURLs = ComputeStationTriviaURLs(stations[i])
			apistations[i].ConnectionURLs = ComputeStationConnectionURLs(stations[i])
		}
		RenderData(c, apistations)
	}
	return nil
}

func ComputeStationTriviaURLs(station *dataobjects.Station) map[string]string {
	m := make(map[string]string)
	supportedLocales := []string{"pt", "en", "es", "fr"}
	for _, locale := range supportedLocales {
		m[locale] = "stationkb/" + locale + "/trivia/" + station.ID + ".html"
	}
	return m
}

func ComputeStationConnectionURLs(station *dataobjects.Station) map[string]map[string]string {
	m := make(map[string]map[string]string)
	locales := []string{"pt", "en", "es", "fr"}
	connections := []string{"boat", "bus", "train", "park", "bike"}
	for _, locale := range locales {
		for _, connection := range connections {
			path := "stationkb/" + locale + "/connections/" + connection + "/" + station.ID + ".html"
			if info, err := os.Stat(path); err == nil && !info.IsDir() {
				if m[connection] == nil {
					m[connection] = make(map[string]string)
				}
				m[connection][locale] = path
			}
		}
	}
	return m
}
