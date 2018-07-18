package discordbot

import (
	"strings"

	"github.com/bwmarrin/discordgo"
	shellquote "github.com/govau/go-shellquote"
)

// Privilege indicates the privilege of a user interacting with the bot, in
// order to restrict access to commands
type Privilege int

const (
	// EveryonePrivilege commands can be used by anyone
	EveryonePrivilege Privilege = iota
	// AdminPrivilege commands can be user by the bot owner or by anyone in the
	// special admin channel
	AdminPrivilege
	// RootPrivilege commands can only be used by the bot owner
	RootPrivilege
)

// Command represents a bot command
type Command struct {
	Name             string
	RequirePrivilege Privilege
	IgnoreMute       bool
	Handler          CommandHandler
}

// CommandHandler is a function capable of handling a bot commmand
type CommandHandler func(s *discordgo.Session, m *discordgo.MessageCreate, args []string)

// NewCommand returns a new Command with the specified name and handler
func NewCommand(name string, handler CommandHandler) Command {
	return Command{
		Name:             name,
		RequirePrivilege: EveryonePrivilege,
		IgnoreMute:       true,
		Handler:          handler,
	}
}

// WithRequirePrivilege sets the minimum privilege to use a command and returns
// the modified copy
func (c Command) WithRequirePrivilege(privilege Privilege) Command {
	c.RequirePrivilege = privilege
	return c
}

// WithIgnoreMute sets whether the command works regardless of the bot being
// muted
func (c Command) WithIgnoreMute(ignoreMute bool) Command {
	c.IgnoreMute = ignoreMute
	return c
}

// Handle executes this command
func (c Command) Handle(s *discordgo.Session, m *discordgo.MessageCreate, args []string) {
	if c.Handler != nil {
		c.Handler(s, m, args)
	}
}

// CommandLibrary handles a set of commands
type CommandLibrary struct {
	commands       map[string]Command
	prefix         string
	adminChannelID string
	botOwnerUserID string
	handledCount   int
	actedUponCount int
}

// NewCommandLibrary returns a new CommandLibrary with the specified prefix
func NewCommandLibrary(prefix, botOwnerUserID string) *CommandLibrary {
	return &CommandLibrary{
		commands:       make(map[string]Command),
		prefix:         prefix,
		botOwnerUserID: botOwnerUserID,
	}
}

// WithAdminChannel sets the admin channel for this command library (used with
// AdminPrivilege)
func (l *CommandLibrary) WithAdminChannel(channelID string) *CommandLibrary {
	l.adminChannelID = channelID
	return l
}

// SetPrefix sets the prefix for the CommandLibrary
func (l *CommandLibrary) SetPrefix(prefix string) {
	l.prefix = prefix
}

// Register registers a command in the library, replacing an existing command
// with the same name, if one exists
func (l *CommandLibrary) Register(command Command) {
	l.commands[command.Name] = command
}

// Handle attempts to handle the provided message; if it fails, it returns false
func (l *CommandLibrary) Handle(s *discordgo.Session, m *discordgo.MessageCreate, muted bool) bool {
	l.handledCount++
	args, err := shellquote.Split(m.Content)
	if err != nil {
		return false
	}

	if len(args) == 0 || !strings.HasPrefix(args[0], l.prefix) {
		return false
	}

	command, present := l.commands[strings.TrimPrefix(args[0], l.prefix)]
	if !present {
		return false
	}

	if muted && !command.IgnoreMute {
		return false
	}

	switch command.RequirePrivilege {
	case AdminPrivilege:
		if m.Author.ID != l.botOwnerUserID && !l.isAdminChannel(m.ChannelID) {
			return false
		}
	case RootPrivilege:
		if m.Author.ID != l.botOwnerUserID {
			return false
		}
	}

	command.Handle(s, m, args[1:])
	l.actedUponCount++
	return true
}

// MessagesHandled returns the number of messages handled by this CommandLibrary
func (l *CommandLibrary) MessagesHandled() int {
	return l.handledCount
}

// MessagesActedUpon returns the number of messages acted upon by this CommandLibrary
func (l *CommandLibrary) MessagesActedUpon() int {
	return l.actedUponCount
}

// Name returns the name of this message handler
func (l *CommandLibrary) Name() string {
	prefix := l.prefix
	if len(prefix) == 0 {
		prefix = "no prefix"
	}
	return "CommandLibrary (" + prefix + ")"
}

func (l *CommandLibrary) isAdminChannel(channelID string) bool {
	return l.adminChannelID != "" && channelID == l.adminChannelID
}
