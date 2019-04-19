package main

import (
	"reflect"

	"github.com/gbl08ma/sqalx"
	"github.com/underlx/disturbancesmlx/ankiddie"
	"github.com/underlx/disturbancesmlx/compute"
	"github.com/underlx/disturbancesmlx/dataobjects"
	"github.com/underlx/disturbancesmlx/discordbot"
	"github.com/underlx/disturbancesmlx/mqttgateway"
	"github.com/underlx/disturbancesmlx/posplay"
	"github.com/underlx/disturbancesmlx/resource"
	"github.com/underlx/disturbancesmlx/utils"
	"github.com/underlx/disturbancesmlx/website"
)

func ankoPackageConfigurator(packages, packageTypes map[string]map[string]interface{}) {
	type pkgInfo struct {
		Name      string
		Types     map[string]reflect.Type
		Functions map[string]reflect.Value
		Consts    map[string]reflect.Value
		Variables map[string]reflect.Value
	}

	processPkg := func(pkg pkgInfo) {
		packages[pkg.Name] = make(map[string]interface{})
		dopkg := packages[pkg.Name]
		for name, function := range pkg.Functions {
			if function.CanInterface() {
				dopkg[name] = function.Interface()
			}
		}
		for name, item := range pkg.Consts {
			if item.CanInterface() {
				dopkg[name] = item.Interface()
			}
		}
		for name, item := range pkg.Variables {
			if item.CanInterface() {
				if item.Kind() == reflect.Ptr {
					item = item.Elem()
				}
				dopkg[name] = item.Interface()
			}
		}
		packageTypes[pkg.Name] = make(map[string]interface{})
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

	packages["underlx"]["RootSqalxNode"] = func() sqalx.Node {
		return rootSqalxNode
	}
	packages["underlx"]["VehicleHandler"] = func() *compute.VehicleHandler {
		return vehicleHandler
	}
	packages["underlx"]["StatsHandler"] = func() *compute.StatsHandler {
		return statsHandler
	}
	packages["underlx"]["ReportHandler"] = func() *compute.ReportHandler {
		return reportHandler
	}
	packages["underlx"]["MQTTGateway"] = func() *mqttgateway.MQTTGateway {
		return mqttGateway
	}

	discordbot.AnkoPackageConfigurator(packages, packageTypes)
}

func defaultAnkoOut(env *ankiddie.Environment, msg string) error {
	mainLog.Printf("[AnkoEnv%d] %s", env.EID(), msg)
	return nil
}
