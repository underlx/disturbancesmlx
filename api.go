package main

import (
	"github.com/yarf-framework/yarf"
	"tny.im/disturbancesmlx/resource"
)

func APIserver() {
	y := yarf.New()

	y.Add("/networks", new(resource.Network).WithNode(rootSqalxNode))
	y.Add("/networks/:id", new(resource.Network).WithNode(rootSqalxNode))

	y.Add("/lines", new(resource.Line).WithNode(rootSqalxNode))
	y.Add("/lines/:id", new(resource.Line).WithNode(rootSqalxNode))

	y.Add("/stations", new(resource.Station).WithNode(rootSqalxNode))
	y.Add("/stations/:id", new(resource.Station).WithNode(rootSqalxNode))

	y.Add("/disturbances", new(resource.Disturbance).WithNode(rootSqalxNode))
	y.Add("/disturbances/:id", new(resource.Disturbance).WithNode(rootSqalxNode))

	y.Logger = webLog
	y.Silent = true
	y.Start(":12000")
}
