package mlxscraper

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"sort"

	"strings"

	"time"

	"github.com/PuerkitoBio/goquery"
	uuid "github.com/satori/go.uuid"
	"tny.im/disturbancesmlx/interfaces"
	"tny.im/disturbancesmlx/scraper"
)

// Scraper is a scraper for the status of Metro de Lisboa
type Scraper struct {
	ticker                 *time.Ticker
	stopChan               chan struct{}
	lines                  map[string]*interfaces.Line
	previousResponse       []byte
	log                    *log.Logger
	statusCallback         func(status *interfaces.Status)
	topologyChangeCallback func(scraper.Scraper)
	firstUpdate            bool

	URL         string
	NetworkID   string
	NetworkName string
	Source      *interfaces.Source
	Period      time.Duration
}

// Begin starts the scraper
func (sc *Scraper) Begin(log *log.Logger,
	statusCallback func(status *interfaces.Status),
	topologyChangeCallback func(scraper.Scraper)) {
	sc.stopChan = make(chan struct{})
	sc.ticker = time.NewTicker(sc.Period)
	sc.log = log
	sc.statusCallback = statusCallback
	sc.topologyChangeCallback = topologyChangeCallback
	sc.lines = make(map[string]*interfaces.Line)
	sc.firstUpdate = true

	sc.log.Println("Scraper starting")
	sc.update()
	sc.firstUpdate = false
	sc.log.Println("Scraper completed first fetch")
	topologyChangeCallback(sc)
	go sc.scrape()
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

			if !sc.firstUpdate {
				// if previousResponse is updated on the first update,
				// status won't be collected on the second update
				sc.previousResponse = content
			}

			newLines := make(map[string]*interfaces.Line)

			doc.Find("table").First().Find("tr").Each(func(i int, s *goquery.Selection) {
				line := s.Find("td").First()
				words := strings.Split(line.Find("b").Text(), " ")
				if len(words) < 2 {
					sc.log.Println("Could not parse line name")
					return
				}
				lineName := words[1]
				lineID := fmt.Sprintf("%s-%s", sc.NetworkID, strings.ToLower(lineName))
				newLines[lineID] = &interfaces.Line{
					Name:    lineName,
					ID:      lineID,
					Network: sc.Networks()[0],
				}

				if !sc.firstUpdate {
					status := line.Next()
					if len(status.Find(".semperturbacao").Nodes) == 0 {
						status := &interfaces.Status{
							ID:         uuid.NewV4().String(),
							Time:       time.Now().UTC(),
							Line:       newLines[lineID],
							IsDowntime: true,
							Status:     status.Find("li").Text(),
							Source:     sc.Source,
						}
						sc.statusCallback(status)
					} else {
						status := &interfaces.Status{
							ID:         uuid.NewV4().String(),
							Time:       time.Now().UTC(),
							Line:       newLines[lineID],
							IsDowntime: false,
							Status:     status.Find("li").Text(),
							Source:     sc.Source,
						}
						sc.statusCallback(status)
					}
				}
			})
			sc.lines = newLines
		}
	}

}

// End stops the scraper
func (sc *Scraper) End() {
	sc.ticker.Stop()
	close(sc.stopChan)
}

// Networks returns the networks monitored by this scraper
func (sc *Scraper) Networks() []*interfaces.Network {
	return []*interfaces.Network{{
		ID:   sc.NetworkID,
		Name: sc.NetworkName,
	}}
}

// Lines returns the lines monitored by this scraper
func (sc *Scraper) Lines() []*interfaces.Line {
	lines := []*interfaces.Line{}
	for _, v := range sc.lines {
		lines = append(lines, v)
	}
	sort.Slice(lines, func(i, j int) bool {
		return lines[i].Name < lines[j].Name
	})
	return lines
}
