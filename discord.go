package main

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/underlx/disturbancesmlx/compute"

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
	err := reportHandler.AddReportManually(dataobjects.NewLineDisturbanceReportDebug(line, "discord"), weight)
	if err != nil {
		discordLog.Println(err)
	}
}

// ClearDisturbanceVotes is called when the bot wants to clear disturbance votes
func (r *BotCommandReceiver) ClearDisturbanceVotes(line *dataobjects.Line) {
	reportHandler.ClearVotesForLine(line)
}

// GetDisturbanceVotes is called when the bot wants to show current disturbance report status
func (r *BotCommandReceiver) GetDisturbanceVotes(messageCallback func(message string)) {
	message := ""
	lines, err := dataobjects.GetLines(rootSqalxNode)
	if err != nil {
		discordLog.Println(err)
	}
	for _, line := range lines {
		message += fmt.Sprintf("`%s`: %d/%d\n", line.ID, reportHandler.CountVotesForLine(line), reportHandler.GetThresholdForLine(line))
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
func (r *BotCommandReceiver) ConfigureAnkoPackage(packages, packageTypes map[string]map[string]interface{}) {
	packages["underlx"] = make(map[string]interface{})
	packages["underlx"]["RootSqalxNode"] = func() sqalx.Node {
		return rootSqalxNode
	}
	packages["underlx"]["VehicleHandler"] = func() *compute.VehicleHandler {
		return vehicleHandler
	}
	packages["underlx"]["StatsHandler"] = func() *compute.StatsHandler {
		return statsHandler
	}
	packages["underlx"]["ReportHandler"] = func() *compute.ReportHandler {
		return reportHandler
	}

	packages["compute"] = make(map[string]interface{})
	packages["compute"]["AverageSpeed"] = compute.AverageSpeed
	packages["compute"]["AverageSpeedFilter"] = compute.AverageSpeedFilter
	packages["compute"]["AverageSpeedCached"] = compute.AverageSpeedCached
	packages["compute"]["UpdateTypicalSeconds"] = compute.UpdateTypicalSeconds
	packages["compute"]["UpdateStatusMsgTypes"] = compute.UpdateStatusMsgTypes

	packages["dataobjects"] = make(map[string]interface{})
	dopkg := packages["dataobjects"]
	for name, function := range dataobjects.Functions {
		if function.CanInterface() {
			dopkg[name] = function.Interface()
		}
	}
	for name, item := range dataobjects.Consts {
		dopkg[name] = item
	}
	for name, item := range dataobjects.Variables {
		dopkg[name] = item
	}
	packageTypes["dataobjects"] = make(map[string]interface{})
	dotypes := packageTypes["dataobjects"]
	for name, item := range dataobjects.Types {
		dotypes[name] = item
	}

	packages["uuid"] = make(map[string]interface{})
	packages["uuid"]["V1"] = uuid.V1
	packages["uuid"]["V2"] = uuid.V2
	packages["uuid"]["V3"] = uuid.V3
	packages["uuid"]["V4"] = uuid.V4
	packages["uuid"]["V5"] = uuid.V5
	packages["uuid"]["VariantNCS"] = uuid.VariantNCS
	packages["uuid"]["VariantRFC4122"] = uuid.VariantRFC4122
	packages["uuid"]["VariantMicrosoft"] = uuid.VariantMicrosoft
	packages["uuid"]["VariantFuture"] = uuid.VariantFuture
	packages["uuid"]["DomainGroup"] = uuid.DomainGroup
	packages["uuid"]["DomainOrg"] = uuid.DomainOrg
	packages["uuid"]["DomainPerson"] = uuid.DomainPerson
	packages["uuid"]["Size"] = uuid.Size
	packages["uuid"]["NamespaceDNS"] = uuid.NamespaceDNS
	packages["uuid"]["NamespaceOID"] = uuid.NamespaceOID
	packages["uuid"]["NamespaceURL"] = uuid.NamespaceURL
	packages["uuid"]["NamespaceX500"] = uuid.NamespaceX500
	packages["uuid"]["Nil"] = uuid.Nil
	packages["uuid"]["Equal"] = uuid.Equal
	packages["uuid"]["FromBytes"] = uuid.FromBytes
	packages["uuid"]["FromBytesOrNil"] = uuid.FromBytesOrNil
	packages["uuid"]["FromString"] = uuid.FromString
	packages["uuid"]["FromStringOrNil"] = uuid.FromStringOrNil
	packages["uuid"]["Must"] = uuid.Must
	packages["uuid"]["NewV1"] = uuid.NewV1
	packages["uuid"]["NewV2"] = uuid.NewV2
	packages["uuid"]["NewV3"] = uuid.NewV3
	packages["uuid"]["NewV4"] = uuid.NewV4
	packages["uuid"]["NewV5"] = uuid.NewV5
	packageTypes["uuid"] = make(map[string]interface{})
	packageTypes["uuid"]["NullUUID"] = uuid.NullUUID{}
	packageTypes["uuid"]["UUID"] = uuid.UUID{}
}
