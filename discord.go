package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/underlx/disturbancesmlx/dataobjects"

	"github.com/underlx/disturbancesmlx/discordbot"
)

// DiscordBot starts the Discord bot if it is enabled in the settings
func DiscordBot() {
	discordToken, present := secrets.Get("discordToken")
	if !present {
		discordLog.Println("Discord token not found, Discord functions disabled")
		return
	}

	discordAdminChannel, present := secrets.Get("discordAdminChannel")
	if !present {
		discordLog.Println("Discord admin channel ID not present")
		discordAdminChannel = ""
	}

	err := discordbot.Start(rootSqalxNode, websiteURL, discordToken,
		discordAdminChannel, discordLog, schedulesToLines, handleBotCommands)
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

func handleBotCommands(command discordbot.ParentCommand) {
	switch t := command.Command().(type) {
	case *discordbot.NewStatusCommand:
		handleNewStatusNotify(t.Status)
	case *discordbot.ControlScraperCommand:
		handleControlScraper(t)
	case *discordbot.ControlNotifsCommand:
		handleControlNotifs(t.Type, t.Enable)
	case *discordbot.ReportDisturbanceCommand:
		err := reportHandler.addReport(dataobjects.NewLineDisturbanceReportDebug(t.Line, "discord"), t.Weight)
		if err != nil {
			discordLog.Println(err)
		}
	case *discordbot.ClearDisturbanceReportsCommand:
		reportHandler.clearVotesForLine(t.Line)
	case *discordbot.GetDisturbanceReportsCommand:
		message := ""
		lines, err := dataobjects.GetLines(rootSqalxNode)
		if err != nil {
			discordLog.Println(err)
		}
		for _, line := range lines {
			message += fmt.Sprintf("`%s`: %d/%d\n", line.ID, reportHandler.countVotesForLine(line), reportHandler.getThresholdForLine(line))
		}
		t.MessageCallback(message)
	case *discordbot.ReportThresholdMultiplierCommand:
		if t.Set {
			reportHandler.SetThresholdMultiplier(t.Multiplier)
		} else {
			t.Multiplier = reportHandler.ThresholdMultiplier()
		}
	case *discordbot.ReportThresholdOffsetCommand:
		if t.Set {
			reportHandler.SetThresholdOffset(t.Offset)
		} else {
			t.Offset = reportHandler.ThresholdOffset()
		}
	default:
		discordLog.Println("Unknown ParentCommand type")
	}
}
