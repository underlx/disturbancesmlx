package main

import (
	"github.com/gbl08ma/sqalx"
	uuid "github.com/satori/go.uuid"
	"github.com/underlx/disturbancesmlx/ankiddie"
	"github.com/underlx/disturbancesmlx/compute"
	"github.com/underlx/disturbancesmlx/dataobjects"
	"github.com/underlx/disturbancesmlx/discordbot"
)

func ankoPackageConfigurator(packages, packageTypes map[string]map[string]interface{}) {
	packages["underlx"] = make(map[string]interface{})
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

	packages["compute"] = make(map[string]interface{})
	packages["compute"]["AverageSpeed"] = compute.AverageSpeed
	packages["compute"]["AverageSpeedFilter"] = compute.AverageSpeedFilter
	packages["compute"]["AverageSpeedCached"] = compute.AverageSpeedCached
	packages["compute"]["UpdateTypicalSeconds"] = compute.UpdateTypicalSeconds
	packages["compute"]["UpdateStatusMsgTypes"] = compute.UpdateStatusMsgTypes

	packages["dataobjects"] = make(map[string]interface{})
	dopkg := packages["dataobjects"]
	for name, function := range dataobjects.Functions {
		if function.CanInterface() {
			dopkg[name] = function.Interface()
		}
	}
	for name, item := range dataobjects.Consts {
		dopkg[name] = item
	}
	for name, item := range dataobjects.Variables {
		dopkg[name] = item
	}
	packageTypes["dataobjects"] = make(map[string]interface{})
	dotypes := packageTypes["dataobjects"]
	for name, item := range dataobjects.Types {
		dotypes[name] = item
	}

	packages["uuid"] = make(map[string]interface{})
	packages["uuid"]["V1"] = uuid.V1
	packages["uuid"]["V2"] = uuid.V2
	packages["uuid"]["V3"] = uuid.V3
	packages["uuid"]["V4"] = uuid.V4
	packages["uuid"]["V5"] = uuid.V5
	packages["uuid"]["VariantNCS"] = uuid.VariantNCS
	packages["uuid"]["VariantRFC4122"] = uuid.VariantRFC4122
	packages["uuid"]["VariantMicrosoft"] = uuid.VariantMicrosoft
	packages["uuid"]["VariantFuture"] = uuid.VariantFuture
	packages["uuid"]["DomainGroup"] = uuid.DomainGroup
	packages["uuid"]["DomainOrg"] = uuid.DomainOrg
	packages["uuid"]["DomainPerson"] = uuid.DomainPerson
	packages["uuid"]["Size"] = uuid.Size
	packages["uuid"]["NamespaceDNS"] = uuid.NamespaceDNS
	packages["uuid"]["NamespaceOID"] = uuid.NamespaceOID
	packages["uuid"]["NamespaceURL"] = uuid.NamespaceURL
	packages["uuid"]["NamespaceX500"] = uuid.NamespaceX500
	packages["uuid"]["Nil"] = uuid.Nil
	packages["uuid"]["Equal"] = uuid.Equal
	packages["uuid"]["FromBytes"] = uuid.FromBytes
	packages["uuid"]["FromBytesOrNil"] = uuid.FromBytesOrNil
	packages["uuid"]["FromString"] = uuid.FromString
	packages["uuid"]["FromStringOrNil"] = uuid.FromStringOrNil
	packages["uuid"]["Must"] = uuid.Must
	packages["uuid"]["NewV1"] = uuid.NewV1
	packages["uuid"]["NewV2"] = uuid.NewV2
	packages["uuid"]["NewV3"] = uuid.NewV3
	packages["uuid"]["NewV4"] = uuid.NewV4
	packages["uuid"]["NewV5"] = uuid.NewV5
	packageTypes["uuid"] = make(map[string]interface{})
	packageTypes["uuid"]["NullUUID"] = uuid.NullUUID{}
	packageTypes["uuid"]["UUID"] = uuid.UUID{}

	discordbot.AnkoPackageConfigurator(packages, packageTypes)
}

func defaultAnkoOut(env *ankiddie.Environment, msg string) error {
	mainLog.Printf("[AnkoEnv%d] %s", env.EID(), msg)
	return nil
}
