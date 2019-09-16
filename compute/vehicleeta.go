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

// NextVehicleETA returns the ETA of the next vehicle arriving at the specified station, in the specified direction
// Returns nil if an ETA is not available
func (h *VehicleETAHandler) NextVehicleETA(station *dataobjects.Station, direction *dataobjects.Station) *dataobjects.VehicleETA {
	etaIface, ok := h.etas.Get(h.cacheKey(station, direction, 1))
	if !ok {
		return nil
	}

	return etaIface.(*dataobjects.VehicleETA)
}

func (h *VehicleETAHandler) cacheKey(station, direction *dataobjects.Station, arrivalOrder int) string {
	return fmt.Sprintf("%s#%s#%d", station.ID, direction.ID, arrivalOrder)
}
