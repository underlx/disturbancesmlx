package scraper

import (
	"log"

	"tny.im/disturbancesmlx/interfaces"
)

// Scraper is something that runs in the background retrieving status of lines
// Scrapers can report duplicate states to the statusReporter
type Scraper interface {
	Begin(log *log.Logger,
		statusReporter func(status *interfaces.Status),
		topologyChangeCallback func(Scraper))
	End()
	Networks() []*interfaces.Network
	Lines() []*interfaces.Line
}
