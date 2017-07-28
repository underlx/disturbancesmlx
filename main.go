package main

import (
	"log"
	"os"
	"time"

	fcm "github.com/NaySoftware/go-fcm"
	"github.com/SaidinWoT/timespan"
	"github.com/heetch/sqalx"
	"github.com/jmoiron/sqlx"
	uuid "github.com/satori/go.uuid"

	sq "github.com/gbl08ma/squirrel"

	"github.com/gbl08ma/disturbancesmlx/dataobjects"
	"github.com/gbl08ma/disturbancesmlx/scraper"
	"github.com/gbl08ma/disturbancesmlx/scraper/mlxscraper"
	"github.com/gbl08ma/keybox"
)

const (
	DEBUG       = true
	MLnetworkID = "pt-ml"
)

var (
	rdb           *sqlx.DB
	sdb           sq.StatementBuilderType
	rootSqalxNode sqalx.Node
	secrets       *keybox.Keybox
	fcmcl         *fcm.FcmClient
	mainLog       = log.New(os.Stdout, "", log.Ldate|log.Ltime)
	webLog        = log.New(os.Stdout, "web", log.Ldate|log.Ltime)
	mlxscr        scraper.Scraper
	lastChange    time.Time
)

// MLcalculator implements resource.StatsCalculator
type MLcalculator struct{}

func (*MLcalculator) Availability(node sqalx.Node, line *dataobjects.Line, startTime time.Time, endTime time.Time) (float64, time.Duration, error) {
	return MLlineAvailability(node, line, startTime, endTime)
}

// MLlastDisturbanceTime returns the time of the latest Metro de Lisboa disturbance
func MLlastDisturbanceTime(node sqalx.Node) (t time.Time, err error) {
	tx, err := node.Beginx()
	if err != nil {
		return time.Now().UTC(), err
	}
	defer tx.Commit() // read-only tx

	n, err := dataobjects.GetNetwork(tx, MLnetworkID)
	if err != nil {
		return time.Now().UTC(), err
	}
	d, err := n.LastDisturbance(tx)
	if err != nil {
		return time.Now().UTC(), err
	}

	if !d.Ended {
		return time.Now().UTC(), nil
	}

	return d.EndTime, nil
}

// MLlineAvailability returns the availability for a Metro de Lisboa line
func MLlineAvailability(node sqalx.Node, line *dataobjects.Line, startTime time.Time, endTime time.Time) (float64, time.Duration, error) {
	// calculate closed time
	var closedDuration time.Duration
	ct := startTime
	wholeSpan := timespan.New(startTime, endTime.Sub(startTime))
	for ct.Before(endTime) {
		closeTime := time.Date(ct.Year(), ct.Month(), ct.Day(), 1, 0, 0, 0, ct.Location())
		openTime := time.Date(ct.Year(), ct.Month(), ct.Day(), 6, 30, 0, 0, ct.Location())

		closedSpan := timespan.New(closeTime, openTime.Sub(closeTime))
		d, hasIntersection := wholeSpan.Intersection(closedSpan)
		if hasIntersection {
			closedDuration += d.Duration()
		}
		ct = ct.AddDate(0, 0, 1)
	}

	return line.Availability(node, startTime, endTime, closedDuration)
}

func main() {
	var err error
	mainLog.Println("Server starting, opening keybox...")
	if DEBUG {
		secrets, err = keybox.Open("secrets-debug.json")
	} else {
		secrets, err = keybox.Open("secrets.json")
	}
	if err != nil {
		mainLog.Fatal(err)
	}
	mainLog.Println("Keybox opened")

	mainLog.Println("Opening database...")
	databaseURI, present := secrets.Get("databaseURI")
	if !present {
		mainLog.Fatal("Database connection string not present in keybox")
	}
	rdb, err = sqlx.Open("postgres", databaseURI)
	if err != nil {
		mainLog.Fatal(err)
	}
	defer rdb.Close()

	err = rdb.Ping()
	if err != nil {
		mainLog.Fatal(err)
	}
	sdb = sq.StatementBuilder.PlaceholderFormat(sq.Dollar).RunWith(rdb)

	rootSqalxNode, err = sqalx.New(rdb)
	if err != nil {
		mainLog.Fatal(err)
	}

	mainLog.Println("Database opened")

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
	defer mlxscr.End()

	go WebServer()

	certPath := "trusted_client_cert.pem"
	if len(os.Args) > 1 {
		certPath = os.Args[1]
	}
	go APIserver(certPath)

	fcmServerKey, present := secrets.Get("firebaseServerKey")
	if !present {
		mainLog.Fatal("Firebase server key not present in keybox")
	}
	fcmcl = fcm.NewFcmClient(fcmServerKey)

	for {
		if DEBUG {
			printLatestDisturbance(rootSqalxNode)
			ld, err := MLlastDisturbanceTime(rootSqalxNode)
			if err != nil {
				mainLog.Println(err)
			}
			mainLog.Printf("Last disturbance: %s", ld.String())
		}
		time.Sleep(1 * time.Minute)
	}
}

func printLatestDisturbance(node sqalx.Node) {
	tx, err := node.Beginx()
	if err != nil {
		mainLog.Println(err)
		return
	}
	defer tx.Commit() // read-only tx

	n, err := dataobjects.GetNetwork(tx, MLnetworkID)
	if err != nil {
		mainLog.Println(err)
		return
	}
	d, err := n.LastDisturbance(tx)
	if err == nil {
		mainLog.Println("Network last disturbance at", d.StartTime, "description:", d.Description)
	} else {
		mainLog.Println(err)
	}
}
