package compute

import (
	"fmt"
	"time"

	cache "github.com/patrickmn/go-cache"
	"github.com/underlx/disturbancesmlx/dataobjects"
)

// VehicleETAHandler aggregates and manages VehicleETAs
type VehicleETAHandler struct {
	etas *cache.Cache
}

// NewVehicleETAHandler returns a new VehicleETAHandler
func NewVehicleETAHandler() *VehicleETAHandler {
	return &VehicleETAHandler{
		etas: cache.New(cache.NoExpiration, 1*time.Minute),
	}
}

// RegisterVehicleETA adds a prediction to the system, replacing any previous predictions for the same vehicle
func (h *VehicleETAHandler) RegisterVehicleETA(eta *dataobjects.VehicleETA) {
	h.etas.Set(
		h.cacheKey(eta.Station, eta.Direction, eta.ArrivalOrder),
		eta,
		eta.ValidFor)
}

// VehicleETAs returns the ETAs of the next `numVehicles` arriving at the specified station, in the specified direction
// Returns an empty slice if no ETA is available
func (h *VehicleETAHandler) VehicleETAs(station *dataobjects.Station, direction *dataobjects.Station, numVehicles int) []*dataobjects.VehicleETA {
	result := []*dataobjects.VehicleETA{}
	for i := 1; i < numVehicles+1; i++ {
		etaIface, ok := h.etas.Get(h.cacheKey(station, direction, i))
		if !ok {
			continue
		}

		result = append(result, etaIface.(*dataobjects.VehicleETA))
	}
	return result
}

func (h *VehicleETAHandler) cacheKey(station, direction *dataobjects.Station, arrivalOrder int) string {
	return fmt.Sprintf("%s#%s#%d", station.ID, direction.ID, arrivalOrder)
}

// TrainPositions returns VehicleETAs containing the closest position for each train in the network.
// The map is indexed by VehicleServiceID
func (h *VehicleETAHandler) TrainPositions() map[string]*dataobjects.VehicleETA {
	// m maps VehicleServiceID to VehicleETAs
	m := make(map[string]*dataobjects.VehicleETA)
	for _, itemIface := range h.etas.Items() {
		item := itemIface.Object.(*dataobjects.VehicleETA)
		min, ok := m[item.VehicleServiceID]
		if !ok || item.LiveETA() < min.LiveETA() {
			m[item.VehicleServiceID] = item
		}
	}
	return m
}
