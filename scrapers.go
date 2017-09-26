package main

import (
	"log"
	"os"
	"time"

	"github.com/gbl08ma/disturbancesmlx/dataobjects"
	"github.com/gbl08ma/disturbancesmlx/scraper"
	"github.com/gbl08ma/disturbancesmlx/scraper/mlxscraper"
	uuid "github.com/satori/go.uuid"
)

var (
	mlxscr    scraper.Scraper
	rssmlxscr scraper.AnnouncementScraper
)

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
	mlxscr.Begin(l, func(status *dataobjects.Status) {
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
			disturbance = &dataobjects.Disturbance{
				ID:   uuid.NewV4().String(),
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
	},
		func(s scraper.Scraper) {
			tx, err := rootSqalxNode.Beginx()
			if err != nil {
				mainLog.Println(err)
				return
			}
			defer tx.Rollback()
			for _, newnetwork := range s.Networks() {
				newnetwork, err := dataobjects.GetNetwork(tx, newnetwork.ID)
				if err != nil {
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
				if err != nil {
					mainLog.Println("New line " + newline.ID)
					err = newline.Update(tx)
					if err != nil {
						mainLog.Println(err)
						return
					}
				}
			}
			tx.Commit()
		})

}

func TearDownScrapers() {
	mlxscr.End()
}

func SetUpAnnouncements() {
	rssl := log.New(os.Stdout, "rssscraper", log.Ldate|log.Ltime)
	network, err := dataobjects.GetNetwork(rootSqalxNode, MLnetworkID)
	if err != nil {
		mainLog.Println(err)
	} else {
		rssmlxscr = &mlxscraper.RSSScraper{
			URL:     network.NewsURL,
			Network: network,
			Period:  1 * time.Minute,
		}
		rssmlxscr.Begin(rssl,
			func(announcement *dataobjects.Announcement) {},
			func(anns []*dataobjects.Announcement) {})
	}
}

func TearDownAnnouncements() {
	if rssmlxscr != nil {
		rssmlxscr.End()
	}
}
