package main

import (
	"math/rand"
	"time"

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
