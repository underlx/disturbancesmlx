package discordbot

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
)

// this file contains functions to link the bot with the PosPlay subsystem

// ProjectGuildMember returns a user from the project guild by ID
func ProjectGuildMember(userID string) (*discordgo.Member, error) {
	if session == nil || commandLib == nil || commandLib.adminChannelID == "" {
		return nil, errors.New("Bot not initialized")
	}
	adminChannel, err := session.Channel(commandLib.adminChannelID)
	if err != nil {
		return nil, err
	}

	return session.GuildMember(adminChannel.GuildID, userID)
}

// User returns the user details of the given userID
func User(userID string) (*discordgo.User, error) {
	return session.User(userID)
}

// SendDMtoUser sends a direct message to the specified user with the specified content
func SendDMtoUser(userID string, data *discordgo.MessageSend) (*discordgo.Message, error) {
	if session == nil {
		return nil, errors.New("Bot not initialized")
	}

	channel, err := session.UserChannelCreate(userID)
	if err != nil {
		return nil, err
	}

	return session.ChannelMessageSendComplex(channel.ID, data)
}

// PosPlayEventManager manages PosPlay reaction events
type PosPlayEventManager struct {
	OnReactionCallback      func(userID, messageID string, XPreward int) bool
	ongoing                 sync.Map
	reactionsHandledCount   int
	reactionsActedUponCount int
}

type posPlayEvent struct {
	MessageID string
	XPreward  int
	StopChan  chan interface{}
}

func (e *PosPlayEventManager) handleStartCommand(s *discordgo.Session, m *discordgo.MessageCreate, args []string) {
	if len(args) < 4 {
		s.ChannelMessageSend(m.ChannelID, "🆖 missing arguments: [channel ID] [XP reward] [duration in seconds] [message]")
		return
	}
	channel, err := s.Channel(args[0])
	if err != nil {
		s.ChannelMessageSend(m.ChannelID, "❌ "+err.Error())
		return
	}
	xpReward, err := strconv.Atoi(args[1])
	if err != nil {
		s.ChannelMessageSend(m.ChannelID, "❌ "+err.Error())
		return
	}
	seconds, err := strconv.Atoi(args[2])
	if err != nil {
		s.ChannelMessageSend(m.ChannelID, "❌ "+err.Error())
		return
	}
	if len(args[3]) == 0 {
		s.ChannelMessageSend(m.ChannelID, "❌ missing message")
		return
	}

	message, err := s.ChannelMessageSend(channel.ID, strings.Join(args[3:], " "))
	if err != nil {
		s.ChannelMessageSend(m.ChannelID, "❌ "+err.Error())
		return
	}

	event := posPlayEvent{
		MessageID: message.ID,
		XPreward:  xpReward,
		StopChan:  make(chan interface{}, 1),
	}
	duration := time.Duration(seconds) * time.Second
	e.ongoing.Store(message.ID, event)
	go func() {
		select {
		case <-event.StopChan:
			s.ChannelMessageEdit(channel.ID, message.ID, message.Content+"\n**Este evento terminou.**")
			e.ongoing.Delete(message.ID)
		case <-time.After(duration):
			s.ChannelMessageEdit(channel.ID, message.ID, message.Content+"\n**Este evento terminou. Seja mais rápido da próxima vez!**")
			e.ongoing.Delete(message.ID)
		}
	}()
	s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("✅ event with ID %s started on channel <#%s> with duration %s and %d XP reward per user",
		message.ID, channel.ID, duration.String(), xpReward))
}

func (e *PosPlayEventManager) handleStopCommand(s *discordgo.Session, m *discordgo.MessageCreate, args []string) {
	if len(args) < 1 {
		s.ChannelMessageSend(m.ChannelID, "🆖 missing event ID argument")
		return
	}

	v, ok := e.ongoing.Load(args[0])
	if !ok {
		s.ChannelMessageSend(m.ChannelID, "❌ no ongoing event with the specified ID")
	}
	event := v.(posPlayEvent)
	event.StopChan <- true
	s.ChannelMessageSend(m.ChannelID, "✅")
}

// HandleReaction attempts to handle the provided reaction
// always returns false as this is a non-authoritative handler
func (e *PosPlayEventManager) HandleReaction(s *discordgo.Session, m *discordgo.MessageReactionAdd) bool {
	e.reactionsHandledCount++
	v, ok := e.ongoing.Load(m.MessageID)
	if !ok {
		return false
	}

	event := v.(posPlayEvent)
	if e.OnReactionCallback != nil && e.OnReactionCallback(m.UserID, m.MessageID, event.XPreward) {
		e.reactionsActedUponCount++
	}
	return false
}

// Name returns the name of this reaction handler
func (e *PosPlayEventManager) Name() string {
	return "ReactionEventManager"
}

// ReactionsHandled returns the number of reactions handled by this InfoHandler
func (e *PosPlayEventManager) ReactionsHandled() int {
	return e.reactionsHandledCount
}

// ReactionsActedUpon returns the number of reactions acted upon by this InfoHandler
func (e *PosPlayEventManager) ReactionsActedUpon() int {
	return e.reactionsActedUponCount
}
