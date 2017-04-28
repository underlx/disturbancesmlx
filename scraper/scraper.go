package scraper

import (
	"log"

	"time"

	"github.com/gbl08ma/disturbancesmlx/dataobjects"
)

// Scraper is something that runs in the background retrieving status of lines
// Scrapers can report duplicate states to the statusReporter
type Scraper interface {
	Begin(log *log.Logger,
		statusReporter func(status *dataobjects.Status),
		topologyChangeCallback func(Scraper))
	End()
	Networks() []*dataobjects.Network
	Lines() []*dataobjects.Line
	LastUpdate() time.Time
}
