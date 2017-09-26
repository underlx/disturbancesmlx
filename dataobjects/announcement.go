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
