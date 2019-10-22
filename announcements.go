package main

import (
	"github.com/underlx/disturbancesmlx/types"
	"github.com/underlx/disturbancesmlx/scraper"
)

var annStore AnnouncementStore

// AnnouncementStore implements types.AnnouncementStore
type AnnouncementStore struct {
	scrapers map[string]scraper.AnnouncementScraper
}

// AddScraper registers all sources provided by this scraper
func (as *AnnouncementStore) AddScraper(s scraper.AnnouncementScraper) {
	if as.scrapers == nil {
		as.scrapers = make(map[string]scraper.AnnouncementScraper)
	}
	for _, source := range s.Sources() {
		as.scrapers[source] = s
	}
}

// AllAnnouncements gets all announcements from all sources, unsorted
func (as *AnnouncementStore) AllAnnouncements() []*types.Announcement {
	ann := []*types.Announcement{}
	for source, scraper := range as.scrapers {
		ann = append(ann, scraper.Announcements(source)...)
	}
	return ann
}

// SourceAnnouncements gets all announcements from a specific source
func (as *AnnouncementStore) SourceAnnouncements(source string) []*types.Announcement {
	ann, ok := as.scrapers[source]
	if !ok {
		return []*types.Announcement{}
	}
	return ann.Announcements(source)
}
