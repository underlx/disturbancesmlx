package main

import (
	"github.com/gbl08ma/disturbancesmlx/dataobjects"
	"github.com/gbl08ma/disturbancesmlx/scraper"
)

var annStore AnnouncementStore

// AnnouncementStore implements dataobjects.AnnouncementStore
type AnnouncementStore struct {
	scrapers map[string]scraper.AnnouncementScraper
}

// AddScraper registers all sources provided by this scraper
func (as *AnnouncementStore) AddScraper(scraper scraper.AnnouncementScraper) {
	for _, source := range scraper.Sources() {
		as.scrapers[source] = scraper
	}
}

// AllAnnouncements gets all announcements from all sources, unsorted
func (as *AnnouncementStore) AllAnnouncements() []*dataobjects.Announcement {
	ann := []*dataobjects.Announcement{}
	for source, scraper := range as.scrapers {
		ann = append(ann, scraper.Announcements(source)...)
	}
	return ann
}

// SourceAnnouncements gets all announcements from a specific source
func (as *AnnouncementStore) SourceAnnouncements(source string) []*dataobjects.Announcement {
	ann, ok := as.scrapers[source]
	if !ok {
		return []*dataobjects.Announcement{}
	}
	return ann.Announcements(source)
}
