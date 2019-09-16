package mlxscraper

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gbl08ma/sqalx"
	"github.com/underlx/disturbancesmlx/dataobjects"
)

// ETAScraper is a scraper for Metro de Lisboa vehicle ETAs
type ETAScraper struct {
	running  bool
	stopChan chan struct{}

	stations     []*dataobjects.Station
	stationsByID map[string]*dataobjects.Station
	log          *log.Logger
	locs         map[string]*time.Location
	etaValidity  time.Duration

	RequestURL            string
	BearerToken           string
	Network               *dataobjects.Network
	HTTPClient            *http.Client
	WaitPeriodBetweenEach time.Duration
	WaitPeriodBetweenAll  time.Duration
	NewETACallback        func(eta *dataobjects.VehicleETA)
}

// ID returns the ID of this scraper
func (sc *ETAScraper) ID() string {
	return "sc-pt-ml-etas"
}

// Init initializes the scraper
func (sc *ETAScraper) Init(node sqalx.Node, log *log.Logger) error {
	sc.log = log
	sc.locs = make(map[string]*time.Location)

	if sc.HTTPClient == nil {
		sc.HTTPClient = &http.Client{
			Timeout: 10 * time.Second,
		}
	}

	tx, err := node.Beginx()
	if err != nil {
		return err
	}
	defer tx.Commit() // read-only tx

	if sc.Network != nil {
		sc.stations, err = sc.Network.Stations(tx)
	} else {
		sc.stations, err = dataobjects.GetStations(tx)
	}
	if err != nil {
		return err
	}

	sc.stationsByID = make(map[string]*dataobjects.Station)
	for _, s := range sc.stations {
		sc.stationsByID[s.ID] = s
	}

	sc.estimateValidity()

	return nil
}

// Begin starts the scraper
func (sc *ETAScraper) Begin() {
	sc.stopChan = make(chan struct{}, 1)
	sc.running = true
	go sc.mainLoop()
}

// End stops the scraper
func (sc *ETAScraper) End() {
	close(sc.stopChan)
	sc.running = false
}

// Running returns whether the scraper is running
func (sc *ETAScraper) Running() bool {
	return sc.running
}

func (sc *ETAScraper) estimateValidity() {
	// assume each request takes 2 seconds to complete
	// add 30 seconds as error margin (don't want the ETAs to be evicted too soon)
	sc.etaValidity = sc.WaitPeriodBetweenAll +
		(sc.WaitPeriodBetweenEach+2*time.Second)*time.Duration(len(sc.stations)) +
		30*time.Second
}

func (sc *ETAScraper) mainLoop() {
	for {
		startTime := time.Now()
		err := sc.fetchStations()
		if err != nil {
			sc.log.Printf("Error fetching ETAs: %v\n", err)
		}

		select {
		case <-sc.stopChan:
			return
		case <-time.After(sc.WaitPeriodBetweenAll):
		}
		// add 20 seconds as error margin
		// (don't want the ETAs to be evicted too soon)
		sc.etaValidity = time.Since(startTime) + 20*time.Second
	}
}

type responseStruct struct {
	Resposta []directionETAs `json:"resposta"`
	Codigo   string          `json:"codigo"`
}

type directionETAs struct {
	StopID        string `json:"stop_id"`
	Cais          string `json:"cais"`
	Hora          string `json:"hora"`
	Comboio       string `json:"comboio"`
	TempoChegada1 string `json:"tempoChegada1"`
	Comboio2      string `json:"comboio2"`
	TempoChegada2 string `json:"tempoChegada2"`
	Comboio3      string `json:"comboio3"`
	TempoChegada3 string `json:"tempoChegada3"`
	Destino       string `json:"destino"`
	SairServico   string `json:"sairServico"`
}

func (sc *ETAScraper) fetchStations() error {
	req, err := http.NewRequest(http.MethodGet, sc.RequestURL, nil)
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", sc.headerToken())
	requestStart := time.Now()
	response, err := sc.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	requestDuration := time.Since(requestStart)

	if response.ContentLength > 1024*1024 || response.StatusCode != http.StatusOK {
		return fmt.Errorf("non-200 status code (%d) in response, or response body unexpectedly big", response.StatusCode)
	}

	var data responseStruct
	err = json.NewDecoder(response.Body).Decode(&data)
	if err != nil {
		return err
	}

	if len(data.Resposta) == 0 {
		sc.log.Println("Warning: response to ETA request contained no ETAs")
	}

	if sc.NewETACallback != nil {
		return sc.processETAdata(data.Resposta, requestDuration)
	}
	return nil
}

func (sc *ETAScraper) headerToken() string {
	return "Bearer " + sc.BearerToken
}

func (sc *ETAScraper) stopIDtoStationID(stopID string) string {
	switch {
	case sc.Network == nil:
		sc.log.Println("Warning: stopIDtoStationID: don't know how to transform", stopID, "into station ID (network not specified)")
		return ""
	case sc.Network.ID == "pt-ml":
		return fmt.Sprintf("%s-%s", sc.Network.ID, strings.ToLower(stopID))
	default:
		sc.log.Println("Warning: stopIDtoStationID: don't know how to transform", stopID, "into station ID (network not supported)")
		return ""
	}
}

func (sc *ETAScraper) processETAdata(dirETAs []directionETAs, requestTime time.Duration) error {
	for _, dirETA := range dirETAs {
		station, ok := sc.stationsByID[sc.stopIDtoStationID(dirETA.StopID)]
		if !ok {
			sc.log.Printf("Warning: response includes ETAs for unknown stop ID %s. Ignoring those\n", dirETA.StopID)
			continue
		}

		// Using the remote system's time here is a bad idea, assuming it's just the time at which the request's response was created
		// If it turns out that this time is their own analogous to our "Computed" field (i.e. the ETAs may already be a bit old when we receive them),
		// then we'll have to figure out a way to compute the drift between our clock and their clock as much as possible,
		// use this "Hora" field in the response, and adjust it for the drift
		// (so our Computed is as close as possible to the true time of computation of the ETA, by their system, relative to our clock)
		/*creation, err := time.ParseInLocation("20060102150405", dirETA.Hora, sc.getLocation(station.Network.Timezone))
		if err != nil {
			return err
		}*/

		commonETA := dataobjects.VehicleETA{
			Station:   station,
			Direction: sc.getDirection(dirETA),
			//Computed:  creation,
			// this assumes their ETAs were computed while producing the response to our request
			Computed:  time.Now().Add(-requestTime / time.Duration(2)),
			ValidFor:  sc.etaValidity,
			Precision: 1 * time.Second,
			Type:      dataobjects.RelativeExact,
		}

		if commonETA.Direction == nil {
			sc.log.Println("Warning: could not decode direction in ETA data for station", station.ID, "- ignoring ETA. ETA data:", dirETA)
			continue
		}

		// ETA for next train
		if dirETA.Comboio != "" { // TODO identify how lack of next train is identified
			firstETA := commonETA
			firstETA.ArrivalOrder = 1
			firstETA.VehicleServiceID = dirETA.Comboio
			seconds, err := strconv.Atoi(dirETA.TempoChegada1)
			if err != nil {
				return err
			}
			firstETA.SetETA(time.Duration(seconds) * time.Second)

			sc.NewETACallback(&firstETA)
		}

		// ETA for train after next train
		if dirETA.Comboio2 != "" { // TODO identify how lack of next train is identified
			secondETA := commonETA
			secondETA.ArrivalOrder = 2
			secondETA.VehicleServiceID = dirETA.Comboio2
			seconds, err := strconv.Atoi(dirETA.TempoChegada2)
			if err != nil {
				return err
			}
			secondETA.SetETA(time.Duration(seconds) * time.Second)

			sc.NewETACallback(&secondETA)
		}

		// ETA for train after next two trains
		if dirETA.Comboio3 != "" { // TODO identify how lack of next train is identified
			thirdETA := commonETA
			thirdETA.ArrivalOrder = 3
			thirdETA.VehicleServiceID = dirETA.Comboio3
			seconds, err := strconv.Atoi(dirETA.TempoChegada3)
			if err != nil {
				return err
			}
			thirdETA.SetETA(time.Duration(seconds) * time.Second)

			sc.NewETACallback(&thirdETA)
		}

	}

	return nil
}

func (sc *ETAScraper) getDirection(dirETA directionETAs) *dataobjects.Station {
	// TODO figure out how to obtain direction from dirETA.Destino and/or dirETA.Cais
	d, ok := destinoToStationID[dirETA.Destino]
	if !ok {
		return nil
	}
	return sc.stationsByID[d]
}

func (sc *ETAScraper) getLocation(timezone string) *time.Location {
	if loc, ok := sc.locs[timezone]; ok {
		return loc
	}

	loc, err := time.LoadLocation(timezone)
	if err != nil {
		sc.log.Fatalln("Invalid timezone", timezone)
	}
	sc.locs[timezone] = loc
	return loc
}

var destinoToStationID = map[string]string{
	"33": "pt-ml-rb",
	"42": "pt-ml-sp",
	"50": "pt-ml-te",
	"54": "pt-ml-cs",
	"38": "pt-ml-ss",
	"60": "pt-ml-ap",
	"43": "pt-ml-od",
	"48": "pt-ml-ra",
	"45": "pt-ml-cg",

	"56": "pt-ml-bv",
	"57": "pt-ml-ch",
	"59": "pt-ml-mo",
	"34": "pt-ml-as",
	"35": "pt-ml-po",
	"36": "pt-ml-cm",
	"37": "pt-ml-la",
	"39": "pt-ml-av",
	"40": "pt-ml-bc",
	"41": "pt-ml-tp",
	"44": "pt-ml-lu",
	"46": "pt-ml-cp",
	"53": "pt-ml-mm",
	"52": "pt-ml-am",
	"51": "pt-ml-al",
}
