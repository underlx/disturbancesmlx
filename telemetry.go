package main

import (
	"runtime"
	"time"

	cache "github.com/patrickmn/go-cache"
	"github.com/underlx/disturbancesmlx/dataobjects"
	statsd "gopkg.in/alexcesaro/statsd.v2"
)

// APIrequestTelemetry is a channel where something should be sent whenever an API
// request is served
var APIrequestTelemetry = make(chan interface{}, 10)

// StatsSender is meant to be called as a goroutine that handles sending telemetry
// to a statsd (or compatible) server
func StatsSender() {
	telemetryKeybox, present := secrets.GetBox("telemetry")
	if !present {
		discordLog.Println("Telemetry Keybox not found, telemetry partially disabled")
		return
	}

	statsdAddress, present := telemetryKeybox.Get("statsdAddress")
	statsdPrefix, present2 := telemetryKeybox.Get("statsdPrefix")
	if !present || !present2 {
		mainLog.Fatal("statsd address/prefix not present in telemetry keybox")
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
			statsHandler.RangeNetworks(rootSqalxNode, func(n *dataobjects.Network, cache *cache.Cache) bool {
				c.Gauge("online_in_transit_"+n.ID, statsHandler.OITInNetwork(n, 0))
				return true
			})

			statsHandler.RangeLines(rootSqalxNode, func(l *dataobjects.Line, cache *cache.Cache) bool {
				c.Gauge("online_in_transit_"+l.ID, statsHandler.OITInLine(l, 0))
				c.Gauge("report_votes_"+l.ID, reportHandler.CountVotesForLine(l))
				c.Gauge("report_threshold_"+l.ID, reportHandler.GetThresholdForLine(l))
				return true
			})

			mqttStats := mqttGateway.Stats()

			c.Gauge("mqtt.current_clients", mqttStats.CurrentClients)
			c.Gauge("mqtt.current_subscriptions", mqttStats.CurrentSubscriptions)
			c.Gauge("mqtt.total_connects", mqttStats.TotalConnects)
			c.Gauge("mqtt.total_disconnects", mqttStats.TotalDisconnects)

			var m runtime.MemStats
			runtime.ReadMemStats(&m)

			c.Gauge("profiling.mem.alloc", m.Alloc)
			c.Gauge("profiling.mem.totalalloc", m.TotalAlloc)
			c.Gauge("profiling.mem.sys", m.Sys)
			c.Gauge("profiling.mem.pausetotalns", m.PauseTotalNs)
			c.Gauge("profiling.mem.heapobjects", m.HeapObjects)
			c.Gauge("profiling.mem.mallocs", m.Mallocs)
			c.Gauge("profiling.mem.frees", m.Frees)

			c.Gauge("profiling.goroutines", runtime.NumGoroutine())

			dbStats := rdb.Stats()
			c.Gauge("profiling.db.openconnections", dbStats.OpenConnections)
			c.Gauge("profiling.db.inuse", dbStats.InUse)
			c.Gauge("profiling.db.idle", dbStats.Idle)

		case <-APIrequestTelemetry:
			c.Increment("apicalls")
		}
	}
}
