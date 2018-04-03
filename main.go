package main

import (
	"log"
	"os"
	"time"

	fcm "github.com/NaySoftware/go-fcm"
	"github.com/heetch/sqalx"
	"github.com/jmoiron/sqlx"

	sq "github.com/gbl08ma/squirrel"

	"github.com/gbl08ma/keybox"
	"github.com/underlx/disturbancesmlx/dataobjects"
)

var (
	rdb           *sqlx.DB
	sdb           sq.StatementBuilderType
	rootSqalxNode sqalx.Node
	secrets       *keybox.Keybox
	fcmcl         *fcm.FcmClient
	mainLog       = log.New(os.Stdout, "", log.Ldate|log.Ltime)
	discordLog    = log.New(os.Stdout, "discord", log.Ldate|log.Ltime)
	webLog        = log.New(os.Stdout, "web", log.Ldate|log.Ltime)
	lastChange    time.Time
)

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
	rdb.SetMaxOpenConns(MaxDBconnectionPoolSize)
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
			err := ComputeTypicalSeconds(rootSqalxNode, 50*time.Millisecond)
			if err != nil {
				mainLog.Println(err)
			}
			time.Sleep(10 * time.Hour)
		}
	}()

	if DEBUG {
		f, err := os.Create("realTimeSimulationData.txt")
		if err == nil {
			toTime := time.Now()
			fromTime := toTime.AddDate(0, 0, -30)
			err = ComputeSimulatedRealtime(rootSqalxNode, fromTime, toTime, f)
			if err != nil {
				mainLog.Println(err)
			}
			f.Close()
			mainLog.Println("Real-time simulation data written")
		}
	}

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
