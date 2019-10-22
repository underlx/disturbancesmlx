package compute

import (
	"fmt"
	"sync"
	"time"

	movingaverage "github.com/RobinUS2/golang-moving-average"
	"github.com/gbl08ma/sqalx"
	cache "github.com/patrickmn/go-cache"
	"github.com/underlx/disturbancesmlx/types"
)

// VehicleETAHandler aggregates and manages VehicleETAs
type VehicleETAHandler struct {
	node sqalx.Node

	etas    *cache.Cache
	etasPos *cache.Cache

	connDurMutex  sync.Mutex
	connDurations map[string]*movingaverage.MovingAverage
}

var platformAliasesForPositions = map[string]string{
	"AP5APO": "AP8APO", // pt-ml-ap
	"AS6RBO": "AS7RBO", // pt-ml-rb
	"CS6O":   "CS13O",  // pt-ml-cs
	"CS27O":  "CS13O",  // pt-ml-cs (2nd departure platform)
	"TE5O":   "TE12O",  // pt-ml-te
	"S27O":   "S26O",   // pt-ml-ss
	"SP2O":   "SP9O",   // pt-ml-sp
}

var terminalArrivalPlatforms = map[string]bool{
	"AP5APO": true,
	"AS6RBO": true,
	"CS6O":   true,
	"TE5O":   true,
	"S27O":   true,
	"SP2O":   true,
}

// NewVehicleETAHandler returns a new VehicleETAHandler
func NewVehicleETAHandler(node sqalx.Node) *VehicleETAHandler {
	return &VehicleETAHandler{
		node:          node,
		etas:          cache.New(cache.NoExpiration, 1*time.Minute),
		etasPos:       cache.New(cache.NoExpiration, 1*time.Minute),
		connDurations: make(map[string]*movingaverage.MovingAverage),
	}
}

// RegisterVehicleETA adds a prediction to the system, replacing any previous predictions for the same vehicle
func (h *VehicleETAHandler) RegisterVehicleETA(eta *types.VehicleETA) {
	h.etas.Set(
		h.cacheKey(eta.Station, eta.Direction, eta.ArrivalOrder),
		eta,
		eta.ValidFor)
	if eta.ArrivalOrder == 1 {
		h.etasPos.Set(
			h.cacheKeyPos(eta.Station, eta.Direction, eta.Platform),
			eta,
			eta.ValidFor)

		if eta.LiveETA() == 0 {
			h.registerTime(eta)
		}
	}
}

// VehicleETAs returns the ETAs of the next `numVehicles` arriving at the specified station, in the specified direction
// Returns an empty slice if no ETA is available
func (h *VehicleETAHandler) VehicleETAs(station *types.Station, direction *types.Station, numVehicles int) []*types.VehicleETA {
	result := []*types.VehicleETA{}
	for i := 1; i < numVehicles+1; i++ {
		etaIface, ok := h.etas.Get(h.cacheKey(station, direction, i))
		if !ok {
			continue
		}

		result = append(result, etaIface.(*types.VehicleETA))
	}
	return result
}

func (h *VehicleETAHandler) cacheKey(station, direction *types.Station, arrivalOrder int) string {
	return fmt.Sprintf("%s#%s#%d", station.ID, direction.ID, arrivalOrder)
}

func (h *VehicleETAHandler) cacheKeyPos(station, direction *types.Station, platform string) string {
	return fmt.Sprintf("%s#%s#%s", station.ID, direction.ID, platform)
}

func (h *VehicleETAHandler) cacheKeyDur(connection *types.Connection) string {
	return fmt.Sprintf("%s#%s", connection.From.ID, connection.To.ID)
}

// TrainPositions returns VehicleETAs containing the closest position for each train in the network.
// The returned map is indexed by VehicleServiceID
func (h *VehicleETAHandler) TrainPositions() map[string]*types.VehicleETA {
	less := func(e1, e2 *types.VehicleETA) bool {
		if platformAliasesForPositions[e1.Platform] == e2.Platform && terminalArrivalPlatforms[e1.Platform] {
			// prefer the terminal arrival platform if the train is still present in the predictions for it
			// set direction to that of station manually, (often their predictions include the same direction for both platforms,
			// and it is that of the opposite terminal)
			e1.Direction = e1.Station
			return true
		}
		if platformAliasesForPositions[e2.Platform] == e1.Platform && terminalArrivalPlatforms[e2.Platform] {
			// same thing as above, swapped
			return false
		}
		return e1.LiveETA() < e2.LiveETA()
	}
	// m maps VehicleServiceID to VehicleETAs
	m := make(map[string]*types.VehicleETA)
	for _, itemIface := range h.etasPos.Items() {
		item := itemIface.Object.(*types.VehicleETA)
		if time.Since(item.Computed) > 2*time.Minute {
			continue
		}
		min, ok := m[item.VehicleServiceID]
		if !ok || less(item, min) {
			m[item.VehicleServiceID] = item
		}
	}
	return m
}

// TrainsInLine returns the position of trains serving the specified line
// The returned map is indexed by VehicleServiceID
func (h *VehicleETAHandler) TrainsInLine(line *types.Line) map[string]*types.VehicleETA {
	m := make(map[string]*types.VehicleETA)
	for _, eta := range h.TrainPositions() {
		id, err := eta.VehicleIDgetLineString()
		if err == nil && id == line.ExternalID {
			m[eta.VehicleServiceID] = eta
		}
	}
	return m
}

func (h *VehicleETAHandler) registerTime(eta *types.VehicleETA) {
	tx, err := h.node.Beginx()
	if err != nil {
		return
	}
	defer tx.Commit() // read-only tx

	conns, err := types.GetConnectionsFromPlatform(tx, eta.Platform)
	if err != nil {
		return
	}
	if len(conns) == 0 {
		return
	}

	// TODO eta.Direction (as provided by the Metro network) may be sometimes "awkward" for our use case
	etas := h.VehicleETAs(conns[0].To, eta.Direction, 1)
	if len(etas) == 0 {
		return
	}

	h.connDurMutex.Lock()
	defer h.connDurMutex.Unlock()

	key := h.cacheKeyDur(conns[0])
	mAvg, ok := h.connDurations[key]
	if !ok {
		mAvg = movingaverage.New(10)
		h.connDurations[key] = mAvg
	}
	mAvg.Add(etas[0].LiveETA().Seconds())
}

// ConnectionDuration returns the typical amount of time it takes for a moving vehicle
// to cross the connection
func (h *VehicleETAHandler) ConnectionDuration(connection *types.Connection) (r time.Duration) {
	h.connDurMutex.Lock()
	defer h.connDurMutex.Unlock()

	r = time.Second * time.Duration(connection.TypicalSeconds+connection.TypicalStopSeconds)
	mAvg, ok := h.connDurations[h.cacheKeyDur(connection)]
	if !ok || mAvg.Count() == 0 {
		return
	}

	min, err := mAvg.Min()
	if err != nil {
		return
	}
	return time.Duration(min * float64(time.Second))
}

// VehiclePosition takes a VehicleETA and returns the last station that vehicle
// has gone through and a percentage indicating an approximation of its position
// on the connection to the next station
// tx is optional (if nil, a new tx will be created) but if a tx is already in
// progress, passing it for better performance is recommended (if this function
// is being called many times in a row)
func (h *VehicleETAHandler) VehiclePosition(tx sqalx.Node, eta *types.VehicleETA) (prev *types.Station, percentage uint) {
	node := tx
	if node == nil {
		node = h.node
	}
	tx, err := node.Beginx()
	if err != nil {
		return
	}
	defer tx.Commit() // read-only tx

	defer func() {
		if prev == nil {
			prev = eta.Station
			percentage = 0
		}
		if percentage > 100 {
			percentage = 100
		}
		if percentage == 100 && eta.LiveETA() > 0 {
			percentage = 99
		}
	}()

	platform := eta.Platform
	if alias, ok := platformAliasesForPositions[eta.Platform]; ok {
		platform = alias
	}

	connections, err := types.GetConnectionsToPlatform(tx, platform)
	if err != nil {
		return nil, 0
	}

	if len(connections) == 0 {
		return nil, 0
	}

	connDur := h.ConnectionDuration(connections[0])
	p := 100 - (eta.LiveETA().Seconds()*100)/connDur.Seconds()
	if p < 0 {
		p = 0
	}
	percentage = uint(p)

	return connections[0].From, percentage
}
