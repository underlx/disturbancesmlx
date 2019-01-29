package ankiddie

import (
	"sync"

	"github.com/underlx/disturbancesmlx/dataobjects"

	"github.com/gbl08ma/anko/packages"
	"github.com/gbl08ma/anko/vm"
	"github.com/gbl08ma/sqalx"
)

// Ankiddie manages the execution of anko scripts
type Ankiddie struct {
	m     sync.Mutex
	node  sqalx.Node
	envs  map[uint]*Environment
	curID uint
}

const scriptType = "anko"

// New returns a new Ankiddie
func New(node sqalx.Node, packageConfigurator func(packages, packageTypes map[string]map[string]interface{})) *Ankiddie {
	ankiddie := &Ankiddie{
		node: node,
		envs: make(map[uint]*Environment),
	}

	if packageConfigurator != nil {
		packageConfigurator(packages.Packages, packages.PackageTypes)
	}
	return ankiddie
}

// NewEnvWithCode returns a new Environment ready to run the provided code
func (ssys *Ankiddie) NewEnvWithCode(code string, out func(env *Environment, msg string) error) *Environment {
	ssys.m.Lock()
	defer ssys.m.Unlock()
	env := ssys.newEnv(ssys.curID, code, out)
	ssys.envs[env.eid] = env
	ssys.curID++
	return env
}

// NewEnvWithScript returns a new Environment ready to run the provided dataobjects.Script
func (ssys *Ankiddie) NewEnvWithScript(script *dataobjects.Script, out func(env *Environment, msg string) error) *Environment {
	ssys.m.Lock()
	defer ssys.m.Unlock()
	env := ssys.newEnv(ssys.curID, script.Code, out)
	env.scriptID = script.ID
	ssys.envs[env.eid] = env
	ssys.curID++
	return env
}

// Environment returns the environment with the given ID, if one exists
func (ssys *Ankiddie) Environment(eid uint) (*Environment, bool) {
	ssys.m.Lock()
	defer ssys.m.Unlock()
	env, ok := ssys.envs[eid]
	return env, ok
}

// Environments returns a map with the currently registered environments
func (ssys *Ankiddie) Environments() map[uint]*Environment {
	ssys.m.Lock()
	defer ssys.m.Unlock()
	envscopy := make(map[uint]*Environment)
	for key, env := range ssys.envs {
		envscopy[key] = env
	}
	return envscopy
}

// ForgetEnv stops execution of the given environment as far as possible and unregisters it
func (ssys *Ankiddie) ForgetEnv(env *Environment) {
	ssys.m.Lock()
	defer ssys.m.Unlock()
	vm.Interrupt(env.vm)
	delete(ssys.envs, env.eid)
}

// FullReset stops execution on all environments and destroys them
func (ssys *Ankiddie) FullReset() {
	ssys.m.Lock()
	defer ssys.m.Unlock()
	for _, env := range ssys.envs {
		vm.Interrupt(env.vm)
	}
	ssys.envs = make(map[uint]*Environment)
}

// StartAutorun executes scripts at the specified autorun level
func (ssys *Ankiddie) StartAutorun(level int, async bool, out func(env *Environment, msg string) error) error {
	tx, err := ssys.node.Beginx()
	if err != nil {
		return err
	}
	defer tx.Commit() // read-only tx

	scripts, err := dataobjects.GetAutorunScriptsWithType(tx, scriptType, level)
	if err != nil {
		return err
	}

	for _, script := range scripts {
		env := ssys.NewEnvWithScript(script, out)
		if async {
			go env.Start()
		} else {
			env.Start()
		}
	}
	return nil
}
