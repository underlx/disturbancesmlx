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
	"regexp"
	"sort"
	"strconv"

	"strings"

	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/gbl08ma/sqalx"
	uuid "github.com/satori/go.uuid"
	"github.com/underlx/disturbancesmlx/dataobjects"
)

// Scraper is a scraper for the status of Metro de Lisboa
type Scraper struct {
	running          bool
	ticker           *time.Ticker
	stopChan         chan struct{}
	lineIDs          []string
	lineNames        []string
	detLineNames     []string
	freqLineNames    []string
	estadoLineNames  []string
	freqRegexp       *regexp.Regexp
	numCarsRegexp    *regexp.Regexp
	lines            map[string]*dataobjects.Line
	previousResponse []byte
	log              *log.Logger
	lastUpdate       time.Time

	lastRandom       string
	randomGeneration time.Time

	StatusCallback    func(status *dataobjects.Status)
	ConditionCallback func(condition *dataobjects.LineCondition)
	Network           *dataobjects.Network
	Source            *dataobjects.Source
	Period            time.Duration
	HTTPClient        *http.Client
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
	sc.freqLineNames = []string{"", "Amarela", "Verde", "Vermelha"}
	sc.freqRegexp = regexp.MustCompile("[0-9]{2}:[0-9]{2}")
	sc.numCarsRegexp = regexp.MustCompile("[0-9]+")

	sc.lines = make(map[string]*dataobjects.Line)

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

func (sc *Scraper) getURL() string {
	if time.Since(sc.randomGeneration) > 0 {
		bytes := make([]byte, 5)
		if _, err := rand.Read(bytes); err == nil {
			sc.lastRandom = hex.EncodeToString(bytes)
			sc.randomGeneration = time.Now().Add(time.Duration(4*60+mathrand.Intn(8*60)) * time.Minute)
		}
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
				isDowntime := !strings.Contains(style, "#33FF00") && !strings.Contains(statusMsg, "Serviço encerrado")
				statusMsg = strings.TrimSpace(statusMsg)
				if strings.HasSuffix(statusMsg, ".") {
					statusMsg = statusMsg[0 : len(statusMsg)-1]
				}

				freqMsg := doc.Find("#freqLinha" + sc.freqLineNames[i]).First().Not("strong").Not("span").Text()
				freqMsg = sc.freqRegexp.FindString(freqMsg)
				var freq time.Duration
				if freqMsg != "" {
					for i, part := range strings.Split(freqMsg, ":") {
						num, err := strconv.Atoi(part)
						if err != nil {
							break
						}
						switch i {
						case 0:
							freq += time.Duration(num) * time.Minute
						case 1:
							freq += time.Duration(num) * time.Second
						}
					}
				}

				numCarsMsg := doc.Find("#estado" + sc.estadoLineNames[i]).First().Find("#unidadesTraccao").Not("strong").Text()
				numCarsMsg = sc.numCarsRegexp.FindString(numCarsMsg)
				var numCars int
				if numCarsMsg != "" {
					numCars, _ = strconv.Atoi(numCarsMsg)
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
				sc.StatusCallback(status)

				id, err = uuid.NewV4()
				if err != nil {
					return
				}
				condition := &dataobjects.LineCondition{
					ID:             id.String(),
					Time:           time.Now().UTC(),
					Line:           sc.lines[lineID],
					TrainCars:      numCars,
					TrainFrequency: dataobjects.Duration(freq),
					Source:         sc.Source,
				}
				sc.ConditionCallback(condition)
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
