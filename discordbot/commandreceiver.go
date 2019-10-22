package discordbot

import (
	"github.com/gbl08ma/ankiddie"
	"github.com/underlx/disturbancesmlx/types"
)

// CommandReceiver is used to send commands and exchange information with the code that set up a bot
type CommandReceiver interface {
	// NewLineStatus is called when the bot wants to add a new line status
	NewLineStatus(status *types.Status)

	// ControlScraper is called when the bot wants to start/stop/change a scraper
	ControlScraper(scraper string, enable bool, messageCallback func(message string))

	// ControlNotifs is caled when the bot wants to block/unblock sending of push notifications
	ControlNotifs(notifType string, enable bool)

	// CastDisturbanceVote is called when the bot wants to cast a disturbance vote
	CastDisturbanceVote(line *types.Line, weight int)

	// ClearDisturbanceVotes is called when the bot wants to clear disturbance votes
	ClearDisturbanceVotes(line *types.Line)

	// GetDisturbanceVotes is called when the bot wants to show current disturbance report status
	GetDisturbanceVotes(messageCallback func(message string))

	// GetThresholdMultiplier is called when the bot wants to know the current vote threshold multiplier
	GetThresholdMultiplier() float32

	// SetThresholdMultiplier is called when the bot wants to set the current vote threshold multiplier
	SetThresholdMultiplier(multiplier float32)

	// GetThresholdOffset is called when the bot wants to know the current vote threshold offset
	GetThresholdOffset() int

	// SetThresholdOffset is called when the bot wants to set the current vote threshold offset
	SetThresholdOffset(offset int)

	// GetVersion is called when the bot wants to get the current server version
	GetVersion() (gitCommit string, buildDate string)

	// GetStats is called when the bot wants to get the current server stats
	GetStats() (dbOpenConnections, apiTotalRequests int)

	// SendNotificationMetaBroadcast sends a FCM message containing a notification to show on some/all clients
	SendNotificationMetaBroadcast(shardID, shardMax int, versionFilter, localeFilter, title, body, url string)

	// SendCommandMetaBroadcast sends a FCM message containing a command to run on some/all clients
	SendCommandMetaBroadcast(shardID, shardMax int, versionFilter, localeFilter, command string, args ...string)

	// GetAnkiddie returns a reference to the global Ankiddie system
	GetAnkiddie() *ankiddie.Ankiddie

	// SetMQTTGatewayEnabled enables or disables the MQTT gateway
	SetMQTTGatewayEnabled(enabled bool) string

	// SendMQTTGatewayCommand sends a command to the MQTT subsystem
	SendMQTTGatewayCommand(command string, args ...string) string

	// SetAPIMOTDforLocale sets the "message of the day" of the API for the specified locale
	SetAPIMOTDforLocale(locale, html string)

	// SetAPIMOTDpriority sets the "message of the day" priority
	SetAPIMOTDpriority(priority int)

	// SetAPIMOTDmainLocale sets the "message of the day" main locale
	SetAPIMOTDmainLocale(mainLocale string)

	// ClearAPIMOTD clears the API MOTD
	ClearAPIMOTD()
}
