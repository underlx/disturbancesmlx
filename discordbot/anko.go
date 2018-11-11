package discordbot

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"sync"

	"github.com/bwmarrin/discordgo"
	"github.com/gbl08ma/anko/core"
	"github.com/gbl08ma/anko/packages"
	"github.com/gbl08ma/anko/vm"
	altmath "github.com/pkg/math"
)

// ScriptSystem handles running anko scripts
type ScriptSystem struct {
	sync.Mutex
	envs      map[uint]*vm.Env
	suspended map[uint]bool
	src       map[uint]string
	curID     uint
}

// Setup initializes the ScriptSystem and configures a command library with
// script-related commands
func (ssys *ScriptSystem) Setup(cl *CommandLibrary, privilege Privilege) {
	ssys.envs = make(map[uint]*vm.Env)
	ssys.suspended = make(map[uint]bool)
	ssys.src = make(map[uint]string)
	cl.Register(NewCommand("ankorun", ssys.handleRun).WithSkipArgParsing(true).WithRequirePrivilege(privilege))
	cl.Register(NewCommand("ankosuspend", ssys.handleSuspend).WithRequirePrivilege(privilege))
	cl.Register(NewCommand("ankorestart", ssys.handleRestart).WithRequirePrivilege(privilege))
	cl.Register(NewCommand("ankorunon", ssys.handleRunOn).WithSkipArgParsing(true).WithRequirePrivilege(privilege))
	cl.Register(NewCommand("ankostop", ssys.handleStop).WithRequirePrivilege(privilege))
	cl.Register(NewCommand("ankoclear", ssys.handleClear).WithRequirePrivilege(privilege))
	cl.Register(NewCommand("ankostatus", ssys.handleStatus).WithRequirePrivilege(privilege))

	cmdReceiver.ConfigureAnkoPackage(packages.Packages, packages.PackageTypes)

	if packages.Packages["underlx"] == nil {
		packages.Packages["underlx"] = make(map[string]interface{})
	}
	if packages.PackageTypes["underlx"] == nil {
		packages.PackageTypes["underlx"] = make(map[string]interface{})
	}

	packages.Packages["underlx"]["DiscordSession"] = func() *discordgo.Session {
		return session
	}
	packages.Packages["underlx"]["BotStats"] = func() *stats {
		return &botstats
	}
	packages.Packages["underlx"]["MessageHandlers"] = func() []MessageHandler {
		return messageHandlers
	}
	packages.Packages["underlx"]["ReactionHandlers"] = func() []ReactionHandler {
		return reactionHandlers
	}
	packages.Packages["underlx"]["StartReactionEvent"] = ThePosPlayBridge.StartReactionEvent
	packages.Packages["underlx"]["StartQuizEvent"] = ThePosPlayBridge.StartQuizEvent
	packages.Packages["underlx"]["StopEvent"] = ThePosPlayBridge.StopEvent

	if packages.Packages["discordgo"] == nil {
		packages.Packages["discordgo"] = make(map[string]interface{})
	}
	if packages.PackageTypes["discordgo"] == nil {
		packages.PackageTypes["discordgo"] = make(map[string]interface{})
	}
	packages.PackageTypes["discordgo"]["Session"] = discordgo.Session{}
	packages.PackageTypes["discordgo"]["MessageCreate"] = discordgo.MessageCreate{}
	packages.PackageTypes["discordgo"]["MessageReactionAdd"] = discordgo.MessageReactionAdd{}
}

func (ssys *ScriptSystem) handleRun(s *discordgo.Session, m *discordgo.MessageCreate, args []string) {
	script := strings.Replace(args[0], "```", "", -1)

	ssys.Lock()
	env := vm.NewEnv()
	envID := ssys.curID
	ssys.envs[envID] = env
	ssys.src[envID] = script
	ssys.curID++
	ssys.Unlock()
	packages.DefineImport(env)
	core.Import(env)

	doSend := func(msg string) error {
		_, err := s.ChannelMessageSend(m.ChannelID, msg[:altmath.Min(len(msg), 2000)])
		return err
	}

	env.Define("println", func(a ...interface{}) (n int, err error) {
		msg := fmt.Sprintf("`%d` ", envID) + fmt.Sprintln(a...)
		return len(msg), doSend(msg)
	})

	env.Define("print", func(a ...interface{}) (n int, err error) {
		msg := fmt.Sprintf("`%d` ", envID) + fmt.Sprint(a...)
		return len(msg), doSend(msg)
	})

	env.Define("printf", func(format string, a ...interface{}) (n int, err error) {
		msg := fmt.Sprintf(fmt.Sprintf("`%d` %s", envID, format), a...)
		return len(msg), doSend(msg)
	})

	env.Define("strengthen", ankoStrengthen)
	env.Define("ptr", func(obj interface{}) interface{} {
		val := reflect.ValueOf(obj)
		vp := reflect.New(val.Type())
		vp.Elem().Set(val)
		return vp.Interface()
	})
	env.Define("inspect", func(obj interface{}) string {
		t := reflect.TypeOf(obj)
		if t != nil {
			return t.String()
		}
		return "nil"
	})

	s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("▶ env %d", envID))
	_, err := env.Execute(script)
	if err != nil {
		s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("❌ env %d error: %s", envID, err.Error()))
		return
	}
	s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("🏃 env %d", envID))
}

func ankoStrengthen(fn interface{}, argsForTypes ...interface{}) interface{} {
	fType := reflect.TypeOf(fn)
	if fType == nil || fType.Kind() != reflect.Func {
		return fn
	}

	ins := make([]reflect.Type, 0)
	outs := make([]reflect.Type, 0)

	i := 0
	transformReturn := false
	for ; i < fType.NumIn() && i < len(argsForTypes); i++ {
		if argsForTypes[i] == nil {
			break
		}
		ins = append(ins, reflect.TypeOf(argsForTypes[i]))
	}

	if i < len(argsForTypes) && argsForTypes[i] == nil {
		transformReturn = true
		i++
	}

	if transformReturn {
		for ; i < len(argsForTypes); i++ {
			outs = append(outs, reflect.TypeOf(argsForTypes[i]))
		}
	}

	outsCount := len(outs)
	variadic := fType.IsVariadic()
	funcType := reflect.FuncOf(ins, outs, variadic)
	transformedFunc := reflect.MakeFunc(funcType, func(in []reflect.Value) []reflect.Value {
		args := make([]reflect.Value, len(in))
		for i, arg := range in {
			// functions in anko always appear to golang as if all their arguments were reflect.Values
			// if we don't wrap args like this, Call below complains that e.g.
			// "panic: reflect: Call using *discordgo.Session as type reflect.Value"
			args[i] = reflect.ValueOf(arg)
		}
		result := reflect.ValueOf(fn).Call(args)
		// we must also convert the result, because all anko functions always return (reflect.Value, error)
		retVal := result[0].Interface().(reflect.Value)
		k := retVal.Kind()
		switch k {
		case reflect.Chan, reflect.Func, reflect.Map, reflect.Ptr:
			if retVal.IsNil() {
				return []reflect.Value{}
			}
		}
		converted := make([]reflect.Value, outsCount)
		retIfaces, ok := retVal.Interface().([]interface{})
		if !ok {
			return converted
		}
		for i := 0; i < outsCount && i < len(retIfaces); i++ {
			converted[i] = reflect.ValueOf(retIfaces[i])
		}
		return converted
	})
	return transformedFunc.Interface()
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
	ssys.Lock()
	defer ssys.Unlock()
	env := ssys.envs[uint(envID)]
	if env == nil {
		s.ChannelMessageSend(m.ChannelID, "🆖 env ID")
		return
	}
	if ssys.suspended[uint(envID)] {
		s.ChannelMessageSend(m.ChannelID, "❌ already suspended")
		return
	}

	vm.Interrupt(env)
	ssys.suspended[uint(envID)] = true
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
	ssys.Lock()
	defer ssys.Unlock()
	env := ssys.envs[uint(envID)]
	if env == nil {
		s.ChannelMessageSend(m.ChannelID, "🆖 env ID")
		return
	}
	if !ssys.suspended[uint(envID)] {
		s.ChannelMessageSend(m.ChannelID, "❌ already running")
		return
	}

	s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("🔁 env %d", envID))
	ssys.suspended[uint(envID)] = false
	vm.ClearInterrupt(env)
	_, err = env.Execute(ssys.src[uint(envID)])
	if err != nil {
		s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("❌ env %d execute error: %s", envID, err.Error()))
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

	ssys.Lock()
	defer ssys.Unlock()
	env := ssys.envs[uint(envID)]
	if env == nil {
		s.ChannelMessageSend(m.ChannelID, "🆖 env ID")
		return
	}
	if !ssys.suspended[uint(envID)] {
		s.ChannelMessageSend(m.ChannelID, "❌ already running")
		return
	}

	startLen := altmath.MinInt(len(args[0]), len(words[0])+1)
	script := strings.Replace(args[0][startLen:], "```", "", -1)

	s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("▶ env %d", envID))
	vm.ClearInterrupt(env)
	ssys.suspended[uint(envID)] = false
	ssys.src[uint(envID)] = script
	_, err = env.Execute(script)
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
	ssys.Lock()
	defer ssys.Unlock()
	env := ssys.envs[uint(envID)]
	if env == nil {
		s.ChannelMessageSend(m.ChannelID, "🆖 env ID")
		return
	}

	vm.Interrupt(env)
	delete(ssys.envs, uint(envID))
	delete(ssys.suspended, uint(envID))
	delete(ssys.src, uint(envID))
	s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("🛑 env %d", envID))
}

func (ssys *ScriptSystem) handleClear(s *discordgo.Session, m *discordgo.MessageCreate, args []string) {
	ssys.Lock()
	defer ssys.Unlock()
	for _, env := range ssys.envs {
		vm.Interrupt(env)
	}

	ssys.envs = make(map[uint]*vm.Env)
	ssys.suspended = make(map[uint]bool)
	ssys.src = make(map[uint]string)
	s.ChannelMessageSend(m.ChannelID, "🗑✅")
}

func (ssys *ScriptSystem) handleStatus(s *discordgo.Session, m *discordgo.MessageCreate, args []string) {
	ssys.Lock()
	defer ssys.Unlock()

	message := "```Environment | Status\n--------------------\n"
	for envID := range ssys.envs {
		status := "▶"
		if ssys.suspended[envID] {
			status = "⏸"
		}
		message += fmt.Sprintf("%11d | %s\n", envID, status)
	}
	message += "```"
	if len(ssys.envs) == 0 {
		message = "No alive environments"
	}
	s.ChannelMessageSend(m.ChannelID, message)
}
