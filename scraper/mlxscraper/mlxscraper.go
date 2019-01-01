package mlxscraper

import (
	"bytes"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"sort"

	"strings"

	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/gbl08ma/sqalx"
	uuid "github.com/satori/go.uuid"
	"github.com/underlx/disturbancesmlx/dataobjects"
	"github.com/underlx/disturbancesmlx/scraper"
)

// Scraper is a scraper for the status of Metro de Lisboa
type Scraper struct {
	running                bool
	ticker                 *time.Ticker
	stopChan               chan struct{}
	lineIDs                []string
	lineNames              []string
	detLineNames           []string
	lines                  map[string]*dataobjects.Line
	previousResponse       []byte
	log                    *log.Logger
	statusCallback         func(status *dataobjects.Status)
	topologyChangeCallback func(scraper.StatusScraper)
	lastUpdate             time.Time

	URL     string
	Network *dataobjects.Network
	Source  *dataobjects.Source
	Period  time.Duration
}

// ID returns the ID of this scraper
func (sc *Scraper) ID() string {
	return "sc-pt-ml-lines"
}

// Init initializes the scraper
func (sc *Scraper) Init(node sqalx.Node, log *log.Logger,
	statusCallback func(status *dataobjects.Status),
	topologyChangeCallback func(scraper.StatusScraper)) {
	sc.log = log
	sc.statusCallback = statusCallback
	sc.topologyChangeCallback = topologyChangeCallback

	sc.lineIDs = []string{"pt-ml-azul", "pt-ml-amarela", "pt-ml-verde", "pt-ml-vermelha"}
	sc.lineNames = []string{"azul", "amarela", "verde", "vermelha"}
	sc.detLineNames = []string{"Azul", "Amar", "Verde", "Verm"}

	sc.lines = make(map[string]*dataobjects.Line)

	tx, err := node.Beginx()
	if err != nil {
		log.Panicln(err)
		return
	}
	defer tx.Commit() // read-only tx

	for _, lineID := range sc.lineIDs {
		sc.lines[lineID], err = dataobjects.GetLine(tx, lineID)
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

func (sc *Scraper) update() {
	response, err := http.Get(sc.URL)
	if err != nil {
		sc.log.Println(err)
		return
	}
	defer response.Body.Close()
	// making sure they don't troll us
	if response.ContentLength < 1024*1024 {
		var buf bytes.Buffer
		tee := io.TeeReader(response.Body, &buf)
		content, err := ioutil.ReadAll(tee)
		if err != nil {
			sc.log.Println(err)
			return
		}
		if !bytes.Equal(content, sc.previousResponse) {
			sc.log.Printf("New status with length %d\n", len(content))

			doc, err := goquery.NewDocumentFromReader(&buf)
			if err != nil {
				sc.log.Println(err)
				return
			}

			for i, lineID := range sc.lineIDs {
				style, _ := doc.Find("#circ_" + sc.lineNames[i]).First().Find("span").Attr("style")
				statusMsg := doc.Find("#det" + sc.detLineNames[i]).Contents().Not("strong").Text()
				isDowntime := !strings.Contains(style, "#33FF00") && !strings.Contains(statusMsg, "Serviço encerrado")
				if strings.HasSuffix(statusMsg, ".") {
					statusMsg = statusMsg[0 : len(statusMsg)-1]
				}

				sc.lastUpdate = time.Now().UTC()

				id, err := uuid.NewV4()
				if err != nil {
					return
				}
				status := &dataobjects.Status{
					ID:         id.String(),
					Time:       time.Now().UTC(),
					Line:       sc.lines[lineID],
					IsDowntime: isDowntime,
					Status:     statusMsg,
					Source:     sc.Source,
				}
				status.ComputeMsgType()
				sc.statusCallback(status)
			}

			sc.previousResponse = content
		}
	}
}

// End stops the scraper
func (sc *Scraper) End() {
	sc.ticker.Stop()
	close(sc.stopChan)
	sc.running = false
}

// Networks returns the networks monitored by this scraper
func (sc *Scraper) Networks() []*dataobjects.Network {
	return []*dataobjects.Network{sc.Network}
}

// Lines returns the lines monitored by this scraper
func (sc *Scraper) Lines() []*dataobjects.Line {
	lines := []*dataobjects.Line{}
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
