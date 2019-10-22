package mlxscraper

import (
	"log"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	cache "github.com/patrickmn/go-cache"

	"github.com/mmcdole/gofeed"
	"github.com/underlx/disturbancesmlx/types"
)

// RSSScraper is an announcement scraper for the Metro de Lisboa website
// It reads the RSS feed from the official website
type RSSScraper struct {
	running        bool
	ticker         *time.Ticker
	stopChan       chan struct{}
	log            *log.Logger
	newAnnCallback func(announcement *types.Announcement)
	firstUpdate    bool
	fp             *gofeed.Parser
	announcements  []*types.Announcement
	imageURLcache  *cache.Cache

	URL     string
	Network *types.Network
	Period  time.Duration
}

// ID returns the ID of this scraper
func (sc *RSSScraper) ID() string {
	return "sc-pt-ml-rss"
}

// Init initializes the scraper
func (sc *RSSScraper) Init(log *log.Logger,
	newAnnCallback func(announcement *types.Announcement)) {
	sc.log = log
	sc.newAnnCallback = newAnnCallback
	sc.firstUpdate = true
	sc.fp = gofeed.NewParser()
	sc.imageURLcache = cache.New(1*time.Hour, 30*time.Minute)

	sc.log.Println("RSSScraper initializing")
	sc.update()
	sc.firstUpdate = false
	sc.log.Println("RSSScraper completed first fetch")
}

// Begin starts the scraper
func (sc *RSSScraper) Begin() {
	sc.stopChan = make(chan struct{})
	sc.ticker = time.NewTicker(sc.Period)
	sc.running = true
	go sc.scrape()
}

// End stops the scraper
func (sc *RSSScraper) End() {
	sc.ticker.Stop()
	close(sc.stopChan)
	sc.running = false
}

// Running returns whether the scraper is running
func (sc *RSSScraper) Running() bool {
	return sc.running
}

func (sc *RSSScraper) copyAnnouncements() []*types.Announcement {
	c := make([]*types.Announcement, len(sc.announcements))
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

	announcements := []*types.Announcement{}
	for _, item := range feed.Items {
		ann := types.Announcement{
			Time:     *item.PublishedParsed,
			Network:  sc.Network,
			Title:    item.Title,
			Body:     sc.adaptPostBody(item.Content),
			ImageURL: sc.getImageForPost(item.Link),
			URL:      item.Link,
			Source:   "pt-ml-rss",
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

func (sc *RSSScraper) getImageForPost(postURL string) string {
	url, present := sc.imageURLcache.Get(postURL)
	if present {
		return url.(string)
	}

	netClient := &http.Client{
		Timeout: time.Second * 10,
	}
	response, err := netClient.Get(postURL)
	if err != nil {
		return ""
	}

	doc, err := goquery.NewDocumentFromReader(response.Body)
	if err != nil {
		return ""
	}
	imageURL := ""
	doc.Find("meta").EachWithBreak(func(i int, s *goquery.Selection) bool {
		if property, ok := s.Attr("property"); ok && property == "og:image" {
			if content, ok := s.Attr("content"); ok {
				imageURL = content
				return false
			}
		}
		return true
	})
	sc.imageURLcache.SetDefault(postURL, imageURL)
	return imageURL
}

// Networks returns the networks monitored by this scraper
func (sc *RSSScraper) Networks() []*types.Network {
	return []*types.Network{sc.Network}
}

// Sources returns the sources provided by this scraper
func (sc *RSSScraper) Sources() []string {
	return []string{"pt-ml-rss"}
}

// Announcements returns the announcements read by this scraper
func (sc *RSSScraper) Announcements(source string) []*types.Announcement {
	return sc.copyAnnouncements()
}
