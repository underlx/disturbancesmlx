package main

import (
	"log"
	"os"
	"time"

	uuid "github.com/satori/go.uuid"
	"github.com/underlx/disturbancesmlx/dataobjects"
	"github.com/underlx/disturbancesmlx/scraper"
	"github.com/underlx/disturbancesmlx/scraper/mlxscraper"
)

var (
	mlxscr    scraper.Scraper
	rssmlxscr scraper.AnnouncementScraper
	fbmlxscr  scraper.AnnouncementScraper
)

// SetUpScrapers initializes and starts the scrapers used to obtain network information
func SetUpScrapers() {
	lisbonLoc, _ := time.LoadLocation("Europe/Lisbon")

	l := log.New(os.Stdout, "mlxscraper", log.Ldate|log.Ltime)
	mlxscr = &mlxscraper.Scraper{
		URL: "http://app.metrolisboa.pt/status/estado_Linhas.php",
		Network: &dataobjects.Network{
			ID:           MLnetworkID,
			Name:         "Metro de Lisboa",
			TypicalCars:  6,
			Holidays:     []int64{},
			OpenTime:     dataobjects.Time(time.Date(0, 0, 0, 6, 30, 0, 0, lisbonLoc)),
			OpenDuration: dataobjects.Duration(18*time.Hour + 30*time.Minute),
			Timezone:     "Europe/Lisbon",
			NewsURL:      "http://www.metrolisboa.pt/feed/",
		},
		Source: &dataobjects.Source{
			ID:          "mlxscraper-pt-ml",
			Name:        "Metro de Lisboa estado_Linhas.php",
			IsAutomatic: true,
		},
		Period: 1 * time.Minute,
	}
	mlxscr.Begin(l, mlxHandleNewStatus, mlxHandleTopologyChange)
}

func mlxHandleNewStatus(status *dataobjects.Status) {
	tx, err := rootSqalxNode.Beginx()
	if err != nil {
		mainLog.Println(err)
		return
	}
	defer tx.Rollback()

	log.Println("New status for line", status.Line.Name, "on network", status.Line.Network.Name)
	log.Println("  ", status.Status)
	if status.IsDowntime {
		log.Println("   Is disturbance!")
	}

	err = status.Update(tx)
	if err != nil {
		mainLog.Println(err)
		return
	}

	d, err := status.Line.LastDisturbance(tx)
	if err == nil {
		mainLog.Println("   Last disturbance at", d.StartTime, "description:", d.Description)
	} else {
		mainLog.Println(err)
	}

	disturbances, err := status.Line.OngoingDisturbances(tx)
	if err != nil {
		mainLog.Println(err)
		return
	}
	found := len(disturbances) > 0
	var disturbance *dataobjects.Disturbance
	if found {
		mainLog.Println("   Found ongoing disturbance")
		disturbance = disturbances[len(disturbances)-1]
	} else {
		id, err := uuid.NewV4()
		if err != nil {
			mainLog.Println(err)
			return
		}
		disturbance = &dataobjects.Disturbance{
			ID:   id.String(),
			Line: status.Line,
		}
	}
	previousStatus := disturbance.LatestStatus()
	if previousStatus != nil && status.Status == previousStatus.Status {
		mainLog.Println("   Repeated status, ignore")
		return
	}
	sendNotification := false
	if status.IsDowntime && !found {
		disturbance.StartTime = status.Time
		disturbance.Description = status.Status
		disturbance.Statuses = append(disturbance.Statuses, status)
		err = disturbance.Update(tx)
		if err != nil {
			mainLog.Println(err)
			return
		}
		sendNotification = true
	} else if found {
		if !status.IsDowntime {
			// "close" this disturbance
			disturbance.EndTime = status.Time
			disturbance.Ended = true
		}
		disturbance.Statuses = append(disturbance.Statuses, status)
		err = disturbance.Update(tx)
		if err != nil {
			mainLog.Println(err)
			return
		}
		sendNotification = true
	}
	lastChange = time.Now().UTC()
	if tx.Commit() == nil && sendNotification {
		SendNotificationForDisturbance(disturbance, status)
	}
}

func mlxHandleTopologyChange(s scraper.Scraper) {
	tx, err := rootSqalxNode.Beginx()
	if err != nil {
		mainLog.Println(err)
		return
	}
	defer tx.Rollback()
	for _, newnetwork := range s.Networks() {
		newnetwork, err := dataobjects.GetNetwork(tx, newnetwork.ID)
		if err == nil {
			mainLog.Println("New network " + newnetwork.ID)
			err = newnetwork.Update(tx)
			if err != nil {
				mainLog.Println(err)
				return
			}
		}
	}
	for _, newline := range s.Lines() {
		newline, err := dataobjects.GetLine(tx, newline.ID)
		if err == nil {
			mainLog.Println("New line " + newline.ID)
			err = newline.Update(tx)
			if err != nil {
				mainLog.Println(err)
				return
			}
		}
	}
	tx.Commit()
}

// TearDownScrapers terminates and cleans up the scrapers used to obtain network information
func TearDownScrapers() {
	mlxscr.End()
}

// SetUpAnnouncements sets up the scrapers used to obtain network announcements
func SetUpAnnouncements(facebookAccessToken string) {
	network, err := dataobjects.GetNetwork(rootSqalxNode, MLnetworkID)

	rssl := log.New(os.Stdout, "rssscraper", log.Ldate|log.Ltime)
	if err != nil {
		mainLog.Println(err)
	} else {
		rssmlxscr = &mlxscraper.RSSScraper{
			URL:     network.NewsURL,
			Network: network,
			Period:  1 * time.Minute,
		}
		rssmlxscr.Begin(rssl, SendNotificationForAnnouncement)

		annStore.AddScraper(rssmlxscr)
	}

	fbl := log.New(os.Stdout, "fbscraper", log.Ldate|log.Ltime)
	if err != nil {
		mainLog.Println(err)
	} else {
		fbmlxscr = &mlxscraper.FacebookScraper{
			AccessToken: facebookAccessToken,
			Network:     network,
			Period:      1 * time.Minute,
		}
		fbmlxscr.Begin(fbl, SendNotificationForAnnouncement)

		annStore.AddScraper(fbmlxscr)
	}
}

// TearDownAnnouncements terminates and cleans up the scrapers used to obtain network announcements
func TearDownAnnouncements() {
	if rssmlxscr != nil {
		rssmlxscr.End()
	}
	if fbmlxscr != nil {
		fbmlxscr.End()
	}
}
