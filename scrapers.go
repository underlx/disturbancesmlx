package main

import (
	"io/ioutil"
	"log"
	"os"
	"strings"
	"time"

	"github.com/gbl08ma/sqalx"
	"github.com/underlx/disturbancesmlx/scraper"
	"github.com/underlx/disturbancesmlx/scraper/mlxscraper"
	"github.com/underlx/disturbancesmlx/types"
)

var (
	mlxscr           scraper.StatusScraper
	mlxETAscr        scraper.ETAScraper
	mlxcondscr       scraper.ETAScraper
	rssmlxscr        scraper.AnnouncementScraper
	fbmlxscr         scraper.AnnouncementScraper
	contestscr       scraper.AnnouncementScraper
	institutionalscr scraper.AnnouncementScraper

	scrapers = make(map[string]scraper.Scraper)
)

// SetUpScrapers initializes and starts the scrapers used to obtain network information
func SetUpScrapers(node sqalx.Node, mlAccessToken string) error {
	tx, err := node.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	lisbonLoc, _ := time.LoadLocation("Europe/Lisbon")

	network, err := types.GetNetwork(tx, MLnetworkID)
	if err != nil {
		// network does not exist, create it
		network = &types.Network{
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
			OpenTime:     types.Time(time.Date(0, 0, 0, 6, 30, 0, 0, lisbonLoc)),
			OpenDuration: types.Duration(18*time.Hour + 30*time.Minute),
			Timezone:     "Europe/Lisbon",
			NewsURL:      "http://www.metrolisboa.pt/feed/",
		}
		err = network.Update(tx)
		if err != nil {
			return err
		}
	}

	mlxscr = &mlxscraper.Scraper{
		StatusCallback: handleNewStatusNotify,
		Network:        network,
		Source: &types.Source{
			ID:        "mlxscraper-pt-ml",
			Name:      "Metro de Lisboa estado_Linhas.php",
			Automatic: true,
			Official:  true,
		},
		Period:      20 * time.Second,
		BearerToken: mlAccessToken,
		EndpointURL: "https://api.metrolisboa.pt:8243/estadoServicoML/1.0.1",
	}
	mlxscr.Init(rootSqalxNode,
		log.New(os.Stdout, "mlxscraper", log.Ldate|log.Ltime))
	mlxscr.Begin()
	scrapers[mlxscr.ID()] = mlxscr

	if mlAccessToken != "" {
		mlxETAscr = &mlxscraper.ETAScraper{
			NewETACallback: vehicleETAHandler.RegisterVehicleETA,
			BearerToken:    mlAccessToken,
			EndpointURL:    "https://api.metrolisboa.pt:8243/estadoServicoML/1.0.1",
			Network:        network,
			Period:         10 * time.Second,
		}
		err = mlxETAscr.Init(rootSqalxNode,
			log.New(os.Stdout, "mlxETAscraper", log.Ldate|log.Ltime))
		if err != nil {
			return err
		}
		mlxETAscr.Begin()
		scrapers[mlxETAscr.ID()] = mlxETAscr

		mlxcondscr = &mlxscraper.ConditionsScraper{
			ConditionCallback: handleNewCondition,
			BearerToken:       mlAccessToken,
			EndpointURL:       "https://api.metrolisboa.pt:8243/estadoServicoML/1.0.1",
			Network:           network,
			Source: &types.Source{
				ID:        "mlxcond-pt-ml",
				Name:      "Metro de Lisboa EstadoServicoML",
				Automatic: true,
				Official:  true,
			},
			Period: 1 * time.Minute,
		}
		err = mlxcondscr.Init(rootSqalxNode,
			log.New(os.Stdout, "mlxcondscraper", log.Ldate|log.Ltime))
		if err != nil {
			return err
		}
		mlxcondscr.Begin()
		scrapers[mlxcondscr.ID()] = mlxcondscr
	} else {
		log.Println("Not scraping pt-ml ETAs or line conditions, as access token is not present")
	}
	return tx.Commit()
}

func handleNewStatusNotify(status *types.Status) {
	handleNewStatus(status, true)
}

func handleNewStatus(status *types.Status, allowNotify bool) {
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

	err = status.Line.AddStatus(tx, status, allowNotify)
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

func handleNewCondition(condition *types.LineCondition) {
	tx, err := rootSqalxNode.Beginx()
	if err != nil {
		mainLog.Println(err)
		return
	}
	defer tx.Rollback()

	latest, err := condition.Line.LastCondition(tx)
	if err != nil || latest.TrainCars != condition.TrainCars || latest.TrainFrequency != condition.TrainFrequency {
		err = condition.Update(tx)
		if err != nil {
			mainLog.Println(err)
			return
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
	network, err := types.GetNetwork(rootSqalxNode, MLnetworkID)
	if err != nil {
		mainLog.Println(err)
		return
	}

	rssl := log.New(os.Stdout, "rssscraper", log.Ldate|log.Ltime)
	rssmlxscr = &mlxscraper.RSSScraper{
		ScraperID: "sc-pt-ml-rss",
		URL:       network.NewsURL,
		Network:   network,
		Period:    1 * time.Minute,
	}
	rssmlxscr.Init(rssl, SendNotificationForAnnouncement)
	rssmlxscr.Begin()
	scrapers[rssmlxscr.ID()] = rssmlxscr

	annStore.AddScraper(rssmlxscr)

	fbl := log.New(os.Stdout, "fbscraper", log.Ldate|log.Ltime)
	fbmlxscr = &mlxscraper.FacebookScraper{
		AccessToken: facebookAccessToken,
		Network:     network,
		Period:      1 * time.Minute,
	}
	fbmlxscr.Init(fbl, SendNotificationForAnnouncement)
	fbmlxscr.Begin()
	scrapers[fbmlxscr.ID()] = fbmlxscr

	annStore.AddScraper(fbmlxscr)

	// contest scraper - not really connected to the general announcements framework for now
	contestl := log.New(os.Stdout, "contestscraper", log.Ldate|log.Ltime)

	contestscr = &mlxscraper.RSSScraper{
		ScraperID: "sc-pt-ml-contests-rss",
		URL:       "https://passatempos.metrolisboa.pt/feed/",
		Network:   network,
		Period:    5 * time.Minute,
	}
	contestscr.Init(contestl, SendNotificationForContest)
	contestscr.Begin()
	scrapers[contestscr.ID()] = contestscr

	// institutional website scraper - not really connected to the general announcements framework for now
	institutionall := log.New(os.Stdout, "institutionalscraper", log.Ldate|log.Ltime)

	institutionalscr = &mlxscraper.RSSScraper{
		ScraperID: "sc-pt-ml-institutional-rss",
		URL:       "https://www.metrolisboa.pt/institucional/feed/",
		Network:   network,
		Period:    5 * time.Minute,
	}
	institutionalscr.Init(institutionall, SendNotificationForInstitutionalPost)
	institutionalscr.Begin()
	scrapers[institutionalscr.ID()] = institutionalscr
}

// RegisterAndStartNewRSSScraper registers and starts a new ad-hoc RSS scraper
func RegisterAndStartNewRSSScraper(scraperID string, url string, network *types.Network, period time.Duration, callback func(announcement *types.Announcement)) *mlxscraper.RSSScraper {
	slog := log.New(ioutil.Discard, "", log.Ldate|log.Ltime)

	scr := &mlxscraper.RSSScraper{
		ScraperID: scraperID,
		URL:       url,
		Network:   network,
		Period:    period,
	}
	scr.Init(slog, callback)
	scr.Begin()
	scrapers[scr.ID()] = scr
	return scr
}

// TearDownAnnouncements terminates and cleans up the scrapers used to obtain network announcements
func TearDownAnnouncements() {
	if rssmlxscr != nil {
		rssmlxscr.End()
	}
	if fbmlxscr != nil {
		fbmlxscr.End()
	}
	if contestscr != nil {
		contestscr.End()
	}
}

func handleControlScraper(scraperID string, enable bool, messageCallback func(message string)) {
	if scraper, ok := scrapers[scraperID]; ok {
		if enable && !scraper.Running() {
			scraper.Begin()
			messageCallback("✅")
		} else if scraper.Running() {
			scraper.End()
			messageCallback("✅")
		} else {
			messageCallback("❌ already started/stopped")
		}
		return
	}
	scraperIDs := make([]string, len(scrapers))
	i := 0
	for id := range scrapers {
		scraperIDs[i] = "`" + id + "`"
		i++
	}
	messageCallback("🆖 second argument must be one of [" + strings.Join(scraperIDs, ",") + "]")
}
