package discordbot

import "github.com/bwmarrin/discordgo"

// A MessageHandler can handle Discord messages
type MessageHandler interface {
	// should return true if no more handlers should run (i.e. the message was handled for good)
	HandleMessage(s *discordgo.Session, m *discordgo.MessageCreate, muted bool) bool
	MessagesHandled() int
	MessagesActedUpon() int
	Name() string
}
