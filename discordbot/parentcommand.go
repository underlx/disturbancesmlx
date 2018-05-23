package discordbot

import "github.com/underlx/disturbancesmlx/dataobjects"

// ParentCommand represents a command that is sent to the bot parent
type ParentCommand interface {
	Command() interface{}
}

// NewStatusCommand is sent when the bot wants to add a new line status
type NewStatusCommand struct {
	Status *dataobjects.Status
}

// Command returns a pointer to itself
func (c *NewStatusCommand) Command() interface{} {
	return c
}

// ControlScraperCommand is sent when the bot wants to start/stop/change a scraper
type ControlScraperCommand struct {
	Scraper         string
	Enable          bool
	MessageCallback func(message string)
}

// Command returns a pointer to itself
func (c *ControlScraperCommand) Command() interface{} {
	return c
}

// ControlNotifsCommand is sent when the bot wants to block/unblock sending of push notifications
type ControlNotifsCommand struct {
	Type   string
	Enable bool
}

// Command returns a pointer to itself
func (c *ControlNotifsCommand) Command() interface{} {
	return c
}
