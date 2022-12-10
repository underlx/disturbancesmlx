package mlxscraper

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"io"
	"io/ioutil"
	"log"
	mathrand "math/rand"
	"net/http"
	"sort"

	"strings"

	"time"

	"github.com/PuerkitoBio/goquery"
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
	detLineNames     []string
	estadoLineNames  []string
	lines            map[string]*types.Line
	previousResponse []byte
	log              *log.Logger
	lastUpdate       time.Time

	lastRandom       string
	randomGeneration time.Time

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
	mathrand.Seed(time.Now().UnixNano())
	sc.log = log

	sc.lineIDs = []string{"pt-ml-azul", "pt-ml-amarela", "pt-ml-verde", "pt-ml-vermelha"}
	sc.lineNames = []string{"azul", "amarela", "verde", "vermelha"}
	sc.estadoLineNames = []string{"Azul", "Amarela", "Verde", "Vermelha"}
	sc.detLineNames = []string{"Azul", "Amar", "Verde", "Verm"}

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

func (sc *Scraper) getURL() string {
	bytes := make([]byte, 5)
	if _, err := rand.Read(bytes); err == nil {
		sc.lastRandom = hex.EncodeToString(bytes)
	}
	return "https://www.metrolisboa.pt/estado_linhas.php?security=" + sc.lastRandom
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

func (sc *Scraper) update() {
	response, err := sc.HTTPClient.Get(sc.getURL())
	if err != nil {
		sc.log.Println(err)
		return
	}
	defer response.Body.Close()
	// making sure they don't troll us
	if response.ContentLength < 1024*1024 && response.StatusCode == http.StatusOK {
		var buf bytes.Buffer
		tee := io.TeeReader(response.Body, &buf)
		content, err := ioutil.ReadAll(tee)
		if err != nil {
			sc.log.Println(err)
			return
		}
		if !bytes.Equal(content, sc.previousResponse) {
			sc.log.Printf("New status with length %d\n", len(content))
			if len(content) < 1000 {
				sc.log.Println("Length too short, probably an error message and not the content we want, ignoring")
				return
			}

			doc, err := goquery.NewDocumentFromReader(&buf)
			if err != nil {
				sc.log.Println(err)
				return
			}

			for i, lineID := range sc.lineIDs {
				style, _ := doc.Find("#circ_" + sc.lineNames[i]).First().Find("span").Attr("style")
				statusMsg := doc.Find("#det" + sc.detLineNames[i]).Contents().Not("strong").Text()
				statusMsg = strings.TrimSpace(statusMsg)
				if strings.HasSuffix(statusMsg, ".") {
					statusMsg = statusMsg[0 : len(statusMsg)-1]
				}

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
				status.ComputeMsgType()
				status.IsDowntime = !strings.Contains(style, "#33FF00") && status.MsgType != types.MLClosedMessage
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
