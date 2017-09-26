package mlxscraper

import (
	"fmt"
	"log"
	"time"

	"github.com/gbl08ma/disturbancesmlx/dataobjects"
	"github.com/mmcdole/gofeed"
)

// RSSScraper is an announcement scraper for the Metro de Lisboa website
// It reads the RSS feed from the official website
type RSSScraper struct {
	ticker              *time.Ticker
	stopChan            chan struct{}
	log                 *log.Logger
	newAnnCallback      func(announcement *dataobjects.Announcement)
	initialDataCallback func(anns []*dataobjects.Announcement)
	firstUpdate         bool
	fp                  *gofeed.Parser
	announcements       []*dataobjects.Announcement

	URL     string
	Network *dataobjects.Network
	Period  time.Duration
}

// Begin starts the scraper
func (sc *RSSScraper) Begin(log *log.Logger,
	newAnnCallback func(announcement *dataobjects.Announcement),
	initialDataCallback func(anns []*dataobjects.Announcement)) {
	sc.stopChan = make(chan struct{})
	sc.ticker = time.NewTicker(sc.Period)
	sc.log = log
	sc.newAnnCallback = newAnnCallback
	sc.initialDataCallback = initialDataCallback
	sc.firstUpdate = true
	sc.fp = gofeed.NewParser()

	sc.log.Println("RSSScraper starting")
	sc.update()
	sc.firstUpdate = false
	sc.log.Println("RSSScraper completed first fetch")
	initialDataCallback(sc.copyAnnouncements())
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
	feed, _ := sc.fp.ParseURL(sc.URL)

	if sc.announcements == nil {
		sc.announcements = []*dataobjects.Announcement{}
		for _, item := range feed.Items {
			fmt.Println(item.Title)
			fmt.Println(item.Description)
			fmt.Println(item.PublishedParsed)
			ann := dataobjects.Announcement{
				Time:    *item.PublishedParsed,
				Network: sc.Network,
				Title:   item.Title,
				Body:    item.Description,
				URL:     item.Link,
				Source:  "pt-ml-rss",
			}
			sc.announcements = append(sc.announcements, &ann)
		}
	}
}

// Networks returns the networks monitored by this scraper
func (sc *RSSScraper) Networks() []*dataobjects.Network {
	return []*dataobjects.Network{sc.Network}
}
