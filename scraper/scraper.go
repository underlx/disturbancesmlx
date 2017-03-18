package scraper

import (
	"log"

	"tny.im/disturbancesmlx/interfaces"
)
import "time"

// Scraper is something that runs in the background retrieving status of lines
// Scrapers can report duplicate states to the statusReporter
type Scraper interface {
	Begin(log *log.Logger, period time.Duration, statusReporter func(status interfaces.Status))
	End()
	Networks() []interfaces.Network
	Lines() []interfaces.Line
}
