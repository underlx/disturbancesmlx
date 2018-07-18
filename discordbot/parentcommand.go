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

// ReportDisturbanceCommand is sent when the bot wants to cast a disturbance vote
type ReportDisturbanceCommand struct {
	Line   *dataobjects.Line
	Weight int
}

// Command returns a pointer to itself
func (c *ReportDisturbanceCommand) Command() interface{} {
	return c
}

// ClearDisturbanceReportsCommand is sent when the bot wants to clear disturbance votes for a certain line
type ClearDisturbanceReportsCommand struct {
	Line *dataobjects.Line
}

// Command returns a pointer to itself
func (c *ClearDisturbanceReportsCommand) Command() interface{} {
	return c
}

// GetDisturbanceReportsCommand is sent when the bot wants to show current disturbance report status
type GetDisturbanceReportsCommand struct {
	MessageCallback func(message string)
}

// Command returns a pointer to itself
func (c *GetDisturbanceReportsCommand) Command() interface{} {
	return c
}

// ReportThresholdMultiplierCommand is sent when the bot wants to get or set the current threshold multiplier
type ReportThresholdMultiplierCommand struct {
	Set        bool
	Multiplier float32
}

// Command returns a pointer to itself
func (c *ReportThresholdMultiplierCommand) Command() interface{} {
	return c
}

// ReportThresholdOffsetCommand is sent when the bot wants to get or set the current threshold offset
type ReportThresholdOffsetCommand struct {
	Set    bool
	Offset int
}

// Command returns a pointer to itself
func (c *ReportThresholdOffsetCommand) Command() interface{} {
	return c
}

// RequestVersionCommand is sent when the bot wants to get the current server version
type RequestVersionCommand struct {
	GitCommit string
	BuildDate string
}

// Command returns a pointer to itself
func (c *RequestVersionCommand) Command() interface{} {
	return c
}

// RequestStatsCommand is sent when the bot wants to get the current server stats
type RequestStatsCommand struct {
	DBopenConnections int
	APItotalRequests  int
}

// Command returns a pointer to itself
func (c *RequestStatsCommand) Command() interface{} {
	return c
}
