package mlxscraper

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"sort"

	"strings"

	"time"

	"github.com/gbl08ma/sqalx"
	uuid "github.com/satori/go.uuid"
	"github.com/underlx/disturbancesmlx/types"
)

// Scraper is a scraper for the status of Metro de Lisboa
type Scraper struct {
	running          bool
	ticker           *time.Ticker
	stopChan         chan struct{}
	lineIDs          []string
	lineNames        []string
	shortLineNames   []string
	lines            map[string]*types.Line
	previousResponse []byte
	log              *log.Logger
	lastUpdate       time.Time

	EndpointURL    string
	BearerToken    string
	StatusCallback func(status *types.Status)
	Network        *types.Network
	Source         *types.Source
	Period         time.Duration
	HTTPClient     *http.Client
}

// ID returns the ID of this scraper
func (sc *Scraper) ID() string {
	return "sc-pt-ml-lines"
}

// Init initializes the scraper
func (sc *Scraper) Init(node sqalx.Node, log *log.Logger) {
	sc.log = log

	sc.lineIDs = []string{"pt-ml-azul", "pt-ml-amarela", "pt-ml-verde", "pt-ml-vermelha"}
	sc.lineNames = []string{"azul", "amarela", "verde", "vermelha"}
	sc.shortLineNames = []string{"az", "am", "vd", "vm"}

	sc.lines = make(map[string]*types.Line)

	if sc.HTTPClient == nil {
		sc.HTTPClient = &http.Client{
			Timeout: 10 * time.Second,
		}
	}

	tx, err := node.Beginx()
	if err != nil {
		log.Panicln(err)
		return
	}
	defer tx.Commit() // read-only tx

	for _, lineID := range sc.lineIDs {
		sc.lines[lineID], err = types.GetLine(tx, lineID)
		if err != nil {
			log.Panicln(err)
			return
		}
	}

	sc.log.Println("Scraper initializing")
	sc.update()
}

// Begin starts the scraper
func (sc *Scraper) Begin() {
	sc.stopChan = make(chan struct{})
	sc.ticker = time.NewTicker(sc.Period)
	sc.running = true
	go sc.scrape()
}

// Running returns whether the scraper is running
func (sc *Scraper) Running() bool {
	return sc.running
}

func (sc *Scraper) scrape() {
	sc.update()
	sc.log.Println("Scraper completed second fetch")
	for {
		select {
		case <-sc.ticker.C:
			sc.update()
			sc.log.Println("Scraper fetch complete")
		case <-sc.stopChan:
			return
		}
	}
}

func (sc *Scraper) headerToken() string {
	return "Bearer " + sc.BearerToken
}

func (sc *Scraper) update() {
	req, err := http.NewRequest(http.MethodGet, sc.EndpointURL+"/estadoLinha/todos", nil)
	if err != nil {
		sc.log.Println(err)
		return
	}

	req.Header.Set("Authorization", sc.headerToken())
	response, err := sc.HTTPClient.Do(req)
	if err != nil {
		sc.log.Println("Error fetching line statuses:", err)
		return
	}
	defer response.Body.Close()

	// making sure they don't troll us
	if response.ContentLength < 1024*1024 && response.StatusCode == http.StatusOK {
		var buf bytes.Buffer
		tee := io.TeeReader(response.Body, &buf)
		content, err := io.ReadAll(tee)
		if err != nil {
			sc.log.Println(err)
			return
		}
		if !bytes.Equal(content, sc.previousResponse) {
			sc.log.Printf("New status with length %d\n", len(content))

			parsed := make(map[string]interface{})
			err := json.Unmarshal(buf.Bytes(), &parsed)
			if err != nil {
				sc.log.Println("Error parsing line status JSON:", err)
				return
			}

			parsed, ok := parsed["resposta"].(map[string]interface{})
			if !ok || parsed == nil {
				sc.log.Println("Field `resposta` not found in response")
				return
			}

			for i, lineID := range sc.lineIDs {
				statusAny, ok := parsed[sc.lineNames[i]]
				if !ok {
					sc.log.Println("Status for line", sc.lineNames[i], "not found in response")
					continue
				}
				statusMsg, ok := statusAny.(string)
				if !ok {
					sc.log.Println("Status for line", sc.lineNames[i], "is not a string")
					continue
				}
				statusMsg = strings.TrimSpace(statusMsg)
				statusMsg = strings.TrimSuffix(statusMsg, ".")

				sc.lastUpdate = time.Now().UTC()

				id, err := uuid.NewV4()
				if err != nil {
					return
				}
				status := &types.Status{
					ID:     id.String(),
					Time:   time.Now().UTC(),
					Line:   sc.lines[lineID],
					Status: statusMsg,
					Source: sc.Source,
				}
				status.IsDowntime = strings.ToLower(status.Status) != "ok"
				if !status.IsDowntime {
					status.Status = "circulação normal" // for consistency with the previous data source
				}
				status.ComputeMsgType()
				sc.StatusCallback(status)
			}

			sc.previousResponse = content
		} else {
			sc.log.Println("Response is the same as the previous one, ignoring")
		}
	} else {
		sc.log.Println("Response has non-200 status or is too large, ignoring")
	}
}

// End stops the scraper
func (sc *Scraper) End() {
	sc.ticker.Stop()
	close(sc.stopChan)
	sc.running = false
}

// Networks returns the networks monitored by this scraper
func (sc *Scraper) Networks() []*types.Network {
	return []*types.Network{sc.Network}
}

// Lines returns the lines monitored by this scraper
func (sc *Scraper) Lines() []*types.Line {
	lines := []*types.Line{}
	for _, v := range sc.lines {
		lines = append(lines, v)
	}
	sort.Slice(lines, func(i, j int) bool {
		return lines[i].Name < lines[j].Name
	})
	return lines
}

// LastUpdate returns the last time this scraper detected a change
func (sc *Scraper) LastUpdate() time.Time {
	return sc.lastUpdate
}
