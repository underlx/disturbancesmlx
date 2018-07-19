package discordbot

import (
	"errors"
	"fmt"
	"log"
	"math"
	"math/rand"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gbl08ma/keybox"

	"github.com/bwmarrin/discordgo"
	"github.com/heetch/sqalx"
	uuid "github.com/satori/go.uuid"
	"github.com/underlx/disturbancesmlx/dataobjects"
)

var stopMute map[string]time.Time // maps channel IDs to the time when the bot can talk again
var channelMute map[string]bool
var tempMessages sync.Map

var commandLib *CommandLibrary
var messageHandlers []MessageHandler
var guildIDs sync.Map
var botstats stats

type stats struct {
	startTime           time.Time
	userCount           int
	botCount            int
	dmChannelCount      int
	groupDMChannelCount int
	textChannelCount    int
	voiceChannelCount   int
}

var node sqalx.Node
var websiteURL string
var botLog *log.Logger
var session *discordgo.Session
var schedToLines func(schedules []*dataobjects.LobbySchedule) []string
var cmdCallback func(command ParentCommand)

// Start starts the Discord bot
func Start(snode sqalx.Node, swebsiteURL string, keybox *keybox.Keybox,
	log *log.Logger,
	schedulesToLines func(schedules []*dataobjects.LobbySchedule) []string,
	cmdCb func(command ParentCommand)) error {
	node = snode
	websiteURL = swebsiteURL
	botLog = log
	schedToLines = schedulesToLines
	cmdCallback = cmdCb
	channelMute = make(map[string]bool)
	rand.Seed(time.Now().Unix())

	discordToken, present := keybox.Get("token")
	if !present {
		return errors.New("Discord bot token not present in keybox")
	}

	adminChannelID, _ := keybox.Get("adminChannel")
	if !present {
		adminChannelID = ""
	}

	dg, err := discordgo.New("Bot " + discordToken)
	if err != nil {
		return err
	}
	session = dg

	selfApp, err := dg.Application("@me")
	if err != nil {
		return err
	}

	commandLib = NewCommandLibrary("$", selfApp.Owner.ID).WithAdminChannel(adminChannelID)
	messageHandlers = append(messageHandlers, commandLib)
	commandLib.Register(NewCommand("ping", func(s *discordgo.Session, m *discordgo.MessageCreate, args []string) {
		embed := NewEmbed()
		addMuteEmbed(embed, m.ChannelID)
		msg := &discordgo.MessageSend{
			Content: "🙌",
		}
		if len(embed.Fields) > 0 {
			msg.Embed = embed.MessageEmbed
		}
		s.ChannelMessageSendComplex(m.ChannelID, msg)
	}))
	commandLib.Register(NewCommand("stats", func(s *discordgo.Session, m *discordgo.MessageCreate, args []string) {
		embed, err := buildBotStatsMessage(m)
		if err == nil {
			s.ChannelMessageSendEmbed(m.ChannelID, embed.MessageEmbed)
		}
		if len(args) > 0 && args[0] == "full" &&
			(commandLib.isAdminChannel(m.ChannelID) || m.Author.ID == selfApp.Owner.ID) {
			embed, err = buildStatsMessage()
			if err == nil {
				s.ChannelMessageSendEmbed(m.ChannelID, embed.MessageEmbed)
			}
		}
	}))
	commandLib.Register(NewCommand("about", func(s *discordgo.Session, m *discordgo.MessageCreate, args []string) {
		embed, err := buildAboutMessage(m)
		if err == nil {
			s.ChannelMessageSendEmbed(m.ChannelID, embed.MessageEmbed)
		}
	}))
	commandLib.Register(NewCommand("help", func(s *discordgo.Session, m *discordgo.MessageCreate, args []string) {
		showAll := len(args) > 0 && args[0] == "full" &&
			(commandLib.isAdminChannel(m.ChannelID) || m.Author.ID == selfApp.Owner.ID)
		msg := "**Comandos suportados**\n"
		for _, command := range commandLib.commands {
			if command.RequirePrivilege == PrivilegeEveryone || showAll {
				msg += commandLib.prefix + command.Name + "\n"
			}
		}
		if commandLib.isAdminChannel(m.ChannelID) && !showAll {
			msg += "_(`$help full` para ver os comandos todos)_"
		}
		s.ChannelMessageSend(m.ChannelID, msg)
	}))
	commandLib.Register(NewCommand("mute", func(s *discordgo.Session, m *discordgo.MessageCreate, args []string) {
		muteDuration := 15 * time.Minute
		if len(args) > 0 {
			if duration, err := time.ParseDuration(args[0]); err == nil {
				muteDuration = duration
			} else if mins, err := strconv.ParseUint(args[0], 10, 32); err == nil {
				muteDuration = time.Duration(mins) * time.Minute
			}
		}
		stopMute[m.ChannelID] = time.Now().Add(muteDuration)
		if muteDuration.Minutes() < 60.0 {
			s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("🤐 por %d minutos", int(math.Round(muteDuration.Minutes()))))
		} else {
			s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("🤐 por %s", muteDuration.String()))
		}
	}))
	commandLib.Register(NewCommand("unmute", func(s *discordgo.Session, m *discordgo.MessageCreate, args []string) {
		stopMute[m.ChannelID] = time.Time{}
		s.ChannelMessageSend(m.ChannelID, "🤗")
	}))
	commandLib.Register(NewCommand("permamute", func(s *discordgo.Session, m *discordgo.MessageCreate, args []string) {
		channelMute[m.ChannelID] = true
		s.ChannelMessageSend(m.ChannelID, "🤐💀")
	}).WithRequirePrivilege(PrivilegeAdmin))
	commandLib.Register(NewCommand("permaunmute", func(s *discordgo.Session, m *discordgo.MessageCreate, args []string) {
		channelMute[m.ChannelID] = false
		s.ChannelMessageSend(m.ChannelID, "🤗🙌")
	}).WithRequirePrivilege(PrivilegeAdmin))
	commandLib.Register(NewCommand("setstatus", handleStatus).WithRequirePrivilege(PrivilegeAdmin))
	commandLib.Register(NewCommand("addlinestatus", handleLineStatus).WithRequirePrivilege(PrivilegeAdmin))
	commandLib.Register(NewCommand("scraper", handleControlScraper).WithRequirePrivilege(PrivilegeAdmin))
	commandLib.Register(NewCommand("notifs", handleControlNotifs).WithRequirePrivilege(PrivilegeAdmin))
	commandLib.Register(NewCommand("russia", handleRUSSIA).WithRequirePrivilege(PrivilegeAdmin))
	commandLib.Register(NewCommand("setprefix", func(s *discordgo.Session, m *discordgo.MessageCreate, args []string) {
		if len(args) == 0 {
			commandLib.SetPrefix("")
		} else {
			commandLib.SetPrefix(args[0])
		}
		s.ChannelMessageSend(m.ChannelID, "✅")
	}).WithRequirePrivilege(PrivilegeRoot))

	infoHandler, err := NewInfoHandler(node)
	if err != nil {
		return err
	}
	messageHandlers = append(messageHandlers, infoHandler)

	stopMute = make(map[string]time.Time)

	user, err := dg.User("@me")
	if err != nil {
		return err
	}
	if user.Username != "UnderLX" {
		_, err := dg.UserUpdate("", "", "UnderLX", "", "")
		if err != nil {
			return err
		}
	}
	dg.AddHandler(messageCreate)
	dg.AddHandler(guildCreate)
	dg.AddHandler(guildDelete)
	dg.AddHandler(guildMemberAdded)
	dg.AddHandler(guildMemberRemoved)
	dg.AddHandler(channelCreate)
	dg.AddHandler(channelDelete)
	dg.AddHandler(messageReactionAdd)
	// Open a websocket connection to Discord and begin listening.
	return dg.Open()
}

// Stop stops the Discord bot
func Stop() {
	// Cleanly close down the Discord session.
	if session != nil {
		session.Close()
	}
}

func guildCreate(s *discordgo.Session, m *discordgo.GuildCreate) {
	if botstats.startTime.IsZero() {
		botstats.startTime = time.Now()
	}
	botstats.userCount += m.Guild.MemberCount
	for _, member := range m.Guild.Members {
		if member.User.Bot {
			botstats.botCount++
		}
	}
	for _, channel := range m.Guild.Channels {
		switch channel.Type {
		case discordgo.ChannelTypeGuildText:
			botstats.textChannelCount++
		case discordgo.ChannelTypeGuildVoice:
			botstats.voiceChannelCount++
		}
	}
	guildIDs.Store(m.ID, m.Guild.MemberCount)
}

func guildDelete(s *discordgo.Session, m *discordgo.GuildDelete) {
	c, ok := guildIDs.Load(m.ID)
	if ok {
		botstats.userCount -= c.(int)
	}
	guildIDs.Delete(m.ID)
}

func guildMemberAdded(s *discordgo.Session, m *discordgo.GuildMemberAdd) {
	botstats.userCount++
}

func guildMemberRemoved(s *discordgo.Session, m *discordgo.GuildMemberRemove) {
	botstats.userCount--
}

func channelCreate(s *discordgo.Session, m *discordgo.ChannelCreate) {
	switch m.Channel.Type {
	case discordgo.ChannelTypeDM:
		botstats.dmChannelCount++
	case discordgo.ChannelTypeGroupDM:
		botstats.groupDMChannelCount++
	}
}

func channelDelete(s *discordgo.Session, m *discordgo.ChannelDelete) {
	switch m.Channel.Type {
	case discordgo.ChannelTypeDM:
		botstats.dmChannelCount--
	case discordgo.ChannelTypeGroupDM:
		botstats.groupDMChannelCount--
	}
}

func messageReactionAdd(s *discordgo.Session, m *discordgo.MessageReactionAdd) {
	v, ok := tempMessages.Load(m.MessageID)
	if m.MessageReaction.UserID == s.State.User.ID || !ok {
		return
	}

	ch := v.(chan interface{})
	ch <- true
}

// This function will be called (due to AddHandler above) every time a new
// message is created on any channel that the autenticated bot has access to.
func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	// Ignore all messages created by the bot itself and other bots
	if m.Author.ID == s.State.User.ID || m.Author.Bot {
		return
	}

	muted := !time.Now().After(stopMute[m.ChannelID]) || channelMute[m.ChannelID]
	for _, handler := range messageHandlers {
		if handler.Handle(s, m, muted) {
			return
		}
	}

}

func handleLineStatus(s *discordgo.Session, m *discordgo.MessageCreate, words []string) {
	if len(words) < 3 {
		s.ChannelMessageSend(m.ChannelID, "🆖 missing arguments")
		return
	}
	id, err := uuid.NewV4()
	if err != nil {
		s.ChannelMessageSend(m.ChannelID, "❌ "+err.Error())
		return
	}

	tx, err := node.Beginx()
	if err != nil {
		s.ChannelMessageSend(m.ChannelID, "❌ "+err.Error())
		return
	}
	defer tx.Commit() // read-only tx

	status := &dataobjects.Status{
		ID:   id.String(),
		Time: time.Now().UTC(),
		Source: &dataobjects.Source{
			ID:        "underlx-bot",
			Name:      "UnderLX Discord bot",
			Automatic: false,
			Official:  false,
		},
	}

	switch words[0] {
	case "up":
		status.IsDowntime = false
	case "down":
		status.IsDowntime = true
	default:
		s.ChannelMessageSend(m.ChannelID, "🆖 first argument must be `up` or `down`")
		return
	}

	line, err := dataobjects.GetLine(tx, words[1])
	if err != nil {
		lines, err := dataobjects.GetLines(tx)
		if err != nil {
			s.ChannelMessageSend(m.ChannelID, "❌ "+err.Error())
			return
		}
		lineIDs := make([]string, len(lines))
		for i := range lines {
			lineIDs[i] = "`" + lines[i].ID + "`"
		}
		s.ChannelMessageSend(m.ChannelID, "🆖 line ID must be one of ["+strings.Join(lineIDs, ",")+"]")
		return
	}

	status.Line = line
	status.Status = strings.Join(words[2:], " ")

	cmdCallback(&NewStatusCommand{
		Status: status,
	})
	s.ChannelMessageSend(m.ChannelID, "✅")
}

func handleControlScraper(s *discordgo.Session, m *discordgo.MessageCreate, words []string) {
	if len(words) < 2 {
		s.ChannelMessageSend(m.ChannelID, "🆖 missing arguments")
		return
	}

	var enabled bool
	switch words[0] {
	case "start":
		enabled = true
	case "stop":
		enabled = false
	default:
		s.ChannelMessageSend(m.ChannelID, "🆖 first argument must be `start` or `stop`")
		return
	}

	cmdCallback(&ControlScraperCommand{
		Scraper: words[1],
		Enable:  enabled,
		MessageCallback: func(message string) {
			s.ChannelMessageSend(m.ChannelID, message)
		},
	})
}

func handleControlNotifs(s *discordgo.Session, m *discordgo.MessageCreate, words []string) {
	if len(words) < 2 {
		s.ChannelMessageSend(m.ChannelID, "🆖 missing arguments")
		return
	}

	var enabled bool
	switch words[0] {
	case "mute":
		enabled = false
	case "unmute":
		enabled = true
	default:
		s.ChannelMessageSend(m.ChannelID, "🆖 first argument must be `mute` or `unmute`")
		return
	}

	switch words[1] {
	case "status":
	case "announcements":
		break
	default:
		s.ChannelMessageSend(m.ChannelID, "🆖 second argument must be `status` or `announcements`")
		return
	}

	cmdCallback(&ControlNotifsCommand{
		Type:   words[1],
		Enable: enabled,
	})
	s.ChannelMessageSend(m.ChannelID, "✅")
}

// RUSSIA: Remarkably Ubiquitous and Safe System for Intelligent Abetment
func handleRUSSIA(s *discordgo.Session, m *discordgo.MessageCreate, words []string) {
	if len(words) < 1 {
		s.ChannelMessageSend(m.ChannelID, "🆖 missing arguments")
		return
	}

	tx, err := node.Beginx()
	if err != nil {
		s.ChannelMessageSend(m.ChannelID, "❌ "+err.Error())
		return
	}
	defer tx.Commit() // read-only tx

	var line *dataobjects.Line
	if words[0] == "cast" || words[0] == "empty" {
		line, err = dataobjects.GetLine(tx, words[1])
		if err != nil {
			lines, err := dataobjects.GetLines(tx)
			if err != nil {
				s.ChannelMessageSend(m.ChannelID, "❌ "+err.Error())
				return
			}
			lineIDs := make([]string, len(lines))
			for i := range lines {
				lineIDs[i] = "`" + lines[i].ID + "`"
			}
			s.ChannelMessageSend(m.ChannelID, "🆖 line ID must be one of ["+strings.Join(lineIDs, ",")+"]")
			return
		}
	}

	switch words[0] {
	case "cast":
		if len(words) < 3 {
			s.ChannelMessageSend(m.ChannelID, "🆖 missing arguments")
			return
		}
		weight, err := strconv.Atoi(words[2])
		if err != nil {
			s.ChannelMessageSend(m.ChannelID, "❌ "+err.Error())
			return
		}
		cmdCallback(&ReportDisturbanceCommand{
			Line:   line,
			Weight: weight,
		})
		s.ChannelMessageSend(m.ChannelID, "✅")
	case "empty":
		if len(words) < 2 {
			s.ChannelMessageSend(m.ChannelID, "🆖 missing arguments")
			return
		}
		cmdCallback(&ClearDisturbanceReportsCommand{
			Line: line,
		})
		s.ChannelMessageSend(m.ChannelID, "✅")
	case "show":
		cmdCallback(&GetDisturbanceReportsCommand{
			MessageCallback: func(message string) {
				s.ChannelMessageSend(m.ChannelID, message)
			},
		})
	case "multiplier":
		if len(words) < 2 {
			command := &ReportThresholdMultiplierCommand{}
			cmdCallback(command)
			s.ChannelMessageSend(m.ChannelID, strconv.FormatFloat(float64(command.Multiplier), 'f', 3, 32))
		} else {
			multiplier, err := strconv.ParseFloat(words[1], 32)
			if err != nil {
				s.ChannelMessageSend(m.ChannelID, "❌ "+err.Error())
				return
			} else if multiplier <= 0 {
				s.ChannelMessageSend(m.ChannelID, "🆖 must be > 0")
				return
			}
			cmdCallback(&ReportThresholdMultiplierCommand{
				Set:        true,
				Multiplier: float32(multiplier),
			})
			s.ChannelMessageSend(m.ChannelID, "✅")
		}
	case "offset":
		if len(words) < 2 {
			command := &ReportThresholdOffsetCommand{}
			cmdCallback(command)
			s.ChannelMessageSend(m.ChannelID, strconv.FormatInt(int64(command.Offset), 10))
		} else {
			offset, err := strconv.ParseInt(words[1], 10, 64)
			if err != nil {
				s.ChannelMessageSend(m.ChannelID, "❌ "+err.Error())
				return
			}
			cmdCallback(&ReportThresholdOffsetCommand{
				Set:    true,
				Offset: int(offset),
			})
			s.ChannelMessageSend(m.ChannelID, "✅")
		}
	default:
		s.ChannelMessageSend(m.ChannelID, "🆖 first argument must be `cast`, `empty`, `multiplier`, `offset` or `show`")
		return
	}

}

func handleStatus(s *discordgo.Session, m *discordgo.MessageCreate, words []string) {
	var err error
	if len(words) == 0 {
		err = s.UpdateStatus(0, "")
	} else if len(words) > 0 {
		usd := &discordgo.UpdateStatusData{
			Status: "online",
		}

		switch words[0] {
		case "playing":
			usd.Game = &discordgo.Game{
				Name: strings.Join(words[1:], " "),
				Type: discordgo.GameTypeGame,
			}
		case "streaming":
			usd.Game = &discordgo.Game{
				Type: discordgo.GameTypeGame,
				URL:  strings.Join(words[1:], " "),
			}
		case "listening":
			usd.Game = &discordgo.Game{
				Name: strings.Join(words[1:], " "),
				Type: discordgo.GameTypeListening,
			}
		case "watching":
			usd.Game = &discordgo.Game{
				Name: strings.Join(words[1:], " "),
				Type: discordgo.GameTypeWatching,
			}
		default:
			usd.Game = &discordgo.Game{
				Name: strings.Join(words, " "),
				Type: discordgo.GameTypeGame,
			}
		}

		err = s.UpdateStatusComplex(*usd)
	}
	if err != nil {
		s.ChannelMessageSend(m.ChannelID, "❌ "+err.Error())
	} else {
		s.ChannelMessageSend(m.ChannelID, "✅")
	}
}

func getEmojiForLine(id string) string {
	switch id {
	case "pt-ml-azul":
		return "<:ml_azul:459100543240110091>"
	case "pt-ml-amarela":
		return "<:ml_amarela:459100497895227403>"
	case "pt-ml-verde":
		return "<:ml_verde:459100596549451776>"
	case "pt-ml-vermelha":
		return "<:ml_vermelha:459100637985112095>"
	case "pt-ml-laranja":
		return "<:ml_laranja:455786569446588446>"
	}
	return ""
}
