package main

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/underlx/disturbancesmlx/discordbot"
)

// DiscordBot starts the Discord bot if it is enabled in the settings
func DiscordBot() {
	discordToken, present := secrets.Get("discordToken")
	if !present {
		discordLog.Println("Discord token not found, Discord functions disabled")
		return
	}
	err := discordbot.Start(rootSqalxNode, websiteURL, discordToken, discordLog, schedulesToLines)
	if err != nil {
		discordLog.Println(err)
		return
	}

	// Wait here until CTRL-C or other term signal is received.
	discordLog.Println("Bot is now running.  Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	//lint:ignore SA1016 contrary to what staticcheck believes, os.Kill can be trapped in some OS and has to be trapped for this to work on Windows
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-sc

	// Cleanly close down the Discord session.
	discordbot.Stop()
}
