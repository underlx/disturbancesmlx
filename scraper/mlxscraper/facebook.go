package mlxscraper

import (
	"encoding/json"
	"log"
	"net/http"
	"sort"
	"time"

	"github.com/PuerkitoBio/goquery"
	cache "github.com/patrickmn/go-cache"
	"github.com/underlx/disturbancesmlx/types"
)

// FacebookScraper is an announcement scraper for the Metro de Lisboa Facebook page
// It reads the Facebook feed using the Facebook API
type FacebookScraper struct {
	ticker         *time.Ticker
	stopChan       chan struct{}
	log            *log.Logger
	newAnnCallback func(announcement *types.Announcement)
	firstUpdate    bool
	announcements  []*types.Announcement
	running        bool
	imageURLcache  *cache.Cache

	AccessToken string
	Network     *types.Network
	Period      time.Duration
}

// ID returns the ID of this scraper
func (sc *FacebookScraper) ID() string {
	return "sc-pt-ml-facebook"
}

// Init initializes the scraper
func (sc *FacebookScraper) Init(log *log.Logger,
	newAnnCallback func(announcement *types.Announcement)) {
	sc.stopChan = make(chan struct{})
	sc.ticker = time.NewTicker(sc.Period)
	sc.log = log
	sc.newAnnCallback = newAnnCallback
	sc.firstUpdate = true
	sc.imageURLcache = cache.New(12*time.Hour, 30*time.Minute)

	sc.log.Println("FacebookScraper initializing")
	sc.update()
	sc.log.Println("FacebookScraper completed first fetch")
}

// Begin starts the scraper
func (sc *FacebookScraper) Begin() {
	sc.stopChan = make(chan struct{})
	sc.ticker = time.NewTicker(sc.Period)
	sc.running = true
	go sc.scrape()
}

// End stops the scraper
func (sc *FacebookScraper) End() {
	sc.ticker.Stop()
	close(sc.stopChan)
	sc.running = false
}

// Running returns whether the scraper is running
func (sc *FacebookScraper) Running() bool {
	return sc.running
}

func (sc *FacebookScraper) copyAnnouncements() []*types.Announcement {
	c := make([]*types.Announcement, len(sc.announcements))
	for i, annPointer := range sc.announcements {
		ann := *annPointer
		c[i] = &ann
	}
	return c
}

func (sc *FacebookScraper) scrape() {
	sc.update()
	sc.log.Println("FacebookScraper completed second fetch")
	for {
		select {
		case <-sc.ticker.C:
			sc.update()
			sc.log.Println("FacebookScraper fetch complete")
		case <-sc.stopChan:
			return
		}
	}
}

func (sc *FacebookScraper) update() {
	response, err := http.Get("https://mobile.facebook.com/metrolisboa")
	if err != nil {
		sc.log.Println(err)
		return
	}

	doc, err := goquery.NewDocumentFromResponse(response)
	if err != nil {
		sc.log.Println(err)
		return
	}

	recent := doc.Find("#recent")
	if recent.Length() == 0 {
		sc.log.Println("Missing elements in response")
		return
	}
	recent = recent.Children().First()
	if recent.Length() == 0 {
		sc.log.Println("Missing elements in response")
		return
	}
	recent = recent.Children().First()
	if recent.Length() == 0 {
		sc.log.Println("Missing elements in response")
		return
	}
	announcements := []*types.Announcement{}
	recent.Children().Each(func(i int, s *goquery.Selection) {
		dataft, ok := s.Attr("data-ft")
		if !ok {
			sc.log.Println("Post missing data-ft attribute")
		}

		var data map[string]interface{}
		if err := json.Unmarshal([]byte(dataft), &data); err != nil {
			sc.log.Println(err)
			return
		}
		if data == nil {
			sc.log.Println("data-ft is not valid json")
			return
		}

		postIDiface := data["mf_story_key"]
		if postIDiface == nil {
			sc.log.Println("data-ft missing mf_story_key")
			return
		}
		postID, ok := postIDiface.(string)
		if !ok {
			sc.log.Println("mf_story_key has unexpected type")
			return
		}

		pageInsights := data["page_insights"]
		if pageInsights == nil {
			sc.log.Println("data-ft missing page_insights")
			return
		}
		pageInsightsMap, ok := pageInsights.(map[string]interface{})
		if !ok {
			sc.log.Println("page_insights has unexpected type")
			return
		}

		var postTime time.Time
		for _, item := range pageInsightsMap {
			// we only care about the item which has publish_time, we don't care what's its key
			pi, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			postContextIface, ok := pi["post_context"]
			if !ok {
				continue
			}
			postContext, ok := postContextIface.(map[string]interface{})
			if !ok {
				continue
			}
			tIface, ok := postContext["publish_time"]
			if !ok {
				continue
			}
			tfloat, ok := tIface.(float64)
			if !ok {
				continue
			}
			postTime = time.Unix(int64(tfloat), 0)
		}
		if postTime.IsZero() {
			sc.log.Println("could not obtain post publish time")
			return
		}

		body := s.Find("div:nth-child(2) > span").First().Find("p").Text()

		imgURL, _ := s.Find("div:nth-child(3) img").First().Attr("src")

		ann := types.Announcement{
			Time:     postTime,
			Network:  sc.Network,
			Title:    "",
			Body:     body,
			ImageURL: imgURL,
			URL:      "https://www.facebook.com/MetroLisboa/posts/" + postID,
			Source:   "pt-ml-facebook",
		}
		announcements = append(announcements, &ann)
	})

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
	sc.firstUpdate = false
}

// Networks returns the networks monitored by this scraper
func (sc *FacebookScraper) Networks() []*types.Network {
	return []*types.Network{sc.Network}
}

// Sources returns the sources provided by this scraper
func (sc *FacebookScraper) Sources() []string {
	return []string{"pt-ml-facebook"}
}

// Announcements returns the announcements read by this scraper
func (sc *FacebookScraper) Announcements(source string) []*types.Announcement {
	return sc.copyAnnouncements()
}
