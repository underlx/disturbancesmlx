package discordbot

import "github.com/bwmarrin/discordgo"

// A ReactionHandler can handle Discord reactions
type ReactionHandler interface {
	// should return true if no more handlers should run (i.e. the reaction was handled for good)
	HandleReaction(s *discordgo.Session, m *discordgo.MessageReactionAdd) bool
	ReactionsHandled() int
	ReactionsActedUpon() int
	Name() string
}
