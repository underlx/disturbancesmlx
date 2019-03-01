package ankiddie

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"time"

	"github.com/gbl08ma/monkey"

	uuid "github.com/satori/go.uuid"
	"github.com/underlx/disturbancesmlx/dataobjects"

	"github.com/gbl08ma/anko/core"
	"github.com/gbl08ma/anko/packages"
	"github.com/gbl08ma/anko/vm"
)

// ErrAlreadySuspended when an environment is already suspended
var ErrAlreadySuspended = errors.New("Environment already suspended")

// ErrAlreadyStarted when an environment had already been started
var ErrAlreadyStarted = errors.New("Environment already started")

// Environment is a anko environment managed by Ankiddie
type Environment struct {
	ssys      *Ankiddie
	eid       uint
	vm        *vm.Env
	ctx       context.Context
	cancel    context.CancelFunc
	started   bool
	suspended bool
	src       string
	srcDirty  bool
	scriptID  string
}

func (ssys *Ankiddie) newEnv(eid uint, code string, out func(env *Environment, msg string) error) *Environment {
	env := &Environment{
		ssys:      ssys,
		eid:       eid,
		src:       code,
		suspended: true,
		vm:        vm.NewEnv(),
	}
	packages.DefineImport(env.vm)
	core.Import(env.vm)

	if out == nil {
		out = func(env *Environment, msg string) error { return nil }
	}

	env.vm.Define("println", func(a ...interface{}) (n int, err error) {
		msg := fmt.Sprintln(a...)
		return len(msg), out(env, msg)
	})

	env.vm.Define("print", func(a ...interface{}) (n int, err error) {
		msg := fmt.Sprint(a...)
		return len(msg), out(env, msg)
	})

	env.vm.Define("printf", func(format string, a ...interface{}) (n int, err error) {
		msg := fmt.Sprintf(format, a...)
		return len(msg), out(env, msg)
	})

	env.vm.Define("strengthen", env.makeStrengthenFunction())
	env.vm.Define("ptr", func(obj interface{}) interface{} {
		val := reflect.ValueOf(obj)
		vp := reflect.New(val.Type())
		vp.Elem().Set(val)
		return vp.Interface()
	})
	env.vm.Define("error", reflect.Zero(reflect.TypeOf((*error)(nil)).Elem()).Interface())
	// TODO inspect might not be really needed, as core.Import already defines typeOf
	env.vm.Define("inspect", func(obj interface{}) string {
		t := reflect.TypeOf(obj)
		if t != nil {
			return t.String()
		}
		return "nil"
	})

	// monkey patching support
	env.vm.Define("monkeyPatch", func(target interface{}, replacement interface{}) *monkey.PatchGuard {
		tt := reflect.TypeOf(target)
		if tt.Kind() == reflect.Func {
			args := []reflect.Type{}
			for i := 0; i < tt.NumIn(); i++ {
				args = append(args, tt.In(i))
			}
			if tt.NumOut() > 0 {
				args = append(args, nil)
			}
			for i := 0; i < tt.NumOut(); i++ {
				args = append(args, tt.Out(i))
			}
			replacement = ankoStrengthenWithTypes(env.ctx, replacement, args)
		}
		return monkey.Patch(target, replacement)
	})
	env.vm.Define("monkeyPatchTypeMethod", func(target interface{}, methodName string, replacement interface{}) *monkey.PatchGuard {
		tt := reflect.TypeOf(target)
		m, ok := tt.MethodByName(methodName)
		if !ok {
			panic(fmt.Sprintf("unknown method %s", methodName))
		}
		args := []reflect.Type{}
		for i := 0; i < m.Func.Type().NumIn(); i++ {
			args = append(args, m.Func.Type().In(i))
		}
		if m.Func.Type().NumOut() > 0 {
			args = append(args, nil)
		}
		for i := 0; i < m.Func.Type().NumOut(); i++ {
			args = append(args, m.Func.Type().Out(i))
		}
		replacement = ankoStrengthenWithTypes(env.ctx, replacement, args)

		return monkey.PatchInstanceMethod(tt, methodName, replacement)
	})
	env.vm.Define("monkeyUnpatch", monkey.Unpatch)
	env.vm.Define("monkeyUnpatchTypeMethod", func(target interface{}, methodName string) bool {
		return monkey.UnpatchInstanceMethod(reflect.TypeOf(target), methodName)
	})
	env.vm.Define("monkeyUnpatchAll", monkey.UnpatchAll)

	return env
}

func (env *Environment) makeStrengthenFunction() func(fn interface{}, argsForTypes ...interface{}) interface{} {
	return func(fn interface{}, argsForTypes ...interface{}) interface{} {
		env.ssys.m.Lock()
		r := ankoStrengthen(env.ctx, fn, argsForTypes)
		env.ssys.m.Unlock()
		return r
	}
}

// Start parses and runs the source associated with the environment
func (env *Environment) Start() (interface{}, error) {
	env.ssys.m.Lock()
	if env.started {
		env.ssys.m.Unlock()
		return nil, ErrAlreadyStarted
	}
	env.started = true
	env.suspended = false
	env.ctx, env.cancel = context.WithCancel(context.Background())
	env.ssys.m.Unlock()
	return env.vm.ExecuteContext(env.ctx, env.src)
}

// Suspend stops the execution on the environment without destroying its state
func (env *Environment) Suspend() error {
	env.ssys.m.Lock()
	defer env.ssys.m.Unlock()

	if env.suspended {
		return ErrAlreadySuspended
	}

	env.cancel()
	env.suspended = true
	return nil
}

// Restart restarts the execution on the environment
func (env *Environment) Restart() (interface{}, error) {
	env.ssys.m.Lock()
	env.cancel()
	env.suspended = false
	env.started = true
	env.ctx, env.cancel = context.WithCancel(context.Background())
	env.ssys.m.Unlock()
	return env.vm.ExecuteContext(env.ctx, env.src)
}

// Execute parses and runs source in current scope
func (env *Environment) Execute(source string, appendToSrc bool) (interface{}, error) {
	env.ssys.m.Lock()
	if appendToSrc {
		env.src = fmt.Sprintf("%s\n// Added on %s:\n%s", env.src, time.Now().Format(time.RFC3339), source)
		env.srcDirty = true
	}
	if env.suspended || !env.started {
		env.ctx, env.cancel = context.WithCancel(context.Background())
	}
	env.started = true
	env.suspended = false
	env.ssys.m.Unlock()
	return env.vm.ExecuteContext(env.ctx, source)
}

// Forget stops execution of the given environment as far as possible and unregisters it
func (env *Environment) Forget() {
	env.ssys.ForgetEnv(env)
}

// SaveScript saves the script to the database under the specified ID
// If no ID is provided, the script is saved under its original ID
// If the script did not have an ID associated, a UUID is generated
func (env *Environment) SaveScript(id string) (*dataobjects.Script, error) {
	tx, err := env.ssys.node.Beginx()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	env.ssys.m.Lock()
	defer env.ssys.m.Unlock()

	if id == "" {
		id = env.scriptID
		if id == "" {
			uid, err := uuid.NewV4()
			if err != nil {
				return nil, err
			}
			id = uid.String()
		}
	}

	script, err := dataobjects.GetScript(tx, id)
	if err != nil {
		script = &dataobjects.Script{
			ID:      id,
			Autorun: -1,
		}
	}

	script.Type = scriptType
	script.Code = env.src

	err = script.Update(tx)
	if err != nil {
		return nil, err
	}

	tx.DeferToCommit(func() {
		env.ssys.m.Lock()
		env.srcDirty = false
		env.ssys.m.Unlock()
	})
	return script, tx.Commit()
}

// EID returns the environment ID
func (env *Environment) EID() uint {
	return env.eid
}

// ScriptID returns the script ID associated with this environment
func (env *Environment) ScriptID() string {
	return env.scriptID
}

// Dirty returns whether the code associated with this environment has had changes
// since the environment was created
func (env *Environment) Dirty() bool {
	return env.srcDirty
}

// Started returns whether execution has ever started in this environment
func (env *Environment) Started() bool {
	return env.started
}

// Suspended returns whether execution is suspended in this environment
func (env *Environment) Suspended() bool {
	return env.suspended
}