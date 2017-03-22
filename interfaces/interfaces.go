package interfaces

import (
	"time"

	"github.com/jinzhu/gorm"
)

// Network is a transportation network
type Network struct {
	gorm.Model
	ID   string `gorm:"type:varchar(64);primary_key"`
	Name string
}

// Line is a Network line
type Line struct {
	gorm.Model
	ID      string `gorm:"type:varchar(32);primary_key"`
	Name    string
	Network Network
}

// Status represents the status of a Line at a certain point in time
type Status struct {
	gorm.Model
	Time       time.Time `gorm:"primary_key"`
	Line       Line      `gorm:"primary_key"`
	IsDowntime bool
	Status     string
	Source     Source
}

// Source represents a Status source
type Source struct {
	gorm.Model
	ID          string `gorm:"type:varchar(32);primary_key"`
	Name        string
	IsAutomatic bool
}

// Disturbance represents a disturbance
type Disturbance struct {
	gorm.Model
	ID          int64 `gorm:"primary_key"`
	StartTime   time.Time
	EndTime     time.Time
	Ended       bool
	Line        Line
	Description string `gorm:"type:text"`
	Statuses    []Status
}
