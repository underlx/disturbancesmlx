package main

import (
	"log"
	"os"
	"time"

	"github.com/heetch/sqalx"
	"github.com/jmoiron/sqlx"
	uuid "github.com/satori/go.uuid"

	sq "github.com/gbl08ma/squirrel"

	"github.com/gbl08ma/keybox"
	"tny.im/disturbancesmlx/interfaces"
	"tny.im/disturbancesmlx/scraper"
	"tny.im/disturbancesmlx/scraper/mlxscraper"
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
	mainLog       = log.New(os.Stdout, "", log.Ldate|log.Ltime)
	webLog        = log.New(os.Stdout, "web", log.Ldate|log.Ltime)
	mlxscr        scraper.Scraper
	lastChange    time.Time
)

// Return the time since the last Metro de Lisboa disturbance in hours and days
func MLNoDisturbanceUptime(node sqalx.Node) (hours int, days int, err error) {
	tx, err := node.Beginx()
	if err != nil {
		return 0, 0, err
	}
	defer tx.Commit() // read-only tx

	n, err := interfaces.GetNetwork(rootSqalxNode, MLnetworkID)
	if err != nil {
		return 0, 0, err
	}
	d, err := n.LastDisturbance(rootSqalxNode)
	if err != nil {
		return 0, 0, err
	}

	if !d.Ended {
		return 0, 0, nil
	}

	difference := time.Now().UTC().Sub(d.EndTime)
	return int(difference.Hours()), int(difference.Hours() / 24), nil
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

	l := log.New(os.Stdout, "mlxscraper", log.Ldate|log.Ltime)
	mlxscr = &mlxscraper.Scraper{
		URL:         "http://app.metrolisboa.pt/status/estado_Linhas.php",
		NetworkID:   MLnetworkID,
		NetworkName: "Metro de Lisboa",
		Source: &interfaces.Source{
			ID:          "mlxscraper-pt-ml",
			Name:        "Metro de Lisboa estado_Linhas.php",
			IsAutomatic: true,
		},
		Period: 1 * time.Minute,
	}
	mlxscr.Begin(l, func(status *interfaces.Status) {
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
		var disturbance *interfaces.Disturbance
		if found {
			mainLog.Println("   Found ongoing disturbance")
			disturbance = disturbances[len(disturbances)-1]
		} else {
			disturbance = &interfaces.Disturbance{
				ID:   uuid.NewV4().String(),
				Line: status.Line,
			}
		}
		if status.IsDowntime && !found {
			disturbance.StartTime = status.Time
			disturbance.Description = status.Status
			disturbance.Statuses = append(disturbance.Statuses, status)
			err = disturbance.Update(tx)
			if err != nil {
				mainLog.Println(err)
				return
			}
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
		}
		lastChange = time.Now().UTC()
		tx.Commit()
	},
		func(s scraper.Scraper) {
			for _, network := range s.Networks() {
				network.Update(rootSqalxNode)
			}
			for _, line := range s.Lines() {
				line.Update(rootSqalxNode)
			}
		})
	defer mlxscr.End()

	go WebServer()
	for {
		if DEBUG {
			printLatestDisturbance(rootSqalxNode)
			hours, days, err := MLNoDisturbanceUptime(rootSqalxNode)
			if err != nil {
				mainLog.Println(err)
			}
			mainLog.Printf("Since last disturbance: %d hours or %d days", hours, days)
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

	n, err := interfaces.GetNetwork(tx, MLnetworkID)
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
