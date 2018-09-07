package resource

import (
	"github.com/gbl08ma/sqalx"
	"github.com/underlx/disturbancesmlx/dataobjects"
	"github.com/underlx/disturbancesmlx/utils"
	"github.com/yarf-framework/yarf"
)

// Station composites resource
type Station struct {
	resource
}

type apiStation struct {
	ID       string               `msgpack:"id" json:"id"`
	Name     string               `msgpack:"name" json:"name"`
	AltNames []string             `msgpack:"altNames" json:"altNames"`
	Tags     []string             `msgpack:"tags" json:"tags"`
	LowTags  []string             `msgpack:"lowTags" json:"lowTags"`
	Network  *dataobjects.Network `msgpack:"-" json:"-"`
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
	POIs           []string                     `msgpack:"pois" json:"pois"`
	TriviaURLs     map[string]string            `msgpack:"triviaURLs" json:"triviaURLs"`
	ConnectionURLs map[string]map[string]string `msgpack:"connURLs" json:"connURLs"`
}

// WithNode associates a sqalx Node with this resource
func (r *Station) WithNode(node sqalx.Node) *Station {
	r.node = node
	return r
}

// Get serves HTTP GET requests on this resource
func (r *Station) Get(c *yarf.Context) error {
	tx, err := r.Beginx()
	if err != nil {
		return err
	}
	defer tx.Commit() // read-only tx

	var stations []*dataobjects.Station

	if c.Param("id") != "" {
		var station *dataobjects.Station
		station, err = dataobjects.GetStation(tx, c.Param("id"))
		stations = []*dataobjects.Station{station}
	} else {
		stations, err = dataobjects.GetStations(tx)
	}

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

		apistations[i].POIs = []string{}
		pois, err := stations[i].POIs(tx)
		if err != nil {
			return err
		}
		for _, poi := range pois {
			apistations[i].POIs = append(apistations[i].POIs, poi.ID)
		}
		apistations[i].TriviaURLs = utils.ComputeStationTriviaURLs(stations[i])
		apistations[i].ConnectionURLs = utils.ComputeStationConnectionURLs(stations[i])

		// compatibility with old clients: set station features
		// TODO remove this once old clients are no longer supported
		apistations[i].Features.Airport = stations[i].HasTag("c_airport")
		apistations[i].Features.Boat = stations[i].HasTag("c_boat")
		apistations[i].Features.Bus = stations[i].HasTag("c_bus")
		apistations[i].Features.Lift = stations[i].HasTag("m_lift_platform") || stations[i].HasTag("m_lift_surface")
		apistations[i].Features.Train = stations[i].HasTag("c_train")
	}

	if c.Param("id") != "" {
		RenderData(c, apistations[0])
	} else {
		RenderData(c, apistations)
	}
	return nil
}
