package main

import (
	"log"
	"os"
	"time"

	"tny.im/disturbancesmlx/interfaces"
	"tny.im/disturbancesmlx/scraper"
	"tny.im/disturbancesmlx/scraper/mlxscraper"

	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/postgres"
)

var db *gorm.DB

func getOngoingDisturbance(line interfaces.Line) (disturbance interfaces.Disturbance, found bool) {
	var count int
	db.Where(&interfaces.Disturbance{
		Line:  line,
		Ended: false,
	}).First(&disturbance).Count(&count)
	return disturbance, count != 0
}

func main() {
	var err error
	db, err = gorm.Open("postgres", "postgresql://www-data:xxx@localhost:5432?sslmode=disable&dbname=disturbances")
	if err != nil {
		panic("failed to connect to database")
	}
	defer db.Close()
	db.LogMode(true)
	db.AutoMigrate(&interfaces.Network{}, &interfaces.Line{}, &interfaces.Status{}, &interfaces.Source{}, &interfaces.Disturbance{})

	l := log.New(os.Stdout, "mlxscraper", log.Ldate|log.Ltime)
	var s scraper.Scraper
	s = &mlxscraper.Scraper{}
	s.Begin(l, 10*time.Second, func(status interfaces.Status) {
		log.Println("New status for line", status.Line.Name, "on network", status.Line.Network.Name)
		log.Println("  ", status.Status)
		if status.IsDowntime {
			log.Println("   Is disturbance!")
		}

		disturbance, found := getOngoingDisturbance(status.Line)
		if found {
			log.Println("Found ongoing disturbance")
		}
		if status.IsDowntime && !found {
			disturbance.StartTime = status.Time
			disturbance.Line = status.Line
			disturbance.Description = status.Status
		} else if !status.IsDowntime && found {
			disturbance.EndTime = status.Time
			disturbance.Ended = true
		}
		disturbance.Statuses = append(disturbance.Statuses, status)
		if found {
			db.Update(&disturbance)
		} else if status.IsDowntime {
			db.Create(&disturbance)
		}
	})

	time.Sleep(1 * time.Minute)
	s.End()
}
