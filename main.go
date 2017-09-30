package main

import (
	"log"
	"os"
	"time"

	fcm "github.com/NaySoftware/go-fcm"
	"github.com/SaidinWoT/timespan"
	"github.com/heetch/sqalx"
	"github.com/jmoiron/sqlx"

	sq "github.com/gbl08ma/squirrel"

	"github.com/gbl08ma/disturbancesmlx/dataobjects"
	"github.com/gbl08ma/keybox"
)

var (
	rdb           *sqlx.DB
	sdb           sq.StatementBuilderType
	rootSqalxNode sqalx.Node
	secrets       *keybox.Keybox
	fcmcl         *fcm.FcmClient
	mainLog       = log.New(os.Stdout, "", log.Ldate|log.Ltime)
	webLog        = log.New(os.Stdout, "web", log.Ldate|log.Ltime)
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
	secrets, err = keybox.Open(SecretsPath)
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

	fcmServerKey, present := secrets.Get("firebaseServerKey")
	if !present {
		mainLog.Fatal("Firebase server key not present in keybox")
	}
	fcmcl = fcm.NewFcmClient(fcmServerKey)

	SetUpScrapers()
	defer TearDownScrapers()

	facebookAccessToken, present := secrets.Get("facebookToken")
	if !present {
		mainLog.Fatal("Facebook API access token not present in keybox")
	}

	SetUpAnnouncements(facebookAccessToken)
	defer TearDownAnnouncements()

	go WebServer()

	certPath := DefaultClientCertPath
	if len(os.Args) > 1 {
		certPath = os.Args[1]
	}
	go APIserver(certPath)

	/*err = ComputeTypicalSeconds(rootSqalxNode)
	if err != nil {
		mainLog.Fatal(err)
	}*/

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
