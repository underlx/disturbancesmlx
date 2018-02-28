package main

import (
	"errors"
	"math/rand"
	"time"

	"github.com/underlx/disturbancesmlx/dataobjects"
	"github.com/heetch/sqalx"
	cache "github.com/patrickmn/go-cache"
)

var statsHandler = new(StatsHandler)

// StatsHandler implements resource.StatsCalculator and resource.RealtimeStatsHandler
type StatsHandler struct {
	activity *cache.Cache
}

func init() {
	statsHandler.activity = cache.New(cache.NoExpiration, 10*time.Minute)
}

func (*StatsHandler) Availability(node sqalx.Node, line *dataobjects.Line, startTime time.Time, endTime time.Time) (float64, time.Duration, error) {
	if line.Network.ID == "pt-ml" {
		return MLlineAvailability(node, line, startTime, endTime)
	}
	return 0, 0, errors.New("Availability: line belongs to unsupported network")
}

func (h *StatsHandler) CurrentlyOnlineInTransit(network *dataobjects.Network, approximateTo int) int {
	value := 0
	if network.ID == "pt-ml" {
		h.activity.DeleteExpired()
		value = h.activity.ItemCount()
	}
	if approximateTo < 2 {
		return value
	}

	value += -approximateTo/2 + rand.Intn(approximateTo)

	if value < 0 {
		value = 0
	}

	value = (value / approximateTo) * approximateTo

	return value
}

func (h *StatsHandler) RegisterActivity(network *dataobjects.Network, user *dataobjects.APIPair, expectedDuration time.Duration) {
	if network.ID == "pt-ml" {
		// the value we set doesn't matter
		h.activity.Set(user.Key, 1, expectedDuration)
	}
}
