package main

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/gbl08ma/sqalx"
	uuid "github.com/satori/go.uuid"
	"github.com/underlx/disturbancesmlx/dataobjects"

	"github.com/underlx/disturbancesmlx/discordbot"
)

// DiscordBot starts the Discord bot if it is enabled in the settings
func DiscordBot() {
	discordBox, present := secrets.GetBox("discord")
	if !present {
		discordLog.Println("Discord Keybox not found, Discord functions disabled")
		return
	}

	webKeybox, present := secrets.GetBox("web")
	if !present {
		discordLog.Fatal("Web keybox not present in keybox")
	}

	url, present := webKeybox.Get("websiteURL")
	if !present {
		discordLog.Fatal("Website URL not present in keybox")
	}

	err := discordbot.Start(rootSqalxNode, url, discordBox, discordLog,
		new(BotCommandReceiver))
	if err != nil {
		discordLog.Println(err)
		return
	}

	// Wait here until CTRL-C or other term signal is received.
	discordLog.Println("Bot is now running.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc

	// Cleanly close down the Discord session.
	discordbot.Stop()

	os.Exit(0)
}

// BotCommandReceiver implements discordbot.CommandReceiver
type BotCommandReceiver struct{}

// NewLineStatus is called when the bot wants to add a new line status
func (r *BotCommandReceiver) NewLineStatus(status *dataobjects.Status) {
	handleNewStatusNotify(status)
}

// ControlScraper is called when the bot wants to start/stop/change a scraper
func (r *BotCommandReceiver) ControlScraper(scraper string, enable bool, messageCallback func(message string)) {
	handleControlScraper(scraper, enable, messageCallback)
}

// ControlNotifs is caled when the bot wants to block/unblock sending of push notifications
func (r *BotCommandReceiver) ControlNotifs(notifType string, enable bool) {
	handleControlNotifs(notifType, enable)
}

// CastDisturbanceVote is called when the bot wants to cast a disturbance vote
func (r *BotCommandReceiver) CastDisturbanceVote(line *dataobjects.Line, weight int) {
	err := reportHandler.addReport(dataobjects.NewLineDisturbanceReportDebug(line, "discord"), weight)
	if err != nil {
		discordLog.Println(err)
	}
}

// ClearDisturbanceVotes is called when the bot wants to clear disturbance votes
func (r *BotCommandReceiver) ClearDisturbanceVotes(line *dataobjects.Line) {
	reportHandler.clearVotesForLine(line)
}

// GetDisturbanceVotes is called when the bot wants to show current disturbance report status
func (r *BotCommandReceiver) GetDisturbanceVotes(messageCallback func(message string)) {
	message := ""
	lines, err := dataobjects.GetLines(rootSqalxNode)
	if err != nil {
		discordLog.Println(err)
	}
	for _, line := range lines {
		message += fmt.Sprintf("`%s`: %d/%d\n", line.ID, reportHandler.countVotesForLine(line), reportHandler.getThresholdForLine(line))
	}
	messageCallback(message)
}

// GetThresholdMultiplier is called when the bot wants to know the current vote threshold multiplier
func (r *BotCommandReceiver) GetThresholdMultiplier() float32 {
	return reportHandler.ThresholdMultiplier()
}

// SetThresholdMultiplier is called when the bot wants to set the current vote threshold multiplier
func (r *BotCommandReceiver) SetThresholdMultiplier(multiplier float32) {
	reportHandler.SetThresholdMultiplier(multiplier)
}

// GetThresholdOffset is called when the bot wants to know the current vote threshold offset
func (r *BotCommandReceiver) GetThresholdOffset() int {
	return reportHandler.ThresholdOffset()
}

// SetThresholdOffset is called when the bot wants to set the current vote threshold offset
func (r *BotCommandReceiver) SetThresholdOffset(offset int) {
	reportHandler.SetThresholdOffset(offset)
}

// GetVersion is called when the bot wants to get the current server version
func (r *BotCommandReceiver) GetVersion() (gitCommit string, buildDate string) {
	return GitCommit, BuildDate
}

// GetStats is called when the bot wants to get the current server stats
func (r *BotCommandReceiver) GetStats() (dbOpenConnections, apiTR int) {
	return rdb.Stats().OpenConnections, apiTotalRequests
}

// SchedulesToLines turns the provided schedule array into a human-readable list of strings
func (r *BotCommandReceiver) SchedulesToLines(schedules []*dataobjects.LobbySchedule) []string {
	return schedulesToLines(schedules)
}

// SendNotificationMetaBroadcast sends a FCM message containing a notification to show on some/all clients
func (r *BotCommandReceiver) SendNotificationMetaBroadcast(versionFilter, localeFilter, title, body, url string) {
	id, err := uuid.NewV4()
	if err != nil {
		return
	}

	SendMetaBroadcast(id.String(), "notification", versionFilter, localeFilter,
		[2]string{"title", title},
		[2]string{"body", body},
		[2]string{"url", url})
}

// SendCommandMetaBroadcast sends a FCM message containing a command to run on some/all clients
func (r *BotCommandReceiver) SendCommandMetaBroadcast(versionFilter, localeFilter, command string, args ...string) {
	id, err := uuid.NewV4()
	if err != nil {
		return
	}

	SendMetaBroadcast(id.String(), "command", versionFilter, localeFilter,
		[2]string{"command", command},
		[2]string{"args", strings.Join(args, "|")})
}

// ConfigureAnkoPackage asks the bot host to set up the package for the anko script system
func (r *BotCommandReceiver) ConfigureAnkoPackage(pkg, packageTypes map[string]interface{}) {
	pkg["RootSqalxNode"] = func() sqalx.Node {
		return rootSqalxNode
	}
	pkg["ComputeAverageSpeed"] = ComputeAverageSpeed
	pkg["ComputeAverageSpeedFilter"] = ComputeAverageSpeedFilter
	pkg["ComputeAverageSpeedCached"] = ComputeAverageSpeedCached
}
