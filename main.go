package main

import (
	"log"
	"os"
	"time"

	"github.com/underlx/disturbancesmlx/mqttgateway"

	"github.com/underlx/disturbancesmlx/ankiddie"

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
	mqttLog          = log.New(os.Stdout, "mqtt", log.Ldate|log.Ltime)
	lastChange       time.Time
	apiTotalRequests int

	kiddie            *ankiddie.Ankiddie
	vehicleHandler    *compute.VehicleHandler
	vehicleETAHandler *compute.VehicleETAHandler
	reportHandler     *compute.ReportHandler
	statsHandler      *compute.StatsHandler
	mqttGateway       *mqttgateway.MQTTGateway

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
	mainLog.Println("Database opened")

	kiddie = ankiddie.New(rootSqalxNode, ankoPackageConfigurator)
	err = kiddie.StartAutorun(0, false, defaultAnkoOut)
	if err != nil {
		mainLog.Fatalln(err)
	}

	statsHandler = compute.NewStatsHandler()
	vehicleHandler = compute.NewVehicleHandler()
	vehicleETAHandler = compute.NewVehicleETAHandler(rootSqalxNode)
	// done like this to ensure rootSqalxNode is not nil at this point
	reportHandler = compute.NewReportHandler(statsHandler, rootSqalxNode, handleNewStatus)

	compute.Initialize(rootSqalxNode, mainLog)

	fcmServerKey, present := secrets.Get("firebaseServerKey")
	if !present {
		mainLog.Fatalln("Firebase server key not present in keybox")
	}
	fcmcl = fcm.NewFcmClient(fcmServerKey)

	err = kiddie.StartAutorun(1, false, defaultAnkoOut)
	if err != nil {
		mainLog.Fatalln(err)
	}

	mqttKeybox, present := secrets.GetBox("mqtt")
	if !present {
		mainLog.Println("MQTT keybox not present in keybox, MQTT gateway will not be available")
	} else {
		mqttGateway, err = mqttgateway.New(mqttgateway.Config{
			Node:              rootSqalxNode,
			Log:               mqttLog,
			Keybox:            mqttKeybox,
			VehicleHandler:    vehicleHandler,
			VehicleETAHandler: vehicleETAHandler,
			StatsHandler:      statsHandler,
			AuthHashKey:       getHashKey(),
		})
		if err != nil {
			mainLog.Fatalln(err)
		}
	}

	go StatsSender()
	go WebServer()
	go DiscordBot()

	if mqttGateway != nil {
		err = mqttGateway.Start()
		if err != nil {
			mainLog.Fatalln(err)
		}
	}

	certPath := DefaultClientCertPath
	if len(os.Args) > 1 {
		certPath = os.Args[1]
	}
	go APIserver(certPath)

	mlAPItoken, _ := secrets.Get("pt-mlAPItoken")

	err = SetUpScrapers(rootSqalxNode, mlAPItoken)
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

	err = kiddie.StartAutorun(2, true, defaultAnkoOut)
	if err != nil {
		mainLog.Fatalln(err)
	}

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

	/*if DEBUG {
		loc, _ := time.LoadLocation("Europe/Lisbon")
		start := time.Date(2017, time.May, 1, 0, 0, 0, 0, loc)
		end := time.Date(2018, time.October, 14, 0, 0, 0, 0, loc)
		points, err := compute.TripsScatterplotNumTripsVsAvgSpeed(rootSqalxNode, start, end, 12)
		if err != nil {
			mainLog.Fatalln(err)
		}
		f, err := os.Create("scatterplot.csv")
		if err != nil {
			mainLog.Fatalln(err)
		}
		f.WriteString("dow,hour,numusers,avgspeed\n")
		for _, point := range points {
			f.WriteString(fmt.Sprintf("%d,%d,%d,%.0f\n", point.DayOfWeek, point.Hour, point.NumUsers, point.AverageSpeed))
		}
		f.Close()

		f, err = os.Create("scatterplot2.csv")
		if err != nil {
			mainLog.Fatalln(err)
		}
		tripsPerDow := make(map[int]map[int]map[int]map[float64]int)
		//tripsPerHour := make(map[int]map[int]map[float64]int)
		//tripsPerNumUsers := make(map[int]map[float64]int)
		//tripsPerAvgSpeed := make(map[float64]int)
		for _, point := range points {
			if tripsPerDow[int(point.DayOfWeek)] == nil {
				tripsPerDow[int(point.DayOfWeek)] = make(map[int]map[int]map[float64]int)
			}
			if tripsPerDow[int(point.DayOfWeek)][point.Hour] == nil {
				tripsPerDow[int(point.DayOfWeek)][point.Hour] = make(map[int]map[float64]int)
			}
			if tripsPerDow[int(point.DayOfWeek)][point.Hour][point.NumUsers] == nil {
				tripsPerDow[int(point.DayOfWeek)][point.Hour][point.NumUsers] = make(map[float64]int)
			}
			tripsPerDow[int(point.DayOfWeek)][point.Hour][point.NumUsers][point.AverageSpeed] = tripsPerDow[int(point.DayOfWeek)][point.Hour][point.NumUsers][point.AverageSpeed] + 1
		}
		f.WriteString("dow,hour,numusers,avgspeed,numtrips\n")
		for dow, hourMap := range tripsPerDow {
			for hour, numUserMap := range hourMap {
				for numUser, avgSpeedMap := range numUserMap {
					for avgSpeed, value := range avgSpeedMap {
						f.WriteString(fmt.Sprintf("%d,%d,%d,%f,%d\n", dow, hour, numUser, avgSpeed, value))
					}
				}
			}
		}

		f.Close()
	}*/

	/*if DEBUG {
		loc, _ := time.LoadLocation("Europe/Lisbon")
		start := time.Date(2017, time.May, 1, 0, 0, 0, 0, loc)
		end := time.Date(2018, time.October, 14, 0, 0, 0, 0, loc)
		cEntries, tEntries, cMinMax, tMinMax, err := compute.TypicalSecondsByDowAndHour(rootSqalxNode, start, end)
		if err != nil {
			mainLog.Fatalln(err)
		}
		f, err := os.Create("connectionseconds.csv")
		if err != nil {
			mainLog.Fatalln(err)
		}
		f.WriteString("from,to,dow,hour,numerator,denominator\n")
		for _, entry := range cEntries {
			f.WriteString(fmt.Sprintf("%s,%s,%d,%d,%.0f,%d\n", entry.From, entry.To, entry.Weekday, entry.Hour, entry.Numerator, entry.Denominator))
		}
		f.Close()

		f, err = os.Create("transferseconds.csv")
		if err != nil {
			mainLog.Fatalln(err)
		}
		f.WriteString("from,to,dow,hour,numerator,denominator\n")
		for _, entry := range tEntries {
			f.WriteString(fmt.Sprintf("%s,%s,%d,%d,%.0f,%d\n", entry.From, entry.To, entry.Weekday, entry.Hour, entry.Numerator, entry.Denominator))
		}
		f.Close()

		f, err = os.Create("connectionminmax.csv")
		if err != nil {
			mainLog.Fatalln(err)
		}
		f.WriteString("from,to,min,max\n")
		for _, entry := range cMinMax {
			f.WriteString(fmt.Sprintf("%s,%s,%d,%d\n", entry.From, entry.To, entry.Min, entry.Max))
		}
		f.Close()

		f, err = os.Create("transferminmax.csv")
		if err != nil {
			mainLog.Fatalln(err)
		}
		f.WriteString("from,to,min,max\n")
		for _, entry := range tMinMax {
			f.WriteString(fmt.Sprintf("%s,%s,%d,%d\n", entry.From, entry.To, entry.Min, entry.Max))
		}
		f.Close()
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
