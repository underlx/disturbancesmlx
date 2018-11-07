package main

import (
	"log"
	"os"
	"time"

	fcm "github.com/NaySoftware/go-fcm"
	"github.com/gbl08ma/sqalx"
	"github.com/jmoiron/sqlx"

	sq "github.com/gbl08ma/squirrel"

	"github.com/gbl08ma/keybox"
	"github.com/underlx/disturbancesmlx/compute"
	"github.com/underlx/disturbancesmlx/dataobjects"
)

var (
	rdb              *sqlx.DB
	sdb              sq.StatementBuilderType
	rootSqalxNode    sqalx.Node
	secrets          *keybox.Keybox
	fcmcl            *fcm.FcmClient
	mainLog          = log.New(os.Stdout, "", log.Ldate|log.Ltime)
	discordLog       = log.New(os.Stdout, "discord", log.Ldate|log.Ltime)
	posplayLog       = log.New(os.Stdout, "posplay", log.Ldate|log.Ltime)
	webLog           = log.New(os.Stdout, "web", log.Ldate|log.Ltime)
	lastChange       time.Time
	apiTotalRequests int

	vehicleHandler *compute.VehicleHandler
	reportHandler  *compute.ReportHandler
	statsHandler   *compute.StatsHandler

	// GitCommit is provided by govvv at compile-time
	GitCommit = "???"
	// BuildDate is provided by govvv at compile-time
	BuildDate = "???"
)

func main() {
	var err error
	mainLog.Println("Server starting, opening keybox...")
	secrets, err = keybox.Open(SecretsPath)
	if err != nil {
		mainLog.Fatalln(err)
	}
	mainLog.Println("Keybox opened")

	mainLog.Println("Opening database...")
	databaseURI, present := secrets.Get("databaseURI")
	if !present {
		mainLog.Fatalln("Database connection string not present in keybox")
	}
	rdb, err = sqlx.Open("postgres", databaseURI)
	if err != nil {
		mainLog.Fatalln(err)
	}
	defer rdb.Close()

	err = rdb.Ping()
	if err != nil {
		mainLog.Fatalln(err)
	}
	rdb.SetMaxOpenConns(MaxDBconnectionPoolSize)
	sdb = sq.StatementBuilder.PlaceholderFormat(sq.Dollar).RunWith(rdb)

	rootSqalxNode, err = sqalx.New(rdb)
	if err != nil {
		mainLog.Fatalln(err)
	}

	statsHandler = compute.NewStatsHandler()
	vehicleHandler = compute.NewVehicleHandler()
	// done like this to ensure rootSqalxNode is not nil at this point
	reportHandler = compute.NewReportHandler(statsHandler, rootSqalxNode, handleNewStatus)

	mainLog.Println("Database opened")

	compute.Initialize(rootSqalxNode, mainLog)

	fcmServerKey, present := secrets.Get("firebaseServerKey")
	if !present {
		mainLog.Fatalln("Firebase server key not present in keybox")
	}
	fcmcl = fcm.NewFcmClient(fcmServerKey)

	err = SetUpScrapers(rootSqalxNode)
	if err != nil {
		mainLog.Fatalln(err)
	}
	defer TearDownScrapers()

	facebookAccessToken, present := secrets.Get("facebookToken")
	if !present {
		mainLog.Fatalln("Facebook API access token not present in keybox")
	}

	SetUpAnnouncements(facebookAccessToken)
	defer TearDownAnnouncements()

	go StatsSender()
	go WebServer()
	go DiscordBot()

	certPath := DefaultClientCertPath
	if len(os.Args) > 1 {
		certPath = os.Args[1]
	}
	go APIserver(certPath)

	go func() {
		time.Sleep(5 * time.Second)
		for {
			err := compute.UpdateTypicalSeconds(rootSqalxNode, 10*time.Millisecond)
			if err != nil {
				mainLog.Println(err)
			}
			vehicleHandler.ClearTypicalSecondsCache()
			time.Sleep(10 * time.Hour)
		}
	}()

	if DEBUG {
		pair, err := dataobjects.NewPair(rootSqalxNode, "test", time.Now(), getHashKey())
		if err != nil {
			mainLog.Println(err)
		} else {
			mainLog.Println("Generated test API key", pair.Key, "with secret", pair.Secret)
		}
	}

	/*if DEBUG {
		f, err := os.Create("realTimeSimulationData.txt")
		if err == nil {
			toTime := time.Now()
			fromTime := toTime.AddDate(0, 0, -30)
			err = compute.SimulateRealtime(rootSqalxNode, fromTime, toTime, f)
			if err != nil {
				mainLog.Println(err)
			}
			f.Close()
			mainLog.Println("Real-time simulation data written")
		}
	}*/

	for {
		if DEBUG {
			printLatestDisturbance(rootSqalxNode)
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
	d, err := n.LastDisturbance(tx, false)
	if err == nil {
		mainLog.Println("Network last disturbance at", d.UStartTime, "description:", d.Description)
	} else {
		mainLog.Println(err)
	}
}
