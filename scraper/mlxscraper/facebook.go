package mlxscraper

import (
	"encoding/json"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"

	cache "github.com/patrickmn/go-cache"
	"github.com/underlx/disturbancesmlx/dataobjects"
)

// FacebookScraper is an announcement scraper for the Metro de Lisboa Facebook page
// It reads the Facebook feed using the Facebook API
type FacebookScraper struct {
	ticker         *time.Ticker
	stopChan       chan struct{}
	log            *log.Logger
	newAnnCallback func(announcement *dataobjects.Announcement)
	firstUpdate    bool
	announcements  []*dataobjects.Announcement
	running        bool
	imageURLcache  *cache.Cache

	AccessToken string
	Network     *dataobjects.Network
	Period      time.Duration
}

// ID returns the ID of this scraper
func (sc *FacebookScraper) ID() string {
	return "sc-pt-ml-facebook"
}

// Init initializes the scraper
func (sc *FacebookScraper) Init(log *log.Logger,
	newAnnCallback func(announcement *dataobjects.Announcement)) {
	sc.stopChan = make(chan struct{})
	sc.ticker = time.NewTicker(sc.Period)
	sc.log = log
	sc.newAnnCallback = newAnnCallback
	sc.firstUpdate = true
	sc.imageURLcache = cache.New(1*time.Hour, 30*time.Minute)

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

func (sc *FacebookScraper) copyAnnouncements() []*dataobjects.Announcement {
	c := make([]*dataobjects.Announcement, len(sc.announcements))
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
	response, err := http.Get("https://graph.facebook.com/MetroLisboa/posts?access_token=" + sc.AccessToken)
	if err != nil {
		sc.log.Println(err)
		return
	}
	defer response.Body.Close()

	var dat map[string]interface{}
	if err := json.NewDecoder(response.Body).Decode(&dat); err != nil {
		sc.log.Println(err)
		return
	}
	if dat == nil || dat["data"] == nil {
		sc.log.Println("FacebookScraper data is nil")
		return
	}
	data := dat["data"].([]interface{})

	announcements := []*dataobjects.Announcement{}
	for _, item := range data {
		pitem := item.(map[string]interface{})
		body := ""
		if pitem["message"] != nil {
			body = pitem["message"].(string)
		} else if pitem["story"] != nil {
			body = pitem["story"].(string)
		}
		postTime, err := time.Parse("2006-01-02T15:04:05-0700", pitem["created_time"].(string))
		if err != nil {
			sc.log.Println(err)
			continue
		}

		ids := strings.Split(pitem["id"].(string), "_")

		ann := dataobjects.Announcement{
			Time:     postTime,
			Network:  sc.Network,
			Title:    "",
			Body:     body,
			ImageURL: sc.getImageForPost(pitem["id"].(string)),
			URL:      "https://www.facebook.com/" + ids[0] + "/posts/" + ids[1],
			Source:   "pt-ml-facebook",
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
	sc.firstUpdate = false
}

func (sc *FacebookScraper) getImageForPost(postID string) string {
	url, present := sc.imageURLcache.Get(postID)
	if present {
		return url.(string)
	}

	netClient := &http.Client{
		Timeout: time.Second * 10,
	}
	objIDresponse, err := netClient.Get("https://graph.facebook.com/" + postID + "?fields=object_id&access_token=" + sc.AccessToken)
	if err != nil {
		return ""
	}

	var dat map[string]interface{}
	if err := json.NewDecoder(objIDresponse.Body).Decode(&dat); err != nil {
		return ""
	}
	if dat == nil || dat["object_id"] == nil {
		return ""
	}
	imageURL := "https://graph.facebook.com/" + dat["object_id"].(string) + "/picture"
	sc.imageURLcache.SetDefault(postID, imageURL)
	return imageURL
}

// Networks returns the networks monitored by this scraper
func (sc *FacebookScraper) Networks() []*dataobjects.Network {
	return []*dataobjects.Network{sc.Network}
}

// Sources returns the sources provided by this scraper
func (sc *FacebookScraper) Sources() []string {
	return []string{"pt-ml-facebook"}
}

// Announcements returns the announcements read by this scraper
func (sc *FacebookScraper) Announcements(source string) []*dataobjects.Announcement {
	return sc.copyAnnouncements()
}
