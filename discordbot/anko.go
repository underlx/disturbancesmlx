package discordbot

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/bwmarrin/discordgo"
	altmath "github.com/pkg/math"
	"github.com/underlx/disturbancesmlx/ankiddie"
)

// ScriptSystem handles scripting-related commands
type ScriptSystem struct {
}

// Setup registers script-related commands
func (ssys *ScriptSystem) Setup(cl *CommandLibrary, privilege Privilege) {
	cl.Register(NewCommand("ankorun", ssys.handleRun).WithSkipArgParsing(true).WithRequirePrivilege(privilege))
	cl.Register(NewCommand("ankosuspend", ssys.handleSuspend).WithRequirePrivilege(privilege))
	cl.Register(NewCommand("ankorestart", ssys.handleRestart).WithRequirePrivilege(privilege))
	cl.Register(NewCommand("ankorunon", ssys.handleRunOn).WithSkipArgParsing(true).WithRequirePrivilege(privilege))
	cl.Register(NewCommand("ankostop", ssys.handleStop).WithRequirePrivilege(privilege))
	cl.Register(NewCommand("ankoclear", ssys.handleClear).WithRequirePrivilege(privilege))
	cl.Register(NewCommand("ankostatus", ssys.handleStatus).WithRequirePrivilege(privilege))
}

func (ssys *ScriptSystem) handleRun(s *discordgo.Session, m *discordgo.MessageCreate, args []string) {
	script := strings.Replace(args[0], "```", "", -1)

	outFn := func(env *ankiddie.Environment, msg string) error {
		msg = fmt.Sprintf("`%d` %s", env.EID(), msg)
		_, err := s.ChannelMessageSend(m.ChannelID, msg[:altmath.Min(len(msg), 2000)])
		return err
	}
	env := cmdReceiver.GetAnkiddie().NewEnvWithCode(script, outFn)
	s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("▶ env %d", env.EID()))
	_, err := env.Start()
	if err != nil {
		s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("❌ env %d error: %s", env.EID(), err.Error()))
		return
	}
	s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("🏃 env %d", env.EID()))
}

func (ssys *ScriptSystem) handleSuspend(s *discordgo.Session, m *discordgo.MessageCreate, args []string) {
	if len(args) < 1 {
		s.ChannelMessageSend(m.ChannelID, "🆖 missing arguments")
		return
	}
	envID, err := strconv.ParseUint(args[0], 10, 32)
	if err != nil {
		s.ChannelMessageSend(m.ChannelID, "🆖 env ID")
		return
	}
	env, ok := cmdReceiver.GetAnkiddie().Environment(uint(envID))
	if !ok {
		s.ChannelMessageSend(m.ChannelID, "🆖 env ID")
		return
	}
	err = env.Suspend()
	if err != nil {
		s.ChannelMessageSend(m.ChannelID, "❌ "+err.Error())
		return
	}
	s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("⏸ env %d", envID))
}

func (ssys *ScriptSystem) handleRestart(s *discordgo.Session, m *discordgo.MessageCreate, args []string) {
	if len(args) < 1 {
		s.ChannelMessageSend(m.ChannelID, "🆖 missing arguments")
		return
	}
	envID, err := strconv.ParseUint(args[0], 10, 32)
	if err != nil {
		s.ChannelMessageSend(m.ChannelID, "🆖 env ID")
		return
	}
	env, ok := cmdReceiver.GetAnkiddie().Environment(uint(envID))
	if !ok {
		s.ChannelMessageSend(m.ChannelID, "🆖 env ID")
		return
	}
	_, err = env.Restart()
	if err != nil {
		s.ChannelMessageSend(m.ChannelID, "❌ "+err.Error())
		return
	}
	s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("🏃 env %d", envID))
}

func (ssys *ScriptSystem) handleRunOn(s *discordgo.Session, m *discordgo.MessageCreate, args []string) {
	if len(args) < 1 {
		s.ChannelMessageSend(m.ChannelID, "🆖 missing arguments")
		return
	}
	words := strings.Fields(args[0])
	envID, err := strconv.ParseUint(words[0], 10, 32)
	if err != nil {
		s.ChannelMessageSend(m.ChannelID, "🆖 env ID")
		return
	}

	env, ok := cmdReceiver.GetAnkiddie().Environment(uint(envID))
	if !ok {
		s.ChannelMessageSend(m.ChannelID, "🆖 env ID")
		return
	}

	startLen := altmath.MinInt(len(args[0]), len(words[0])+1)
	script := strings.Replace(args[0][startLen:], "```", "", -1)

	s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("▶ env %d", envID))

	_, err = env.Execute(script, false)
	if err != nil {
		s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("❌ env %d execute error: %s", envID, err.Error()))
		return
	}
	s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("🏃 env %d", envID))
}

func (ssys *ScriptSystem) handleStop(s *discordgo.Session, m *discordgo.MessageCreate, args []string) {
	if len(args) < 1 {
		s.ChannelMessageSend(m.ChannelID, "🆖 missing arguments")
		return
	}
	envID, err := strconv.ParseUint(args[0], 10, 32)
	if err != nil {
		s.ChannelMessageSend(m.ChannelID, "🆖 env ID")
		return
	}
	env, ok := cmdReceiver.GetAnkiddie().Environment(uint(envID))
	if !ok {
		s.ChannelMessageSend(m.ChannelID, "🆖 env ID")
		return
	}
	env.Forget()
	s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("🛑 env %d", envID))
}

func (ssys *ScriptSystem) handleClear(s *discordgo.Session, m *discordgo.MessageCreate, args []string) {
	cmdReceiver.GetAnkiddie().FullReset()
	s.ChannelMessageSend(m.ChannelID, "🗑✅")
}

func (ssys *ScriptSystem) handleStatus(s *discordgo.Session, m *discordgo.MessageCreate, args []string) {
	message := "```Environment | Status\n--------------------\n"
	envs := cmdReceiver.GetAnkiddie().Environments()
	for envID, env := range envs {
		status := "▶"
		if env.Suspended() {
			status = "⏸"
		}
		message += fmt.Sprintf("%11d | %s\n", envID, status)
	}
	message += "```"
	if len(envs) == 0 {
		message = "No alive environments"
	}
	s.ChannelMessageSend(m.ChannelID, message)
}

// AnkoPackageConfigurator exports functions for use with the anko scripting system
func AnkoPackageConfigurator(packages, packageTypes map[string]map[string]interface{}) {
	if packages["underlx"] == nil {
		packages["underlx"] = make(map[string]interface{})
	}
	if packageTypes["underlx"] == nil {
		packageTypes["underlx"] = make(map[string]interface{})
	}

	packages["underlx"]["DiscordSession"] = func() *discordgo.Session {
		return session
	}
	packages["underlx"]["BotStats"] = func() *stats {
		return &botstats
	}
	packages["underlx"]["MessageHandlers"] = func() []MessageHandler {
		return messageHandlers
	}
	packages["underlx"]["ReactionHandlers"] = func() []ReactionHandler {
		return reactionHandlers
	}
	packages["underlx"]["StartReactionEvent"] = ThePosPlayBridge.StartReactionEvent
	packages["underlx"]["StartQuizEvent"] = ThePosPlayBridge.StartQuizEvent
	packages["underlx"]["StopEvent"] = ThePosPlayBridge.StopEvent

	packages["discordgo"] = make(map[string]interface{})
	dopkg := packages["discordgo"]
	for name, function := range DiscordGoFunctions {
		if function.CanInterface() {
			dopkg[name] = function.Interface()
		}
	}
	for name, item := range DiscordGoConsts {
		dopkg[name] = item
	}
	for name, item := range DiscordGoVariables {
		dopkg[name] = item
	}
	packageTypes["discordgo"] = make(map[string]interface{})
	dotypes := packageTypes["discordgo"]
	for name, item := range DiscordGoTypes {
		dotypes[name] = item
	}
}
