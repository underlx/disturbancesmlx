package scraper

import (
	"log"

	"time"

	"github.com/gbl08ma/sqalx"
	"github.com/underlx/disturbancesmlx/types"
)

// Scraper is something that fetches information
type Scraper interface {
	ID() string
	Begin()
	End()
	Running() bool
}

// StatusScraper is something that runs in the background retrieving status of lines
// StatusScrapers can report duplicate states to the statusReporter
type StatusScraper interface {
	Scraper
	Init(node sqalx.Node, log *log.Logger)
	Networks() []*types.Network
	Lines() []*types.Line
	LastUpdate() time.Time
}

// ETAScraper is something that runs in the background retrieving vehicle ETAs
type ETAScraper interface {
	Scraper
	Init(node sqalx.Node, log *log.Logger) error
}

// AnnouncementScraper runs in the background retrieving announcements about a
// network.
type AnnouncementScraper interface {
	Scraper
	Init(log *log.Logger,
		newAnnouncementReporter func(announcement *types.Announcement))
	Networks() []*types.Network
	Sources() []string
	Announcements(source string) []*types.Announcement
}
