package mlxscraper

import (
	"bytes"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"sort"

	"strings"

	"github.com/PuerkitoBio/goquery"
	"tny.im/disturbancesmlx/interfaces"
)
import "time"

const statusURL = "http://app.metrolisboa.pt/status/estado_Linhas.php"

// Scraper is a scraper for the status of Metro de Lisboa
type Scraper struct {
	ticker           *time.Ticker
	stopChan         chan struct{}
	lines            map[string]interfaces.Line
	previousResponse []byte
	log              *log.Logger
	statusReporter   func(status interfaces.Status)
}

// Begin starts the scraper
func (sc *Scraper) Begin(log *log.Logger, period time.Duration, statusReporter func(status interfaces.Status)) {
	sc.stopChan = make(chan struct{})
	sc.ticker = time.NewTicker(period)
	sc.log = log
	sc.statusReporter = statusReporter
	sc.lines = make(map[string]interfaces.Line)

	sc.log.Println("Scraper starting")
	sc.update()
	sc.log.Println("Scraper completed first fetch")
	go sc.scrape()
}

func (sc *Scraper) scrape() {
	for {
		select {
		case <-sc.ticker.C:
			sc.update()
		case <-sc.stopChan:
			return
		}
	}
}

func (sc *Scraper) update() {
	response, err := http.Get(statusURL)
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
			sc.previousResponse = content

			newLines := make(map[string]interfaces.Line)

			doc.Find("table").First().Find("tr").Each(func(i int, s *goquery.Selection) {
				line := s.Find("td").First()
				words := strings.Split(line.Find("b").Text(), " ")
				if len(words) < 2 {
					sc.log.Println("Could not parse line name")
					return
				}
				lineName := words[1]
				lineID := strings.ToLower(lineName)
				newLines[lineID] = interfaces.Line{
					Name:    lineName,
					ID:      lineID,
					Network: sc.Networks()[0],
				}

				status := line.Next()
				if len(status.Find(".semperturbacao").Nodes) == 0 {
					status := interfaces.Status{
						Time:       time.Now().UTC(),
						Line:       newLines[lineID],
						IsDowntime: true,
						Status:     status.Find("li").Text(),
						Source: interfaces.Source{
							ID:          "Scraper",
							Name:        "Metro de Lisboa estado_Linhas.php",
							IsAutomatic: true,
						},
					}
					sc.statusReporter(status)
				} else {
					status := interfaces.Status{
						Time:       time.Now().UTC(),
						Line:       newLines[lineID],
						IsDowntime: false,
						Status:     status.Find("li").Text(),
						Source: interfaces.Source{
							ID:          "Scraper",
							Name:        "Metro de Lisboa estado_Linhas.php",
							IsAutomatic: true,
						},
					}
					sc.statusReporter(status)
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
func (sc *Scraper) Networks() []interfaces.Network {
	return []interfaces.Network{{
		ID:   "pt-ml",
		Name: "Metro de Lisboa",
	}}
}

// Lines returns the lines monitored by this scraper
func (sc *Scraper) Lines() []interfaces.Line {
	lines := []interfaces.Line{}
	for _, v := range sc.lines {
		lines = append(lines, v)
	}
	sort.Slice(lines, func(i, j int) bool {
		return lines[i].Name < lines[j].Name
	})
	return lines
}
