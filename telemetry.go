package main

import (
	"time"

	"github.com/gbl08ma/disturbancesmlx/dataobjects"
	statsd "gopkg.in/alexcesaro/statsd.v2"
)

// APIrequestTelemetry is a channel where something should be sent whenever an API
// request is served
var APIrequestTelemetry = make(chan interface{}, 10)

// StatsSender is meant to be called as a goroutine that handles sending telemetry
// to a statsd (or compatible) server
func StatsSender() {
	statsdAddress, present := secrets.Get("statsdAddress")
	statsdPrefix, present2 := secrets.Get("statsdPrefix")
	if !present || !present2 {
		return
	}

	network, err := dataobjects.GetNetwork(rootSqalxNode, MLnetworkID)
	if err != nil {
		mainLog.Println(err)
		return
	}

	c, err := statsd.New(statsd.Address(statsdAddress), statsd.Prefix(statsdPrefix))
	if err != nil {
		// If nothing is listening on the target port, an error is returned and
		// the returned client does nothing but is still usable. So we can
		// just log the error and go on.
		mainLog.Println(err)
	}
	defer c.Close()

	ticker := time.NewTicker(1 * time.Minute)

	for {
		select {
		case <-ticker.C:
			c.Gauge("online_in_transit", statsHandler.CurrentlyOnlineInTransit(network, 0))
		case <-APIrequestTelemetry:
			c.Increment("apicalls")
		}
	}
}
