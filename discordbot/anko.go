package discordbot

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"reflect"
	"strconv"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/gbl08ma/ankiddie"
	"github.com/gbl08ma/sqalx"
	altmath "github.com/pkg/math"
	"github.com/underlx/disturbancesmlx/types"
)

// ScriptSystem handles scripting-related commands
type ScriptSystem struct {
	node sqalx.Node
}

// Setup registers script-related commands
func (ssys *ScriptSystem) Setup(node sqalx.Node, cl *CommandLibrary, privilege Privilege) {
	ssys.node = node
	cl.Register(NewCommand("ankorun", ssys.handleRun).WithSkipArgParsing(true).WithRequirePrivilege(privilege))
	cl.Register(NewCommand("ankorunscript", ssys.handleRunScript).WithRequirePrivilege(privilege))
	cl.Register(NewCommand("ankosuspend", ssys.handleSuspend).WithRequirePrivilege(privilege))
	cl.Register(NewCommand("ankorestart", ssys.handleRestart).WithRequirePrivilege(privilege))
	cl.Register(NewCommand("ankorunon", ssys.handleRunOn).WithSkipArgParsing(true).WithRequirePrivilege(privilege))
	cl.Register(NewCommand("ankostop", ssys.handleStop).WithRequirePrivilege(privilege))
	cl.Register(NewCommand("ankoclear", ssys.handleClear).WithRequirePrivilege(privilege))
	cl.Register(NewCommand("ankostatus", ssys.handleStatus).WithRequirePrivilege(privilege))
	cl.Register(NewCommand("ankosavescript", ssys.handleSaveScript).WithRequirePrivilege(privilege))
	cl.Register(NewCommand("ankouploadscript", ssys.handleUploadScript).WithRequirePrivilege(privilege))
	cl.Register(NewCommand("ankodownloadscript", ssys.handleDownloadScript).WithRequirePrivilege(privilege))
	cl.Register(NewCommand("ankolistscripts", ssys.handleListScripts).WithRequirePrivilege(privilege))
	cl.Register(NewCommand("ankoautorunscript", ssys.handleAutorunScript).WithRequirePrivilege(privilege))
}

// BuildAnkoOutFunction returns a function suitable for passing to Ankiddie as the output function of anko environments,
// that sends messages to a discord channel
func BuildAnkoOutFunction(channelID string) func(env *ankiddie.Environment, msg string) error {
	return func(env *ankiddie.Environment, msg string) error {
		msg = fmt.Sprintf("`%d` %s", env.EID(), msg)
		_, err := session.ChannelMessageSend(channelID, msg[:altmath.Min(len(msg), 2000)])
		return err
	}
}

func (ssys *ScriptSystem) handleRun(s *discordgo.Session, m *discordgo.MessageCreate, args []string) {
	script := strings.Replace(args[0], "```", "", -1)

	env := cmdReceiver.GetAnkiddie().NewEnvWithCode(script, BuildAnkoOutFunction(m.ChannelID))
	s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("▶💠 %d", env.EID()))
	_, err := env.Start()
	if err != nil {
		s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("❌💠 %d: %s", env.EID(), err.Error()))
		return
	}
	s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("🏃💠 %d", env.EID()))
}

func (ssys *ScriptSystem) handleRunScript(s *discordgo.Session, m *discordgo.MessageCreate, args []string) {
	if len(args) < 1 {
		s.ChannelMessageSend(m.ChannelID, "🆖 missing arguments")
		return
	}

	tx, err := ssys.node.Beginx()
	if err != nil {
		s.ChannelMessageSend(m.ChannelID, "❌ "+err.Error())
		return
	}
	defer tx.Commit() // read-only tx

	script, err := types.GetScript(tx, args[0])
	if err != nil {
		s.ChannelMessageSend(m.ChannelID, "🆖📜🆔")
		return
	}
	ascript := ankiddie.Script(*script)
	env := cmdReceiver.GetAnkiddie().NewEnvWithScript(&ascript, BuildAnkoOutFunction(m.ChannelID))
	s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("▶💠 %d", env.EID()))
	_, err = env.Start()
	if err != nil {
		s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("❌💠 %d: %s", env.EID(), err.Error()))
		return
	}
	s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("🏃💠 %d", env.EID()))
}

func (ssys *ScriptSystem) handleSuspend(s *discordgo.Session, m *discordgo.MessageCreate, args []string) {
	if len(args) < 1 {
		s.ChannelMessageSend(m.ChannelID, "🆖 missing arguments")
		return
	}
	envID, err := strconv.ParseUint(args[0], 10, 32)
	if err != nil {
		s.ChannelMessageSend(m.ChannelID, "🆖💠🆔")
		return
	}
	env, ok := cmdReceiver.GetAnkiddie().Environment(uint(envID))
	if !ok {
		s.ChannelMessageSend(m.ChannelID, "🆖💠🆔")
		return
	}
	err = env.Suspend()
	if err != nil {
		s.ChannelMessageSend(m.ChannelID, "❌ "+err.Error())
		return
	}
	s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("⏸💠 %d", envID))
}

func (ssys *ScriptSystem) handleRestart(s *discordgo.Session, m *discordgo.MessageCreate, args []string) {
	if len(args) < 1 {
		s.ChannelMessageSend(m.ChannelID, "🆖 missing arguments")
		return
	}
	envID, err := strconv.ParseUint(args[0], 10, 32)
	if err != nil {
		s.ChannelMessageSend(m.ChannelID, "🆖💠🆔")
		return
	}
	env, ok := cmdReceiver.GetAnkiddie().Environment(uint(envID))
	if !ok {
		s.ChannelMessageSend(m.ChannelID, "🆖💠🆔")
		return
	}
	_, err = env.Restart()
	if err != nil {
		s.ChannelMessageSend(m.ChannelID, "❌ "+err.Error())
		return
	}
	s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("🏃💠%d", envID))
}

func (ssys *ScriptSystem) handleRunOn(s *discordgo.Session, m *discordgo.MessageCreate, args []string) {
	if len(args) < 1 {
		s.ChannelMessageSend(m.ChannelID, "🆖 missing arguments")
		return
	}
	words := strings.Fields(args[0])
	envID, err := strconv.ParseUint(words[0], 10, 32)
	if err != nil {
		s.ChannelMessageSend(m.ChannelID, "🆖💠🆔")
		return
	}

	env, ok := cmdReceiver.GetAnkiddie().Environment(uint(envID))
	if !ok {
		s.ChannelMessageSend(m.ChannelID, "🆖💠🆔")
		return
	}

	startLen := altmath.MinInt(len(args[0]), len(words[0])+1)
	script := strings.Replace(args[0][startLen:], "```", "", -1)

	s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("▶💠 %d", envID))

	_, err = env.Execute(script, false)
	if err != nil {
		s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("❌💠 %d: %s", envID, err.Error()))
		return
	}
	s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("🏃💠 %d", envID))
}

func (ssys *ScriptSystem) handleStop(s *discordgo.Session, m *discordgo.MessageCreate, args []string) {
	if len(args) < 1 {
		s.ChannelMessageSend(m.ChannelID, "🆖 missing arguments")
		return
	}
	envID, err := strconv.ParseUint(args[0], 10, 32)
	if err != nil {
		s.ChannelMessageSend(m.ChannelID, "🆖💠🆔")
		return
	}
	env, ok := cmdReceiver.GetAnkiddie().Environment(uint(envID))
	if !ok {
		s.ChannelMessageSend(m.ChannelID, "🆖💠🆔")
		return
	}
	env.Forget()
	s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("🛑💠 %d", envID))
}

func (ssys *ScriptSystem) handleClear(s *discordgo.Session, m *discordgo.MessageCreate, args []string) {
	cmdReceiver.GetAnkiddie().FullReset()
	s.ChannelMessageSend(m.ChannelID, "🗑✅")
}

func (ssys *ScriptSystem) handleStatus(s *discordgo.Session, m *discordgo.MessageCreate, args []string) {
	message := "```Environment | Status | Script\n-----------------------------\n"
	envs := cmdReceiver.GetAnkiddie().Environments()
	for envID, env := range envs {
		status := "▶"
		if !env.Started() {
			status = "🎬"
		} else if env.Suspended() {
			status = "⏸"
		}
		script := env.ScriptID()
		if env.Dirty() {
			script += "*"
		}
		message += fmt.Sprintf("%11d | %6s | %s\n", envID, status, script)
	}
	message += "```"
	if len(envs) == 0 {
		message = "No alive environments"
	}
	s.ChannelMessageSend(m.ChannelID, message)
}

func (ssys *ScriptSystem) handleSaveScript(s *discordgo.Session, m *discordgo.MessageCreate, args []string) {
	if len(args) < 1 {
		s.ChannelMessageSend(m.ChannelID, "🆖 missing arguments")
		return
	}
	envID, err := strconv.ParseUint(args[0], 10, 32)
	if err != nil {
		s.ChannelMessageSend(m.ChannelID, "🆖💠🆔")
		return
	}
	env, ok := cmdReceiver.GetAnkiddie().Environment(uint(envID))
	if !ok {
		s.ChannelMessageSend(m.ChannelID, "🆖💠🆔")
		return
	}
	scriptID := ""
	if len(args) > 1 {
		scriptID = args[1]
	}
	script, err := env.SaveScript(scriptID)
	if err != nil {
		s.ChannelMessageSend(m.ChannelID, "❌ "+err.Error())
		return
	}
	s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("💾📜🆔`%s`", script.ID))
}

func (ssys *ScriptSystem) handleUploadScript(s *discordgo.Session, m *discordgo.MessageCreate, args []string) {
	if len(m.Attachments) < 1 {
		s.ChannelMessageSend(m.ChannelID, "🆖 missing attachments")
		return
	}
	for i, attachment := range m.Attachments {
		response, err := netClient.Get(attachment.URL)
		if err != nil {
			s.ChannelMessageSend(m.ChannelID, "❌ "+err.Error())
			return
		}
		content, err := ioutil.ReadAll(response.Body)
		if err != nil {
			s.ChannelMessageSend(m.ChannelID, "❌ "+err.Error())
			return
		}
		response.Body.Close()

		contentType := http.DetectContentType(content)
		if !strings.HasPrefix(contentType, "text/plain") {
			s.ChannelMessageSend(m.ChannelID, "❌📎 not plain text")
			return
		}

		id := ""
		if i < len(args) {
			id = args[i]
		} else {
			id = attachment.Filename
			if strings.HasSuffix(id, ".anko") && len(id) > 5 {
				id = id[0 : len(id)-5]
			}
		}

		script, err := cmdReceiver.GetAnkiddie().SaveScript(id, string(content))
		if err != nil {
			s.ChannelMessageSend(m.ChannelID, "❌ "+err.Error())
			return
		}
		s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("💾📜🆔`%s`", script.ID))
	}
}

func (ssys *ScriptSystem) handleDownloadScript(s *discordgo.Session, m *discordgo.MessageCreate, args []string) {
	if len(args) < 1 {
		s.ChannelMessageSend(m.ChannelID, "🆖 missing arguments")
		return
	}
	tx, err := ssys.node.Beginx()
	if err != nil {
		s.ChannelMessageSend(m.ChannelID, "❌ "+err.Error())
		return
	}
	defer tx.Commit() // read-only tx

	files := []*discordgo.File{}
	content := ""
	for _, scriptName := range args {
		asTXT := strings.HasSuffix(scriptName, ".txt") && len(scriptName) > 4
		if asTXT {
			scriptName = scriptName[0 : len(scriptName)-4]
		}

		script, err := types.GetScript(tx, scriptName)
		if err != nil {
			content += "🆖📜🆔 `" + scriptName + "`\n"
			continue
		}

		name := script.ID + ".anko"
		if asTXT {
			name = script.ID + ".txt"
		}

		files = append(files, &discordgo.File{
			Name:        name,
			ContentType: "text/plain",
			Reader:      strings.NewReader(script.Code),
		})
	}

	if len(files) > 0 {
		content += "✅"
		s.ChannelMessageSendComplex(m.ChannelID, &discordgo.MessageSend{
			Content: content,
			Files:   files,
		})
	} else {
		s.ChannelMessageSend(m.ChannelID, content)
	}
}

func (ssys *ScriptSystem) handleListScripts(s *discordgo.Session, m *discordgo.MessageCreate, args []string) {
	tx, err := ssys.node.Beginx()
	if err != nil {
		s.ChannelMessageSend(m.ChannelID, "❌ "+err.Error())
		return
	}
	defer tx.Commit() // read-only tx
	new(SQLSystem).runOnTx(s, m, tx, "SELECT id, autorun, notes FROM script WHERE type = 'anko'")
}

func (ssys *ScriptSystem) handleAutorunScript(s *discordgo.Session, m *discordgo.MessageCreate, args []string) {
	if len(args) < 1 {
		s.ChannelMessageSend(m.ChannelID, "🆖 missing arguments")
		return
	}

	tx, err := ssys.node.Beginx()
	if err != nil {
		s.ChannelMessageSend(m.ChannelID, "❌ "+err.Error())
		return
	}
	defer tx.Rollback()

	script, err := types.GetScript(tx, args[0])
	if err != nil {
		s.ChannelMessageSend(m.ChannelID, "🆖📜🆔")
		return
	}

	if len(args) == 1 {
		autorun := "Autorun disabled"
		if script.Autorun >= 0 {
			autorun = fmt.Sprintf("Autorun level %d", script.Autorun)
		}
		s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("📜🆔`%s`: %s", script.ID, autorun))
		tx.Commit()
		return
	}

	autorun := int64(-1)
	if args[1] != "disabled" {
		autorun, err = strconv.ParseInt(args[1], 10, 32)
		if err != nil {
			s.ChannelMessageSend(m.ChannelID, "🆖 autorun level")
			return
		}
	}
	script.Autorun = int(autorun)
	err = script.Update(tx)
	if err != nil {
		s.ChannelMessageSend(m.ChannelID, "❌ "+err.Error())
		return
	}

	err = tx.Commit()
	if err != nil {
		s.ChannelMessageSend(m.ChannelID, "❌ "+err.Error())
		return
	}
	s.ChannelMessageSend(m.ChannelID, "✅")
}

// AnkoPackageConfigurator exports functions for use with the anko scripting system
func AnkoPackageConfigurator(packages map[string]map[string]reflect.Value, packageTypes map[string]map[string]reflect.Type) {
	if packages["underlx"] == nil {
		packages["underlx"] = make(map[string]reflect.Value)
	}
	if packageTypes["underlx"] == nil {
		packageTypes["underlx"] = make(map[string]reflect.Type)
	}

	packages["underlx"]["DiscordSession"] = reflect.ValueOf(func() *discordgo.Session {
		return session
	})
	packages["underlx"]["StartReactionEvent"] = reflect.ValueOf(ThePosPlayBridge.StartReactionEvent)
	packages["underlx"]["StartQuizEvent"] = reflect.ValueOf(ThePosPlayBridge.StartQuizEvent)
	packages["underlx"]["StopEvent"] = reflect.ValueOf(ThePosPlayBridge.StopEvent)
}
