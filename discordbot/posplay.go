package discordbot

import (
	"errors"

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
