package main

import (
	"github.com/yarf-framework/yarf"
	"tny.im/disturbancesmlx/resource"
)

func APIserver() {
	y := yarf.New()

	v1 := yarf.RouteGroup("/v1")

	v1.Add("/networks", new(resource.Network).WithNode(rootSqalxNode))
	v1.Add("/networks/:id", new(resource.Network).WithNode(rootSqalxNode))

	v1.Add("/lines", new(resource.Line).WithNode(rootSqalxNode))
	v1.Add("/lines/:id", new(resource.Line).WithNode(rootSqalxNode))

	v1.Add("/stations", new(resource.Station).WithNode(rootSqalxNode))
	v1.Add("/stations/:id", new(resource.Station).WithNode(rootSqalxNode))

	v1.Add("/disturbances", new(resource.Disturbance).WithNode(rootSqalxNode))
	v1.Add("/disturbances/:id", new(resource.Disturbance).WithNode(rootSqalxNode))

	v1.Add("/datasets", new(resource.Dataset).WithNode(rootSqalxNode).WithSquirrel(&sdb))
	v1.Add("/datasets/:id", new(resource.Dataset).WithNode(rootSqalxNode).WithSquirrel(&sdb))

	y.AddGroup(v1)

	y.Logger = webLog
	y.Silent = true
	y.Start(":12000")
}
