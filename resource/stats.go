package resource

import (
	"time"

	"github.com/gbl08ma/disturbancesmlx/dataobjects"
	"github.com/heetch/sqalx"
	"github.com/yarf-framework/yarf"
)

// Stats composites resource
type Stats struct {
	resource
	calculator StatsCalculator
}

type apiStats struct {
	LineStats       map[string]apiLineStats `msgpack:"lineStats" json:"lineStats"`
	LastDisturbance time.Time               `msgpack:"lastDisturbance" json:"lastDisturbance"`
}

type apiLineStats struct {
	Availability               float64              `msgpack:"availability" json:"availability"`
	AverageDisturbanceDuration dataobjects.Duration `msgpack:"avgDistDuration" json:"avgDistDuration"`
}

type StatsCalculator interface {
	Availability(node sqalx.Node, line *dataobjects.Line, startTime time.Time, endTime time.Time) (float64, time.Duration, error)
}

func (r *Stats) WithNode(node sqalx.Node) *Stats {
	r.node = node
	return r
}

func (r *Stats) WithCalculator(calculator StatsCalculator) *Stats {
	r.calculator = calculator
	return r
}

func (r *Stats) Get(c *yarf.Context) error {
	tx, err := r.Beginx()
	if err != nil {
		return err
	}
	defer tx.Commit() // read-only tx

	start := c.Request.URL.Query().Get("start")
	startTime := time.Now().AddDate(0, 0, -7)
	if start != "" {
		startTime, err = time.Parse(time.RFC3339, start)
		if err != nil {
			return err
		}
	}

	end := c.Request.URL.Query().Get("end")
	endTime := time.Now()
	if end != "" {
		endTime, err = time.Parse(time.RFC3339, end)
		if err != nil {
			return err
		}
	}

	if c.Param("id") != "" {
		network, err := dataobjects.GetNetwork(tx, c.Param("id"))
		if err != nil {
			return err
		}

		stats, err := r.getStatsForNetwork(tx, network, startTime, endTime)
		if err != nil {
			return err
		}

		RenderData(c, stats)
	} else {
		statsMap := make(map[string]apiStats)
		networks, err := dataobjects.GetNetworks(tx)
		if err != nil {
			return err
		}

		for _, network := range networks {
			stats, err := r.getStatsForNetwork(tx, network, startTime, endTime)
			if err != nil {
				return err
			}
			statsMap[network.ID] = stats
		}
		RenderData(c, statsMap)
	}
	return nil
}

func (r *Stats) getStatsForNetwork(node sqalx.Node, network *dataobjects.Network, startTime time.Time, endTime time.Time) (apiStats, error) {
	tx, err := r.Beginx()
	if err != nil {
		return apiStats{}, err
	}
	defer tx.Commit() // read-only tx

	lastDist, err := r.getLastDisturbanceTimeForNetwork(tx, network)

	stats := apiStats{
		LastDisturbance: lastDist,
		LineStats:       make(map[string]apiLineStats),
	}

	lines, err := network.Lines(tx)
	if err != nil {
		return apiStats{}, err
	}

	for _, line := range lines {
		availability, avgDuration, err := r.calculator.Availability(tx, line, startTime, endTime)
		if err != nil {
			return apiStats{}, err
		}
		stats.LineStats[line.ID] = apiLineStats{
			Availability:               availability,
			AverageDisturbanceDuration: dataobjects.Duration(avgDuration),
		}
	}
	return stats, nil
}

func (r *Stats) getLastDisturbanceTimeForNetwork(node sqalx.Node, network *dataobjects.Network) (time.Time, error) {
	tx, err := r.Beginx()
	if err != nil {
		return time.Now().UTC(), err
	}
	defer tx.Commit() // read-only tx

	d, err := network.LastDisturbance(tx)
	if err != nil {
		return time.Now().UTC(), err
	}

	if !d.Ended {
		return time.Now().UTC(), nil
	}
	return d.EndTime, nil
}
