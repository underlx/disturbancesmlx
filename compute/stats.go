package compute

import (
	"sync"
	"time"

	"github.com/thoas/go-funk"

	"github.com/gbl08ma/sqalx"
	cache "github.com/patrickmn/go-cache"
	"github.com/underlx/disturbancesmlx/types"
	"github.com/underlx/disturbancesmlx/utils"
)

// StatsHandler implements resource.StatsCalculator and resource.RealtimeStatsHandler
type StatsHandler struct {
	activityPerNetwork sync.Map
	activityPerLine    sync.Map
}

// NewStatsHandler returns a new, initialized StatsHandler
func NewStatsHandler() *StatsHandler {
	return new(StatsHandler)
}

func (h *StatsHandler) getNetworkCache(network *types.Network) *cache.Cache {
	actual, _ := h.activityPerNetwork.LoadOrStore(network.ID, cache.New(cache.NoExpiration, 10*time.Minute))
	return actual.(*cache.Cache)
}

func (h *StatsHandler) getLineCache(line *types.Line) *cache.Cache {
	actual, _ := h.activityPerLine.LoadOrStore(line.ID, cache.New(cache.NoExpiration, 10*time.Minute))
	return actual.(*cache.Cache)
}

// RangeNetworks calls f sequentially for each network known to this StatsHandler. If f returns false, the iteration is stopped.
func (h *StatsHandler) RangeNetworks(node sqalx.Node, f func(network *types.Network, cache *cache.Cache) bool) error {
	tx, err := node.Beginx()
	if err != nil {
		return err
	}
	defer tx.Commit() // read-only tx

	var innerErr error
	h.activityPerNetwork.Range(func(key, value interface{}) bool {
		network, err := types.GetNetwork(tx, key.(string))
		if err != nil {
			innerErr = err
			return false
		}
		return f(network, value.(*cache.Cache))
	})
	return innerErr
}

// RangeLines calls f sequentially for each line known to this StatsHandler. If f returns false, the iteration is stopped.
func (h *StatsHandler) RangeLines(node sqalx.Node, f func(line *types.Line, cache *cache.Cache) bool) error {
	tx, err := node.Beginx()
	if err != nil {
		return err
	}
	defer tx.Commit() // read-only tx

	var innerErr error
	h.activityPerLine.Range(func(key, value interface{}) bool {
		line, err := types.GetLine(tx, key.(string))
		if err != nil {
			innerErr = err
			return false
		}
		return f(line, value.(*cache.Cache))
	})
	return innerErr
}

// UserInNetwork returns true if the specified user has recently been in the specified network
// (data obtained through real-time location reports)
func (h *StatsHandler) UserInNetwork(network *types.Network, user *types.APIPair) bool {
	cache := h.getNetworkCache(network)
	_, present := cache.Get(user.Key)
	return present
}

// UserInLine returns true if the specified user has recently been in the specified line
// (data obtained through real-time location reports)
func (h *StatsHandler) UserInLine(line *types.Line, user *types.APIPair) bool {
	cache := h.getLineCache(line)
	_, present := cache.Get(user.Key)
	return present
}

// OITInNetwork returns the number of users online in transit in the specified network
// fudged to the unit indicated by approximateTo (so if it equals 5, this function will return
// 0, 5, 10...). Use approximateTo = 0 to return the exact value
func (h *StatsHandler) OITInNetwork(network *types.Network, approximateTo int) int {
	cache := h.getNetworkCache(network)
	cache.DeleteExpired()
	return utils.Fudge(cache.ItemCount(), approximateTo)
}

// OITInLine returns the number of users online in transit in the specified line
// fudged to the unit indicated by approximateTo (so if it equals 5, this function will return
// 0, 5, 10...). Use approximateTo = 0 to return the exact value
func (h *StatsHandler) OITInLine(line *types.Line, approximateTo int) int {
	cache := h.getLineCache(line)
	cache.DeleteExpired()
	return utils.Fudge(cache.ItemCount(), approximateTo)
}

// RegisterActivity registers that a user is in transit in the given lines
func (h *StatsHandler) RegisterActivity(lines []*types.Line, user *types.APIPair, justEntered bool) {
	expectedDuration := 4 * time.Minute
	// user just entered the network, is going to wait for a vehicle, or
	// user might change lines and will need to wait for a vehicle
	// (even if the user doesn't change lines, stations with multiple lines typically have more people and take longer to exit)
	if justEntered || len(lines) > 1 {
		expectedDuration = 8 * time.Minute
	}

	// add participation for the lines where the user actually is
	lineIDs := []string{}
	for _, line := range lines {
		lineIDs = append(lineIDs, line.ID)
		lineCache := h.getLineCache(line)
		// the value we set doesn't matter
		lineCache.Set(user.Key, 1, expectedDuration)

		networkCache := h.getNetworkCache(line.Network)
		// the value we set doesn't matter
		networkCache.Set(user.Key, 1, expectedDuration)
	}

	// cancel participation for the lines where the user isn't
	h.activityPerLine.Range(func(key, value interface{}) bool {
		if !funk.ContainsString(lineIDs, key.(string)) {
			value.(*cache.Cache).Delete(user.Key)
		}
		return true
	})
}
