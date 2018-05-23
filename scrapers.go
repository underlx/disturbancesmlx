package main

import (
	"log"
	"os"
	"strings"
	"time"

	"github.com/underlx/disturbancesmlx/discordbot"

	"github.com/heetch/sqalx"
	"github.com/underlx/disturbancesmlx/dataobjects"
	"github.com/underlx/disturbancesmlx/scraper"
	"github.com/underlx/disturbancesmlx/scraper/mlxscraper"
)

var (
	mlxscr    scraper.StatusScraper
	rssmlxscr scraper.AnnouncementScraper
	fbmlxscr  scraper.AnnouncementScraper

	scrapers = make(map[string]scraper.Scraper)
)

// SetUpScrapers initializes and starts the scrapers used to obtain network information
func SetUpScrapers(node sqalx.Node) error {
	tx, err := node.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	lisbonLoc, _ := time.LoadLocation("Europe/Lisbon")

	network, err := dataobjects.GetNetwork(tx, MLnetworkID)
	if err != nil {
		// network does not exist, create it
		network = &dataobjects.Network{
			ID:         MLnetworkID,
			Name:       "Metro de Lisboa",
			MainLocale: "pt",
			Names: map[string]string{
				"en": "Lisbon Metro",
				"fr": "Métro de Lisbonne",
				"pt": "Metro de Lisboa",
			},
			TypicalCars:  6,
			Holidays:     []int64{},
			OpenTime:     dataobjects.Time(time.Date(0, 0, 0, 6, 30, 0, 0, lisbonLoc)),
			OpenDuration: dataobjects.Duration(18*time.Hour + 30*time.Minute),
			Timezone:     "Europe/Lisbon",
			NewsURL:      "http://www.metrolisboa.pt/feed/",
		}
		err = network.Update(tx)
		if err != nil {
			return err
		}
	}

	l := log.New(os.Stdout, "mlxscraper", log.Ldate|log.Ltime)
	mlxscr = &mlxscraper.Scraper{
		URL:     "http://app.metrolisboa.pt/status/estado_Linhas.php",
		Network: network,
		Source: &dataobjects.Source{
			ID:        "mlxscraper-pt-ml",
			Name:      "Metro de Lisboa estado_Linhas.php",
			Automatic: true,
			Official:  true,
		},
		Period: 1 * time.Minute,
	}
	mlxscr.Init(l, handleNewStatus, handleTopologyChange)
	mlxscr.Begin()
	scrapers[mlxscr.ID()] = mlxscr
	return tx.Commit()
}

func handleNewStatus(status *dataobjects.Status) {
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

	err = status.Line.AddStatus(tx, status, true)
	if err != nil {
		mainLog.Println(err)
		return
	}

	err = tx.Commit()
	if err != nil {
		mainLog.Println(err)
		return
	}

	lastChange = time.Now().UTC()
}

func handleTopologyChange(s scraper.StatusScraper) {
	tx, err := rootSqalxNode.Beginx()
	if err != nil {
		mainLog.Println(err)
		return
	}
	defer tx.Rollback()
	for _, newnetwork := range s.Networks() {
		_, err := dataobjects.GetNetwork(tx, newnetwork.ID)
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
		_, err := dataobjects.GetLine(tx, newline.ID)
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
		rssmlxscr.Init(rssl, SendNotificationForAnnouncement)
		rssmlxscr.Begin()
		scrapers[rssmlxscr.ID()] = rssmlxscr

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
		fbmlxscr.Init(fbl, SendNotificationForAnnouncement)
		fbmlxscr.Begin()
		scrapers[fbmlxscr.ID()] = fbmlxscr

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

func handleControlScraper(command *discordbot.ControlScraperCommand) {
	if scraper, ok := scrapers[command.Scraper]; ok {
		if command.Enable && !scraper.Running() {
			scraper.Begin()
			command.MessageCallback("✅")
		} else if scraper.Running() {
			scraper.End()
			command.MessageCallback("✅")
		} else {
			command.MessageCallback("❌ already started/stopped")
		}
		return
	}
	scraperIDs := make([]string, len(scrapers))
	i := 0
	for id := range scrapers {
		scraperIDs[i] = "`" + id + "`"
		i++
	}
	command.MessageCallback("🆖 second argument must be one of [" + strings.Join(scraperIDs, ",") + "]")
}
