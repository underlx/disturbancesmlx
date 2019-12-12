package mlxscraper

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"time"

	"github.com/gbl08ma/sqalx"
	uuid "github.com/satori/go.uuid"
	"github.com/thoas/go-funk"
	"github.com/underlx/disturbancesmlx/types"
)

// ConditionsScraper is a scraper for Metro de Lisboa line conditions
type ConditionsScraper struct {
	running  bool
	ticker   *time.Ticker
	stopChan chan struct{}
	log      *log.Logger

	lines []*types.Line

	EndpointURL       string
	BearerToken       string
	Network           *types.Network
	Source            *types.Source
	HTTPClient        *http.Client
	Period            time.Duration
	ConditionCallback func(condition *types.LineCondition)
}

// ID returns the ID of this scraper
func (sc *ConditionsScraper) ID() string {
	return "sc-pt-ml-freqs"
}

// Init initializes the scraper
func (sc *ConditionsScraper) Init(node sqalx.Node, log *log.Logger) error {
	sc.log = log

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
		sc.lines, err = sc.Network.Lines(tx)
	} else {
		sc.lines, err = types.GetLines(tx)
	}
	if err != nil {
		return err
	}
	return nil
}

// Begin starts the scraper
func (sc *ConditionsScraper) Begin() {
	sc.stopChan = make(chan struct{}, 1)
	sc.ticker = time.NewTicker(sc.Period)
	sc.running = true
	go sc.fetchConditions()
	go sc.mainLoop()
}

// End stops the scraper
func (sc *ConditionsScraper) End() {
	sc.ticker.Stop()
	close(sc.stopChan)
	sc.running = false
}

// Running returns whether the scraper is running
func (sc *ConditionsScraper) Running() bool {
	return sc.running
}

func (sc *ConditionsScraper) mainLoop() {
	for {
		select {
		case <-sc.stopChan:
			return
		case <-sc.ticker.C:
			err := sc.fetchConditions()
			if err != nil {
				sc.log.Println(err)
			}
		}
	}
}

type responseStructFreq struct {
	Resposta lineCondition `json:"resposta"`
	Codigo   string        `json:"codigo"`
}

type lineCondition struct {
	Linha      string `json:"Linha"`
	HoraInicio string `json:"HoraInicio"`
	HoraFim    string `json:"HoraFim"`
	Intervalo  string `json:"Intervalo"`
	UT         int    `json:"UT"`
}

func (sc *ConditionsScraper) fetchConditions() error {
	for _, line := range sc.lines {
		err := sc.fetchConditionsForLine(line)
		if err != nil {
			return err
		}
	}
	return nil
}

func (sc *ConditionsScraper) fetchConditionsForLine(line *types.Line) error {
	t := time.Now().Format("150405")
	dayType := "S"
	switch time.Now().Weekday() {
	case time.Saturday:
		dayType = "F"
	case time.Sunday:
		dayType = "F"
	}
	if funk.InInt64s(sc.Network.Holidays, int64(time.Now().YearDay())) {
		dayType = "F"
	}
	url := fmt.Sprintf("%s/infoIntervalos/%s/%s/%s", sc.EndpointURL, line.Name, dayType, t)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", sc.headerToken())
	response, err := sc.HTTPClient.Do(req)
	if err != nil {
		return err
	}

	if response.ContentLength > 1024*1024 || response.StatusCode != http.StatusOK {
		return fmt.Errorf("non-200 status code (%d) in response, or response body unexpectedly big", response.StatusCode)
	}

	responseBytes, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return err
	}
	response.Body.Close()

	var data responseStructFreq
	err = json.Unmarshal(responseBytes, &data)
	if err != nil {
		var altData responseStructAlternative
		err := json.Unmarshal(responseBytes, &altData)
		if err != nil {
			return nil
		}
	}

	freqStr := data.Resposta.Intervalo
	if len(freqStr) != 8 {
		return errors.New("Invalid Intervalo string")
	}
	freqStr = freqStr[0:2] + "m" + freqStr[3:5] + "s"

	freq, err := time.ParseDuration(freqStr)
	if err != nil {
		return err
	}

	id, err := uuid.NewV4()
	if err != nil {
		return err
	}
	condition := &types.LineCondition{
		ID:             id.String(),
		Time:           time.Now().UTC(),
		Line:           line,
		TrainCars:      data.Resposta.UT * 3,
		TrainFrequency: types.Duration(freq),
		Source:         sc.Source,
	}
	sc.ConditionCallback(condition)

	return nil
}

func (sc *ConditionsScraper) headerToken() string {
	return "Bearer " + sc.BearerToken
}
