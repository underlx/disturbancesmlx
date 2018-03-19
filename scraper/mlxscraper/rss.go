package mlxscraper

import (
	"log"
	"sort"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"

	"github.com/mmcdole/gofeed"
	"github.com/underlx/disturbancesmlx/dataobjects"
)

// RSSScraper is an announcement scraper for the Metro de Lisboa website
// It reads the RSS feed from the official website
type RSSScraper struct {
	ticker         *time.Ticker
	stopChan       chan struct{}
	log            *log.Logger
	newAnnCallback func(announcement *dataobjects.Announcement)
	firstUpdate    bool
	fp             *gofeed.Parser
	announcements  []*dataobjects.Announcement

	URL     string
	Network *dataobjects.Network
	Period  time.Duration
}

// Begin starts the scraper
func (sc *RSSScraper) Begin(log *log.Logger,
	newAnnCallback func(announcement *dataobjects.Announcement)) {
	sc.stopChan = make(chan struct{})
	sc.ticker = time.NewTicker(sc.Period)
	sc.log = log
	sc.newAnnCallback = newAnnCallback
	sc.firstUpdate = true
	sc.fp = gofeed.NewParser()

	sc.log.Println("RSSScraper starting")
	sc.update()
	sc.firstUpdate = false
	sc.log.Println("RSSScraper completed first fetch")
	go sc.scrape()
}

// End stops the scraper
func (sc *RSSScraper) End() {
	sc.ticker.Stop()
	close(sc.stopChan)
}

func (sc *RSSScraper) copyAnnouncements() []*dataobjects.Announcement {
	c := make([]*dataobjects.Announcement, len(sc.announcements))
	for i, annPointer := range sc.announcements {
		ann := *annPointer
		c[i] = &ann
	}
	return c
}

func (sc *RSSScraper) scrape() {
	sc.update()
	sc.log.Println("RSSScraper completed second fetch")
	for {
		select {
		case <-sc.ticker.C:
			sc.update()
			sc.log.Println("RSSScraper fetch complete")
		case <-sc.stopChan:
			return
		}
	}
}

func (sc *RSSScraper) update() {
	feed, err := sc.fp.ParseURL(sc.URL)
	if err != nil {
		sc.log.Println(err)
		return
	}

	announcements := []*dataobjects.Announcement{}
	for _, item := range feed.Items {
		ann := dataobjects.Announcement{
			Time:    *item.PublishedParsed,
			Network: sc.Network,
			Title:   item.Title,
			Body:    sc.adaptPostBody(item.Content),
			URL:     item.Link,
			Source:  "pt-ml-rss",
		}
		announcements = append(announcements, &ann)
	}

	sort.SliceStable(announcements, func(i, j int) bool {
		return announcements[i].Time.Before(announcements[j].Time)
	})

	if !sc.firstUpdate && len(announcements) > 0 {
		isNew := false
		curLast := announcements[len(announcements)-1]
		if len(sc.announcements) == 0 {
			isNew = true
		} else {
			// decide if an announcement is new by looking only into the published date
			prevLast := sc.announcements[len(sc.announcements)-1]
			if curLast.Time.After(prevLast.Time) {
				isNew = true
			}
		}
		if isNew {
			sc.newAnnCallback(curLast)
		}
	}
	sc.announcements = announcements
}

func (sc *RSSScraper) adaptPostBody(original string) string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(original))
	if err != nil {
		sc.log.Println(err)
		return ""
	}
	return doc.Find("p").First().Text()
}

// Networks returns the networks monitored by this scraper
func (sc *RSSScraper) Networks() []*dataobjects.Network {
	return []*dataobjects.Network{sc.Network}
}

// Sources returns the sources provided by this scraper
func (sc *RSSScraper) Sources() []string {
	return []string{"pt-ml-rss"}
}

// Announcements returns the announcements read by this scraper
func (sc *RSSScraper) Announcements(source string) []*dataobjects.Announcement {
	return sc.copyAnnouncements()
}
