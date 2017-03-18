package interfaces

import (
	"time"
)

// Network is a transportation network
type Network struct {
	ID   string
	Name string
}

// Line is a Network line
type Line struct {
	ID      string
	Name    string
	Network Network
}

// Status represents the status of a Line at a certain point in time
type Status struct {
	Time          time.Time
	Line          Line
	IsDisturbance bool
	Status        string
	Source        Source
}

// Source represents a Status source
type Source struct {
	ID          string
	Name        string
	IsAutomatic bool
}
