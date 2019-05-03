package discordbot

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/thoas/go-funk"

	"github.com/bwmarrin/discordgo"
	cache "github.com/patrickmn/go-cache"
	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
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

// PosPlayBridge manages PosPlay reaction events and rewards for user participation
type PosPlayBridge struct {
	PlayerXPInfo                      func(userID string) (PosPlayXPInfo, error)
	OnEventWinCallback                func(userID, messageID string, XPreward int, eventType string) bool
	OnDiscussionParticipationCallback func(userID string, XPreward int) bool
	ReloadAchievementsCallback        func() error
	ongoing                           sync.Map
	participation                     *cache.Cache
	spamChannels                      []string
	reactionsHandledCount             int
	reactionsActedUponCount           int
	handledCount                      int
	actedUponCount                    int
}

// PosPlayXPInfo contains information for the $xp command
type PosPlayXPInfo struct {
	Username      string
	ProfileURL    string
	AvatarURL     string
	Level         int
	LevelProgress float64
	XP            int
	XPthisWeek    int
	Rank          int
	RankThisWeek  int
}

func init() {
	ThePosPlayBridge.participation = cache.New(3*time.Minute, 10*time.Minute)
}

type stoppable interface {
	StopChan() chan interface{}
}

type posPlayEvent struct {
	MessageID string
	XPreward  int
	stopChan  chan interface{}
}

func (e posPlayEvent) StopChan() chan interface{} {
	return e.stopChan
}

type posPlayReactionEvent struct {
	posPlayEvent
}

type posPlayQuizEvent struct {
	posPlayEvent
	Trigger      string
	Answer       string
	MaxAttempts  uint
	AttemptTally *sync.Map
}

func (e *PosPlayBridge) handleReloadAchievements(s *discordgo.Session, m *discordgo.MessageCreate, args []string) {
	err := e.ReloadAchievementsCallback()
	if err != nil {
		s.ChannelMessageSend(m.ChannelID, "❌ "+err.Error())
		return
	}

	s.ChannelMessageSend(m.ChannelID, "✅")
}

func (e *PosPlayBridge) handleStartCommand(s *discordgo.Session, m *discordgo.MessageCreate, args []string) {
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
	duration := time.Duration(seconds) * time.Second
	message, err := e.StartReactionEvent(s, channel, xpReward, duration, strings.Join(args[3:], " "))
	if err != nil {
		s.ChannelMessageSend(m.ChannelID, "❌ "+err.Error())
		return
	}

	s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("✅ event with ID %s started on channel <#%s> with duration %s and %d XP reward per user",
		message.ID, channel.ID, duration.String(), xpReward))
}

// StartReactionEvent starts a reaction event on the specified channel with the given XP reward and message, expiring after the given duration
func (e *PosPlayBridge) StartReactionEvent(s *discordgo.Session, channel *discordgo.Channel, xpReward int, duration time.Duration, messageContents string) (*discordgo.Message, error) {
	message, err := s.ChannelMessageSend(channel.ID, messageContents)
	if err != nil {
		return nil, err
	}
	event := posPlayReactionEvent{
		posPlayEvent: posPlayEvent{
			MessageID: message.ID,
			XPreward:  xpReward,
			stopChan:  make(chan interface{}, 1),
		},
	}
	e.ongoing.Store(message.ID, event)
	go func() {
		select {
		case <-event.StopChan():
			s.ChannelMessageEdit(channel.ID, message.ID, message.Content+"\n**Este evento terminou.**")
			e.ongoing.Delete(message.ID)
		case <-time.After(duration):
			s.ChannelMessageEdit(channel.ID, message.ID, message.Content+"\n**Este evento terminou. Seja mais rápido da próxima vez!**")
			e.ongoing.Delete(message.ID)
		}
	}()
	return message, nil
}

func (e *PosPlayBridge) handleQuizStartCommand(s *discordgo.Session, m *discordgo.MessageCreate, args []string) {
	if len(args) < 7 {
		s.ChannelMessageSend(m.ChannelID, "🆖 missing arguments: [channel ID] [answer trigger] [answer] [max attempts] [XP reward] [duration in seconds] [message]")
		return
	}
	channel, err := s.Channel(args[0])
	if err != nil {
		s.ChannelMessageSend(m.ChannelID, "❌ "+err.Error())
		return
	}
	re := regexp.MustCompile("^[a-zA-Z0-9]+$")
	if !re.MatchString(args[1]) {
		s.ChannelMessageSend(m.ChannelID, "❌ invalid answer trigger, must contain a-zA-Z0-9 only")
		return
	}
	args[1] = strings.ToLower(args[1])
	if _, ok := e.ongoing.Load(args[1]); ok {
		s.ChannelMessageSend(m.ChannelID, "❌ an event with ID `"+args[1]+"` is already running")
		return
	}
	numberOfAttempts, err := strconv.Atoi(args[3])
	if err != nil {
		s.ChannelMessageSend(m.ChannelID, "❌ "+err.Error())
		return
	}
	xpReward, err := strconv.Atoi(args[4])
	if err != nil {
		s.ChannelMessageSend(m.ChannelID, "❌ "+err.Error())
		return
	}
	seconds, err := strconv.Atoi(args[5])
	if err != nil {
		s.ChannelMessageSend(m.ChannelID, "❌ "+err.Error())
		return
	}
	if len(args[6]) == 0 {
		s.ChannelMessageSend(m.ChannelID, "❌ missing message")
		return
	}

	duration := time.Duration(seconds) * time.Second
	_, err = e.StartQuizEvent(s, channel, args[1], args[2], numberOfAttempts, xpReward, duration, strings.Join(args[6:], " "))
	if err != nil {
		s.ChannelMessageSend(m.ChannelID, "❌ "+err.Error())
		return
	}

	s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("✅ event with ID `%s` started on channel <#%s> with duration %s and %d XP reward per user",
		args[1], channel.ID, duration.String(), xpReward))
}

// StartQuizEvent starts a quiz event on the specified channel with the given XP reward and message, expiring after the given duration
func (e *PosPlayBridge) StartQuizEvent(s *discordgo.Session, channel *discordgo.Channel, answerTrigger, answer string, numberOfAttempts int, xpReward int, duration time.Duration, messageContents string) (*discordgo.Message, error) {
	message, err := s.ChannelMessageSend(channel.ID, messageContents)
	if err != nil {
		return nil, err
	}

	event := posPlayQuizEvent{
		posPlayEvent: posPlayEvent{
			MessageID: message.ID,
			XPreward:  xpReward,
			stopChan:  make(chan interface{}, 1),
		},
		Trigger:      answerTrigger,
		Answer:       answer,
		MaxAttempts:  uint(numberOfAttempts),
		AttemptTally: new(sync.Map),
	}

	e.ongoing.Store(event.Trigger, event)
	go func() {
		select {
		case <-event.StopChan():
			s.ChannelMessageEdit(channel.ID, message.ID, message.Content+"\n**Este desafio terminou.**")
			e.ongoing.Delete(event.Trigger)
		case <-time.After(duration):
			s.ChannelMessageEdit(channel.ID, message.ID, message.Content+"\n**Este desafio terminou. Seja mais rápido da próxima vez!**")
			e.ongoing.Delete(event.Trigger)
		}
	}()
	return nil, err
}

func (e *PosPlayBridge) handleStopCommand(s *discordgo.Session, m *discordgo.MessageCreate, args []string) {
	if len(args) < 1 {
		s.ChannelMessageSend(m.ChannelID, "🆖 missing event ID argument")
		return
	}

	err := e.StopEvent(strings.ToLower(args[0]))
	if err != nil {
		s.ChannelMessageSend(m.ChannelID, "❌ "+err.Error())
		return
	}
	s.ChannelMessageSend(m.ChannelID, "✅")
}

// StopEvent stops the specified event, or returns an error if the ID does not match any event
func (e *PosPlayBridge) StopEvent(eventID string) error {
	v, ok := e.ongoing.Load(eventID)
	if !ok {
		return errors.New("no ongoing event with the specified ID")
	}
	event, ok := v.(stoppable)
	if !ok {
		return errors.New("no ongoing event with the specified ID")
	}
	event.StopChan() <- true
	return nil
}

func (e *PosPlayBridge) handleMarkSpamChannel(s *discordgo.Session, m *discordgo.MessageCreate, args []string) {
	channel := m.ChannelID
	if len(args) > 0 {
		channel = args[0]
	}
	if !funk.ContainsString(e.spamChannels, channel) {
		e.spamChannels = append(e.spamChannels, channel)
	}
}

func (e *PosPlayBridge) handleUnmarkSpamChannel(s *discordgo.Session, m *discordgo.MessageCreate, args []string) {
	channel := m.ChannelID
	if len(args) > 0 {
		channel = args[0]
	}
	e.spamChannels = funk.FilterString(e.spamChannels, func(s string) bool {
		return s != channel
	})
}

// HandleReaction attempts to handle the provided reaction
// always returns false as this is a non-authoritative handler
func (e *PosPlayBridge) HandleReaction(s *discordgo.Session, m *discordgo.MessageReactionAdd) bool {
	e.reactionsHandledCount++
	v, ok := e.ongoing.Load(m.MessageID)
	if !ok {
		return false
	}

	event, ok := v.(posPlayReactionEvent)
	if ok && e.OnEventWinCallback != nil && e.OnEventWinCallback(m.UserID, event.MessageID, event.XPreward, "DISCORD_REACTION_EVENT") {
		e.reactionsActedUponCount++
	}
	return false
}

// HandleMessage attempts to handle the provided message
func (e *PosPlayBridge) HandleMessage(s *discordgo.Session, m *discordgo.MessageCreate, muted bool) bool {
	e.handledCount++
	if isdm, err := ComesFromDM(s, m); !isdm || err != nil {
		if !funk.ContainsString(e.spamChannels, m.ChannelID) {
			e.registerUserActivity(m.Author.ID)
		}
		return false
	}

	words := strings.SplitN(m.Content, " ", 2)
	reg := regexp.MustCompile("[^a-zA-Z0-9]+")
	trigger := strings.ToLower(reg.ReplaceAllString(words[0], ""))
	v, ok := e.ongoing.Load(trigger)
	if !ok {
		return false
	}

	event, ok := v.(posPlayQuizEvent)
	if !ok {
		return false
	}

	e.actedUponCount++

	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	answer, _, err := transform.String(t, strings.ToLower(event.Answer))
	if err != nil {
		botLog.Println(err)
		return true
	}
	userAnswer, _, err := transform.String(t, strings.ToLower(words[1]))
	if err != nil {
		botLog.Println(err)
		return true
	}
	attemptsiface, _ := event.AttemptTally.LoadOrStore(m.Author.ID, uint(0))
	attempts := attemptsiface.(uint)
	if event.MaxAttempts > 0 && attempts >= event.MaxAttempts {
		s.ChannelMessageSend(m.ChannelID, "❌ número máximo de tentativas excedido")
		return true
	}
	if answer != userAnswer {
		attempts++
		event.AttemptTally.Store(m.Author.ID, attempts)
		if event.MaxAttempts > 0 && attempts >= event.MaxAttempts {
			s.ChannelMessageSend(m.ChannelID, "❌ resposta incorrecta. Esgotou as suas tentativas 😦")
		} else {
			s.ChannelMessageSend(m.ChannelID, "❌ resposta incorrecta 😦")
		}
	} else if e.OnEventWinCallback != nil && e.OnEventWinCallback(m.Author.ID, event.MessageID, event.XPreward, "DISCORD_CHALLENGE_EVENT") {
		s.ChannelMessageSend(m.ChannelID, "✅ resposta correcta!")
	} else {
		s.ChannelMessageSend(m.ChannelID, "⚠ a sua resposta está correcta, mas não foi possível atribuir-lhe a recompensa.")
	}
	return true
}

func (e *PosPlayBridge) registerUserActivity(userID string) {
	_, present := e.participation.Get(userID)
	if present {
		// already counted participation in the last N minutes
		return
	}
	e.participation.SetDefault(userID, true)
	e.OnDiscussionParticipationCallback(userID, 1)
}

// Name returns the name of this handler
func (e *PosPlayBridge) Name() string {
	return "PosPlayBridge"
}

// ReactionsHandled returns the number of reactions handled by this handler
func (e *PosPlayBridge) ReactionsHandled() int {
	return e.reactionsHandledCount
}

// ReactionsActedUpon returns the number of reactions acted upon by this handler
func (e *PosPlayBridge) ReactionsActedUpon() int {
	return e.reactionsActedUponCount
}

// MessagesHandled returns the number of messages handled by this handler
func (e *PosPlayBridge) MessagesHandled() int {
	return e.handledCount
}

// MessagesActedUpon returns the number of messages acted upon by this handler
func (e *PosPlayBridge) MessagesActedUpon() int {
	return e.actedUponCount
}

// ComesFromDM returns true if a message comes from a DM channel
func ComesFromDM(s *discordgo.Session, m *discordgo.MessageCreate) (bool, error) {
	channel, err := s.State.Channel(m.ChannelID)
	if err != nil {
		if channel, err = s.Channel(m.ChannelID); err != nil {
			return false, err
		}
	}

	return channel.Type == discordgo.ChannelTypeDM, nil
}
