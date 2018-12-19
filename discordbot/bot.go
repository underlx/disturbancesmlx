package discordbot

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"math/rand"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gbl08ma/keybox"

	"github.com/bwmarrin/discordgo"
	"github.com/gbl08ma/sqalx"
	cache "github.com/patrickmn/go-cache"
	uuid "github.com/satori/go.uuid"
	"github.com/underlx/disturbancesmlx/dataobjects"
	"github.com/underlx/disturbancesmlx/utils"
)

var started bool
var muteManager = NewMuteManager()
var commandLib *CommandLibrary
var messageHandlers []MessageHandler
var reactionHandlers []ReactionHandler
var guildIDs sync.Map

var botstats = stats{
	DMChannels: make(map[string]bool),
}

type stats struct {
	StartTime           time.Time
	UserCount           int
	BotCount            int
	DMChannels          map[string]bool
	GroupDMChannelCount int
	TextChannelCount    int
	VoiceChannelCount   int
}

var recentInvites *cache.Cache

type inviteInfo struct {
	Code            string
	RequesterIPAddr string
	RequestTime     time.Time
	InviteTime      time.Time
}

var node sqalx.Node
var websiteURL string
var botLog *log.Logger
var session *discordgo.Session
var cmdReceiver CommandReceiver

// ThePosPlayBridge is the PosPlayBridge of the bot
// (exported so the posplay package can reach it)
var ThePosPlayBridge = new(PosPlayBridge)

var scriptSystem = new(ScriptSystem)

// Start starts the Discord bot
func Start(snode sqalx.Node, swebsiteURL string, keybox *keybox.Keybox,
	log *log.Logger,
	cmdRecv CommandReceiver) error {
	started = true
	node = snode
	websiteURL = swebsiteURL
	botLog = log
	cmdReceiver = cmdRecv
	recentInvites = cache.New(24*time.Hour, 1*time.Hour)
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
		msgSend := &discordgo.MessageSend{
			Content: "🙌",
		}
		if len(embed.Fields) > 0 {
			msgSend.Embed = embed.MessageEmbed
		}
		beforeSend := time.Now()
		msg, err := s.ChannelMessageSendComplex(m.ChannelID, msgSend)
		if err == nil {
			s.ChannelMessageEdit(m.ChannelID, msg.ID, fmt.Sprintf("🙌 RTT mensagem: %d ms", time.Since(beforeSend).Nanoseconds()/1000000))
		}
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
		embed, err := buildAboutMessage(s, m)
		if err == nil {
			s.ChannelMessageSendEmbed(m.ChannelID, embed.MessageEmbed)
		}
	}))
	commandLib.Register(NewCommand("xp", func(s *discordgo.Session, m *discordgo.MessageCreate, args []string) {
		embed, err := buildPosPlayXPMessage(m)
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
				msg += commandLib.prefix + command.Name
				if command.RequirePrivilege == PrivilegeNobody {
					msg += " _(disabled)_"
				}
				msg += "\n"
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
		muteManager.MuteChannel(m.ChannelID, muteDuration)
		if muteDuration.Minutes() < 60.0 {
			s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("🤐 por %d minutos", int(math.Round(muteDuration.Minutes()))))
		} else {
			s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("🤐 por %s", muteDuration.String()))
		}
	}))
	commandLib.Register(NewCommand("unmute", func(s *discordgo.Session, m *discordgo.MessageCreate, args []string) {
		muteManager.UnmuteChannel(m.ChannelID)
		s.ChannelMessageSend(m.ChannelID, "🤗")
	}))
	commandLib.Register(NewCommand("permamute", func(s *discordgo.Session, m *discordgo.MessageCreate, args []string) {
		muteManager.PermaMuteChannel(m.ChannelID)
		s.ChannelMessageSend(m.ChannelID, "🤐💀")
	}).WithRequirePrivilege(PrivilegeAdmin))
	commandLib.Register(NewCommand("permaunmute", func(s *discordgo.Session, m *discordgo.MessageCreate, args []string) {
		muteManager.PermaUnmuteChannel(m.ChannelID)
		s.ChannelMessageSend(m.ChannelID, "🤗🙌")
	}).WithRequirePrivilege(PrivilegeAdmin))
	commandLib.Register(NewCommand("setstatus", handleStatus).WithRequirePrivilege(PrivilegeAdmin))
	commandLib.Register(NewCommand("addlinestatus", handleLineStatus).WithRequirePrivilege(PrivilegeAdmin))
	commandLib.Register(NewCommand("scraper", handleControlScraper).WithRequirePrivilege(PrivilegeAdmin))
	commandLib.Register(NewCommand("notifs", handleControlNotifs).WithRequirePrivilege(PrivilegeAdmin))
	commandLib.Register(NewCommand("russia", handleRUSSIA).WithRequirePrivilege(PrivilegeAdmin))
	commandLib.Register(NewCommand("sendbroadcast", handleSendBroadcast).WithRequirePrivilege(PrivilegeAdmin))
	commandLib.Register(NewCommand("sendcommand", handleSendCommand).WithRequirePrivilege(PrivilegeAdmin))
	commandLib.Register(NewCommand("sendtochannel", handleSendToChannel).WithRequirePrivilege(PrivilegeAdmin))
	commandLib.Register(NewCommand("emptychannel", handleEmptyChannel).WithRequirePrivilege(PrivilegeAdmin))
	commandLib.Register(NewCommand("invitehistory", handleInviteHistory).WithRequirePrivilege(PrivilegeAdmin))
	commandLib.Register(NewCommand("startreactionevent", ThePosPlayBridge.handleStartCommand).WithRequirePrivilege(PrivilegeAdmin))
	commandLib.Register(NewCommand("startquizevent", ThePosPlayBridge.handleQuizStartCommand).WithRequirePrivilege(PrivilegeAdmin))
	commandLib.Register(NewCommand("stopevent", ThePosPlayBridge.handleStopCommand).WithRequirePrivilege(PrivilegeAdmin))
	commandLib.Register(NewCommand("markspamchannel", ThePosPlayBridge.handleMarkSpamChannel).WithRequirePrivilege(PrivilegeAdmin))
	commandLib.Register(NewCommand("unmarkspamchannel", ThePosPlayBridge.handleUnmarkSpamChannel).WithRequirePrivilege(PrivilegeAdmin))
	commandLib.Register(NewCommand("setprefix", func(s *discordgo.Session, m *discordgo.MessageCreate, args []string) {
		if len(args) == 0 {
			commandLib.SetPrefix("")
		} else {
			commandLib.SetPrefix(args[0])
		}
		s.ChannelMessageSend(m.ChannelID, "✅")
	}).WithRequirePrivilege(PrivilegeRoot))
	commandLib.Register(NewCommand("setcmdpriv", handleSetCommandPrivilege).WithRequirePrivilege(PrivilegeRoot))
	scriptSystem.Setup(commandLib, PrivilegeRoot)
	new(SQLSystem).Setup(node, commandLib, PrivilegeRoot)

	reactionHandlers = append(reactionHandlers, ThePosPlayBridge)
	messageHandlers = append(messageHandlers, ThePosPlayBridge)

	infoHandler, err := NewInfoHandler(node)
	if err != nil {
		return err
	}
	messageHandlers = append(messageHandlers, infoHandler)
	reactionHandlers = append(reactionHandlers, infoHandler)

	/*disduper := new(bot.Disduper)
	err = disduper.InitIntegrated(log, session)
	if err != nil {
		return err
	}
	messageHandlers = append(messageHandlers, disduper)*/

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
	if !started {
		return
	}
	started = false
	// Cleanly close down the Discord session.
	if session != nil {
		session.Close()
	}
	scriptSystem.doClear()
}

// CreateInvite creates a single-use invite for the specified channel
func CreateInvite(channelID, requesterIPaddr string) (*discordgo.Invite, error) {
	if !started || session == nil {
		return nil, fmt.Errorf("Bot not ready")
	}
	invite := discordgo.Invite{
		MaxAge:    600,
		MaxUses:   1,
		Temporary: false,
	}
	i, err := session.ChannelInviteCreate(channelID, invite)
	if err == nil {
		uuid, err := uuid.NewV4()
		if err != nil {
			return i, err
		}
		inviteTime, _ := i.CreatedAt.Parse()
		recentInvites.SetDefault(uuid.String(), inviteInfo{
			Code:            i.Code,
			RequesterIPAddr: requesterIPaddr,
			RequestTime:     time.Now(),
			InviteTime:      inviteTime,
		})
	}
	return i, err
}

func guildCreate(s *discordgo.Session, m *discordgo.GuildCreate) {
	if botstats.StartTime.IsZero() {
		botstats.StartTime = time.Now()
	}
	botstats.UserCount += m.Guild.MemberCount
	for _, member := range m.Guild.Members {
		if member.User.Bot {
			botstats.BotCount++
		}
	}
	for _, channel := range m.Guild.Channels {
		switch channel.Type {
		case discordgo.ChannelTypeGuildText:
			botstats.TextChannelCount++
		case discordgo.ChannelTypeGuildVoice:
			botstats.VoiceChannelCount++
		}
	}
	guildIDs.Store(m.ID, m.Guild.MemberCount)
}

func guildDelete(s *discordgo.Session, m *discordgo.GuildDelete) {
	c, ok := guildIDs.Load(m.ID)
	if ok {
		botstats.UserCount -= c.(int)
	}
	guildIDs.Delete(m.ID)
}

func guildMemberAdded(s *discordgo.Session, m *discordgo.GuildMemberAdd) {
	botstats.UserCount++
	if m.Member.User.Bot {
		botstats.BotCount++
	}
}

func guildMemberRemoved(s *discordgo.Session, m *discordgo.GuildMemberRemove) {
	botstats.UserCount--
	if m.Member.User.Bot {
		botstats.BotCount--
	}
}

func channelCreate(s *discordgo.Session, m *discordgo.ChannelCreate) {
	switch m.Channel.Type {
	case discordgo.ChannelTypeDM:
		botstats.DMChannels[m.Channel.ID] = true
	case discordgo.ChannelTypeGroupDM:
		botstats.GroupDMChannelCount++
	}
}

func channelDelete(s *discordgo.Session, m *discordgo.ChannelDelete) {
	switch m.Channel.Type {
	case discordgo.ChannelTypeDM:
		delete(botstats.DMChannels, m.Channel.ID)
	case discordgo.ChannelTypeGroupDM:
		botstats.GroupDMChannelCount--
	}
}

func messageReactionAdd(s *discordgo.Session, m *discordgo.MessageReactionAdd) {
	// Ignore all reactions created by the bot itself
	if m.UserID == s.State.User.ID {
		return
	}

	for _, handler := range reactionHandlers {
		if handler.HandleReaction(s, m) {
			return
		}
	}
}

// This function will be called (due to AddHandler above) every time a new
// message is created on any channel that the autenticated bot has access to.
func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	// Ignore all messages created by the bot itself and other bots
	if m.Author.ID == s.State.User.ID || m.Author.Bot {
		return
	}

	muted := muteManager.MutedAny(m.ChannelID)
	for _, handler := range messageHandlers {
		if handler.HandleMessage(s, m, muted) {
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

	cmdReceiver.NewLineStatus(status)
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

	cmdReceiver.ControlScraper(words[1], enabled, func(message string) {
		s.ChannelMessageSend(m.ChannelID, message)
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

	cmdReceiver.ControlNotifs(words[1], enabled)

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
		cmdReceiver.CastDisturbanceVote(line, weight)
		s.ChannelMessageSend(m.ChannelID, "✅")
	case "empty":
		if len(words) < 2 {
			s.ChannelMessageSend(m.ChannelID, "🆖 missing arguments")
			return
		}
		cmdReceiver.ClearDisturbanceVotes(line)
		s.ChannelMessageSend(m.ChannelID, "✅")
	case "show":
		cmdReceiver.GetDisturbanceVotes(func(message string) {
			s.ChannelMessageSend(m.ChannelID, message)
		})
	case "multiplier":
		if len(words) < 2 {
			multiplier := cmdReceiver.GetThresholdMultiplier()
			s.ChannelMessageSend(m.ChannelID, strconv.FormatFloat(float64(multiplier), 'f', 3, 32))
		} else {
			multiplier, err := strconv.ParseFloat(words[1], 32)
			if err != nil {
				s.ChannelMessageSend(m.ChannelID, "❌ "+err.Error())
				return
			} else if multiplier <= 0 {
				s.ChannelMessageSend(m.ChannelID, "🆖 must be > 0")
				return
			}
			cmdReceiver.SetThresholdMultiplier(float32(multiplier))
			s.ChannelMessageSend(m.ChannelID, "✅")
		}
	case "offset":
		if len(words) < 2 {
			offset := cmdReceiver.GetThresholdOffset()
			s.ChannelMessageSend(m.ChannelID, strconv.FormatInt(int64(offset), 10))
		} else {
			offset, err := strconv.ParseInt(words[1], 10, 64)
			if err != nil {
				s.ChannelMessageSend(m.ChannelID, "❌ "+err.Error())
				return
			}
			cmdReceiver.SetThresholdOffset(int(offset))
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

func handleSendBroadcast(s *discordgo.Session, m *discordgo.MessageCreate, args []string) {
	if len(args) < 2 {
		s.ChannelMessageSend(m.ChannelID, "🆖 missing arguments")
		return
	} else if len(args) < 3 {
		cmdReceiver.SendNotificationMetaBroadcast("", "", args[0], args[1], "")
	} else if len(args) < 4 {
		cmdReceiver.SendNotificationMetaBroadcast("", "", args[0], args[1], args[2])
	} else if len(args) < 5 {
		cmdReceiver.SendNotificationMetaBroadcast("", args[0], args[1], args[2], args[3])
	} else {
		cmdReceiver.SendNotificationMetaBroadcast(args[0], args[1], args[2], args[3], args[4])
	}
	s.ChannelMessageSend(m.ChannelID, "✅")
}

func handleSendCommand(s *discordgo.Session, m *discordgo.MessageCreate, args []string) {
	if len(args) < 3 {
		s.ChannelMessageSend(m.ChannelID, "🆖 missing arguments")
		return
	}
	cmdReceiver.SendCommandMetaBroadcast(args[0], args[1], args[2], args[3:]...)
	s.ChannelMessageSend(m.ChannelID, "✅")
}

func handleSendToChannel(s *discordgo.Session, m *discordgo.MessageCreate, args []string) {
	if len(args) < 2 {
		s.ChannelMessageSend(m.ChannelID, "🆖 missing arguments")
		return
	}

	var netClient = &http.Client{
		Timeout: time.Second * 10,
	}
	response, err := netClient.Get(args[1])
	if err != nil {
		s.ChannelMessageSend(m.ChannelID, "❌ "+err.Error())
		return
	}
	defer response.Body.Close()
	var messages []*discordgo.MessageSend
	err = json.NewDecoder(response.Body).Decode(&messages)
	if err != nil {
		s.ChannelMessageSend(m.ChannelID, "❌ "+err.Error())
		return
	}

	for _, message := range messages {
		_, err = s.ChannelMessageSendComplex(args[0], message)
		if err != nil {
			s.ChannelMessageSend(m.ChannelID, "❌ "+err.Error())
			return
		}
	}
	s.ChannelMessageSend(m.ChannelID, "✅")
}

func handleEmptyChannel(s *discordgo.Session, m *discordgo.MessageCreate, args []string) {
	if len(args) < 1 {
		s.ChannelMessageSend(m.ChannelID, "🆖 missing arguments")
		return
	}

	before := ""
	messageIDs := []string{}
	for {
		messages, err := s.ChannelMessages(args[0], 100, before, "", "")
		if err != nil {
			s.ChannelMessageSend(m.ChannelID, "❌ "+err.Error())
			return
		}
		if len(messages) == 0 {
			break
		}
		for _, message := range messages {
			if len(args) > 1 && args[1] != message.Author.ID {
				continue
			}
			messageIDs = append(messageIDs, message.ID)
			before = message.ID
		}
	}

	for i, id := range messageIDs {
		err := s.ChannelMessageDelete(args[0], id)
		if err != nil {
			s.ChannelMessageSend(m.ChannelID, "❌ "+err.Error())
			return
		}
		if (i+1)%10 == 0 {
			s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("🗑 %d/%d (%.00f%%) on `%s`. You can keep using the bot.",
				i+1, len(messageIDs), float64(i+1)/float64(len(messageIDs))*100, args[0]))
		}
	}
	s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("🗑 %d messages on `%s`", len(messageIDs), args[0]))
}

func handleInviteHistory(s *discordgo.Session, m *discordgo.MessageCreate, args []string) {
	items := recentInvites.Items()
	if len(items) == 0 {
		s.ChannelMessageSend(m.ChannelID, "🏜")
		return
	}
	invites := make([]inviteInfo, len(items))
	i := 0
	for _, item := range items {
		invites[i] = item.Object.(inviteInfo)
		i++
	}
	sort.Slice(invites, func(i, j int) bool {
		return invites[i].RequestTime.Before(invites[j].RequestTime)
	})
	message := ""
	for _, invite := range invites {
		line := invite.RequestTime.UTC().Format(time.RFC3339)
		if utils.DurationAbs(invite.RequestTime.Sub(invite.InviteTime)) > 5*time.Second {
			line += " (" + invite.InviteTime.UTC().Format(time.RFC3339) + ")"
		}
		line += " - " + invite.RequesterIPAddr + " - " + invite.Code + "\n"
		if len(message)+len(line) > 2000 {
			s.ChannelMessageSend(m.ChannelID, message)
			message = ""
		}
		message += line
	}
	s.ChannelMessageSend(m.ChannelID, message)
}

func handleSetCommandPrivilege(s *discordgo.Session, m *discordgo.MessageCreate, args []string) {
	if len(args) < 2 {
		s.ChannelMessageSend(m.ChannelID, "🆖 missing arguments")
		return
	}
	command, present := commandLib.Get(args[0])
	if !present {
		s.ChannelMessageSend(m.ChannelID, "🆖 invalid command")
		return
	}
	switch args[1] {
	case "nobody":
		if command.Name != "setcmdpriv" {
			command.RequirePrivilege = PrivilegeNobody
		} else {
			s.ChannelMessageSend(m.ChannelID, "https://www.youtube.com/watch?v=7qnd-hdmgfk")
			return
		}
	case "root":
		command.RequirePrivilege = PrivilegeRoot
	case "admin":
		command.RequirePrivilege = PrivilegeAdmin
	case "everyone":
		command.RequirePrivilege = PrivilegeEveryone
	default:
		s.ChannelMessageSend(m.ChannelID, "🆖 second argument must be `nobody`, `root`, `admin` or `everyone`")
		return
	}

	s.ChannelMessageSend(m.ChannelID, "✅")
}

func getEmojiSnowflakeForLine(id string) string {
	switch id {
	case "pt-ml-azul":
		return "459100543240110091"
	case "pt-ml-amarela":
		return "459100497895227403"
	case "pt-ml-verde":
		return "459100596549451776"
	case "pt-ml-vermelha":
		return "459100637985112095"
	case "pt-ml-laranja":
		return "455786569446588446"
	}
	return ""
}

func getEmojiForLine(id string) string {
	p := strings.Split(id, "-")
	return "<:ml_" + p[len(p)-1] + ":" + getEmojiSnowflakeForLine(id) + ">"
}

func getEmojiURLForLine(id string) string {
	return "https://cdn.discordapp.com/emojis/" + getEmojiSnowflakeForLine(id) + ".png"
}
