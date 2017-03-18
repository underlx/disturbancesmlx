package main

import (
	"log"
	"os"
	"time"

	"tny.im/disturbancesmlx/interfaces"
	"tny.im/disturbancesmlx/scraper"
	"tny.im/disturbancesmlx/scraper/mlxscraper"
)

func main() {
	l := log.New(os.Stdout, "mlxscraper", log.Ldate|log.Ltime)
	var s scraper.Scraper
	s = &mlxscraper.Scraper{}
	s.Begin(l, 10*time.Second, func(status interfaces.Status) {
		log.Println("New status for line", status.Line.Name, "on network", status.Line.Network.Name)
		log.Println("  ", status.Status)
		if status.IsDisturbance {
			log.Println("   Is disturbance!")
		}
	})

	time.Sleep(1 * time.Minute)
	s.End()
}
