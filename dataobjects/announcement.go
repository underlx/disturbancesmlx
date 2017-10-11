package dataobjects

import (
	"time"
)

// Announcement contains an announcement for a network
type Announcement struct {
	Time    time.Time
	Network *Network
	Title   string
	Body    string
	URL     string
	Source  string
}

// AnnouncementStore manages announcements for one or more networks
type AnnouncementStore interface {
	AllAnnouncements() []*Announcement
	SourceAnnouncements(source string) []*Announcement
}
