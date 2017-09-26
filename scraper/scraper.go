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

// AnnouncementScraper runs in the background retrieving announcements about a
// network.
type AnnouncementScraper interface {
	Begin(log *log.Logger,
		newAnnouncementReporter func(announcement *dataobjects.Announcement),
		initialDataCallback func(announcements []*dataobjects.Announcement))
	End()
	Networks() []*dataobjects.Network
}
