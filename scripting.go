package main

import (
	"reflect"

	"github.com/underlx/disturbancesmlx/scraper"

	"github.com/gbl08ma/ankiddie"
	"github.com/gbl08ma/sqalx"
	"github.com/underlx/disturbancesmlx/compute"
	"github.com/underlx/disturbancesmlx/dataobjects"
	"github.com/underlx/disturbancesmlx/discordbot"
	"github.com/underlx/disturbancesmlx/mqttgateway"
	"github.com/underlx/disturbancesmlx/posplay"
	"github.com/underlx/disturbancesmlx/resource"
	"github.com/underlx/disturbancesmlx/utils"
	"github.com/underlx/disturbancesmlx/website"

	_ "github.com/gbl08ma/anko/packages"
)

type ankoInterop struct {
	node sqalx.Node
}

func (ai *ankoInterop) ConfigurePackages(packages map[string]map[string]reflect.Value, packageTypes map[string]map[string]reflect.Type) {
	type pkgInfo struct {
		Name      string
		Types     map[string]reflect.Type
		Functions map[string]reflect.Value
		Consts    map[string]reflect.Value
		Variables map[string]reflect.Value
	}

	processPkg := func(pkg pkgInfo) {
		packages[pkg.Name] = make(map[string]reflect.Value)
		dopkg := packages[pkg.Name]
		for name, function := range pkg.Functions {
			dopkg[name] = function
		}
		for name, item := range pkg.Consts {
			dopkg[name] = item
		}
		for name, item := range pkg.Variables {
			dopkg[name] = item
		}
		packageTypes[pkg.Name] = make(map[string]reflect.Type)
		dotypes := packageTypes[pkg.Name]
		for name, item := range pkg.Types {
			dotypes[name] = item
		}
	}

	processPkg(pkgInfo{"compute", compute.Types, compute.Functions, compute.Consts, compute.Variables})
	processPkg(pkgInfo{"dataobjects", dataobjects.Types, dataobjects.Functions, dataobjects.Consts, dataobjects.Variables})
	processPkg(pkgInfo{"discordbot", discordbot.Types, discordbot.Functions, discordbot.Consts, discordbot.Variables})
	processPkg(pkgInfo{"resource", resource.Types, resource.Functions, resource.Consts, resource.Variables})
	processPkg(pkgInfo{"posplay", posplay.Types, posplay.Functions, posplay.Consts, posplay.Variables})
	processPkg(pkgInfo{"utils", utils.Types, utils.Functions, utils.Consts, utils.Variables})
	processPkg(pkgInfo{"website", website.Types, website.Functions, website.Consts, website.Variables})
	processPkg(pkgInfo{"underlx", Types, Functions, Consts, Variables})

	processPkg(pkgInfo{"discordgo", extpkgDiscordGoTypes, extpkgDiscordGoFunctions, extpkgDiscordGoConsts, extpkgDiscordGoVariables})
	processPkg(pkgInfo{"uuid", extpkgUUIDTypes, extpkgUUIDFunctions, extpkgUUIDConsts, extpkgUUIDVariables})

	packages["underlx"]["RootSqalxNode"] = reflect.ValueOf(func() sqalx.Node {
		return rootSqalxNode
	})
	packages["underlx"]["VehicleHandler"] = reflect.ValueOf(func() *compute.VehicleHandler {
		return vehicleHandler
	})
	packages["underlx"]["VehicleETAHandler"] = reflect.ValueOf(func() *compute.VehicleETAHandler {
		return vehicleETAHandler
	})
	packages["underlx"]["StatsHandler"] = reflect.ValueOf(func() *compute.StatsHandler {
		return statsHandler
	})
	packages["underlx"]["ReportHandler"] = reflect.ValueOf(func() *compute.ReportHandler {
		return reportHandler
	})
	packages["underlx"]["MQTTGateway"] = reflect.ValueOf(func() *mqttgateway.MQTTGateway {
		return mqttGateway
	})
	packages["underlx"]["ContestScraper"] = reflect.ValueOf(func() scraper.AnnouncementScraper {
		return contestscr
	})

	discordbot.AnkoPackageConfigurator(packages, packageTypes)
}

func (ai *ankoInterop) GetScript(id string) (*ankiddie.Script, error) {
	script, err := dataobjects.GetScript(ai.node, id)
	if err != nil {
		return nil, err
	}
	s := ankiddie.Script(*script)
	return &s, nil
}

func (ai *ankoInterop) GetAutorunScripts(autorunLevel int) ([]*ankiddie.Script, error) {
	scripts, err := dataobjects.GetAutorunScriptsWithType(ai.node, "anko", autorunLevel)
	if err != nil {
		return []*ankiddie.Script{}, err
	}
	as := make([]*ankiddie.Script, len(scripts))
	for i, s := range scripts {
		t := ankiddie.Script(*s)
		as[i] = &t
	}
	return as, nil
}

func (ai *ankoInterop) StoreScript(script *ankiddie.Script) error {
	tx, err := ai.node.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	s := dataobjects.Script(*script)
	err = s.Update(tx)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func defaultAnkoOut(env *ankiddie.Environment, msg string) error {
	mainLog.Printf("[AnkoEnv%d] %s", env.EID(), msg)
	return nil
}
