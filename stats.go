package main

import (
	"errors"
	"math/rand"
	"time"

	"github.com/heetch/sqalx"
	cache "github.com/patrickmn/go-cache"
	"github.com/underlx/disturbancesmlx/dataobjects"
)

var statsHandler = new(StatsHandler)

// StatsHandler implements resource.StatsCalculator and resource.RealtimeStatsHandler
type StatsHandler struct {
	activity *cache.Cache
}

func init() {
	statsHandler.activity = cache.New(cache.NoExpiration, 10*time.Minute)
}

// Availability returns the availability of a line during the specified period of time
func (*StatsHandler) Availability(node sqalx.Node, line *dataobjects.Line, startTime time.Time, endTime time.Time) (float64, time.Duration, error) {
	if line.Network.ID == "pt-ml" {
		return MLlineAvailability(node, line, startTime, endTime)
	}
	return 0, 0, errors.New("Availability: line belongs to unsupported network")
}

// CurrentlyOnlineInTransit returns the number of users in transit in the network
// fudged to the unit indicated by approximateTo (so if it equals 5, this function will return
// 0, 5, 10...). Use approximateTo = 0 to return the exact value
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

// RegisterActivity registers that a user is in transit in the given network
func (h *StatsHandler) RegisterActivity(network *dataobjects.Network, user *dataobjects.APIPair, expectedDuration time.Duration) {
	if network.ID == "pt-ml" {
		// the value we set doesn't matter
		h.activity.Set(user.Key, 1, expectedDuration)
	}
}
