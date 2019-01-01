package scraper

import (
	"log"

	"time"

	"github.com/gbl08ma/sqalx"
	"github.com/underlx/disturbancesmlx/dataobjects"
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
	Init(node sqalx.Node, log *log.Logger,
		statusReporter func(status *dataobjects.Status),
		conditionCallback func(condition *dataobjects.LineCondition))
	Networks() []*dataobjects.Network
	Lines() []*dataobjects.Line
	LastUpdate() time.Time
}

// AnnouncementScraper runs in the background retrieving announcements about a
// network.
type AnnouncementScraper interface {
	Scraper
	Init(log *log.Logger,
		newAnnouncementReporter func(announcement *dataobjects.Announcement))
	Networks() []*dataobjects.Network
	Sources() []string
	Announcements(source string) []*dataobjects.Announcement
}
