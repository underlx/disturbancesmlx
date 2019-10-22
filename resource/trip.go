package resource

import (
	"net/http"
	"time"

	"github.com/gbl08ma/sqalx"
	"github.com/underlx/disturbancesmlx/types"
	"github.com/underlx/disturbancesmlx/posplay"
	"github.com/yarf-framework/yarf"
)

// Trip composites resource
type Trip struct {
	resource
}

type apiTrip struct {
	ID            string                    `msgpack:"id" json:"id"`
	StartTime     time.Time                 `msgpack:"startTime" json:"startTime"`
	EndTime       time.Time                 `msgpack:"endTime" json:"endTime"`
	Submitter     *types.APIPair      `msgpack:"-" json:"-"`
	SubmitTime    time.Time                 `msgpack:"submitTime" json:"submitTime"`
	EditTime      time.Time                 `msgpack:"editTime" json:"editTime"`
	Edited        bool                      `msgpack:"edited" json:"edited"`
	UserConfirmed bool                      `msgpack:"userConfirmed" json:"userConfirmed"`
	StationUses   []*types.StationUse `msgpack:"-" json:"-"`
}

type apiTripWrapper struct {
	apiTrip        `msgpack:",inline"`
	APIstationUses []apiStationUseWrapper `msgpack:"uses" json:"uses"`
}

type apiTripCreationRequest struct {
	// ID must be a v4 UUID
	ID            string                 `msgpack:"id" json:"id"`
	Uses          []apiStationUseWrapper `msgpack:"uses" json:"uses"`
	UserConfirmed bool                   `msgpack:"userConfirmed" json:"userConfirmed"`
}

type apiStationUse struct {
	Station    *types.Station       `msgpack:"-" json:"-"`
	EntryTime  time.Time                  `msgpack:"entryTime" json:"entryTime"`
	LeaveTime  time.Time                  `msgpack:"leaveTime" json:"leaveTime"`
	Type       types.StationUseType `msgpack:"-" json:"-"`
	Manual     bool                       `msgpack:"manual" json:"manual"`
	SourceLine *types.Line          `msgpack:"-" json:"-"`
	TargetLine *types.Line          `msgpack:"-" json:"-"`
}

type apiStationUseWrapper struct {
	apiStationUse `msgpack:",inline"`
	StationID     string `msgpack:"station" json:"station"`
	TypeString    string `msgpack:"type" json:"type"`
	SourceLineID  string `msgpack:"sourceLine" json:"sourceLine"`
	TargetLineID  string `msgpack:"targetLine" json:"targetLine"`
}

// WithNode associates a sqalx Node with this resource
func (r *Trip) WithNode(node sqalx.Node) *Trip {
	r.node = node
	return r
}

// WithHashKey associates a HMAC key with this resource so it can participate in authentication processes
func (r *Trip) WithHashKey(key []byte) *Trip {
	r.hashKey = key
	return r
}

// Get serves HTTP GET requests on this resource
func (r *Trip) Get(c *yarf.Context) error {
	pair, err := r.AuthenticateClient(c)
	if err != nil {
		RenderUnauthorized(c)
		return nil
	}

	tx, err := r.Beginx()
	if err != nil {
		return err
	}
	defer tx.Commit() // read-only tx

	if c.Param("id") != "" {
		trip, err := types.GetTrip(tx, c.Param("id"))
		if err != nil || trip.Submitter.Key != pair.Key {
			return &yarf.CustomError{
				HTTPCode:  http.StatusNotFound,
				ErrorMsg:  "The specified trip does not exist",
				ErrorBody: "The specified trip does not exist",
			}
		}

		r.render(c, trip)
	} else {
		trips, err := types.GetTripsForSubmitter(tx, pair)
		if err != nil {
			return err
		}

		apitrips := make([]apiTripWrapper, len(trips))

		for i := range trips {
			apitrips[i] = apiTripWrapper{
				apiTrip:        apiTrip(*trips[i]),
				APIstationUses: []apiStationUseWrapper{},
			}

			for _, use := range trips[i].StationUses {
				sw := apiStationUseWrapper{
					apiStationUse: apiStationUse(*use),
					StationID:     use.Station.ID,
					TypeString:    string(use.Type),
				}
				apitrips[i].APIstationUses = append(apitrips[i].APIstationUses, sw)
			}
		}

		RenderData(c, apitrips, "private")
	}

	return nil
}

// Post serves HTTP POST requests on this resource
func (r *Trip) Post(c *yarf.Context) error {
	pair, err := r.AuthenticateClient(c)
	if err != nil {
		RenderUnauthorized(c)
		return nil
	}

	var request apiTripCreationRequest
	err = r.DecodeRequest(c, &request)
	if err != nil {
		return err
	}

	if len(request.Uses) == 0 {
		return &yarf.CustomError{
			HTTPCode:  http.StatusBadRequest,
			ErrorMsg:  "Trip contains no station uses",
			ErrorBody: "Trip contains no station uses",
		}
	}

	tx, err := r.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Trip UUIDs are client-generated, so we can't really trust their (lack of) uniqueness...
	if _, err := types.GetTrip(tx, request.ID); err == nil {
		return &yarf.CustomError{
			HTTPCode:  http.StatusBadRequest,
			ErrorMsg:  "A trip with the specified ID already exists. Better luck generating a UUID next time.",
			ErrorBody: "A trip with the specified ID already exists. Better luck generating a UUID next time.",
		}
	}

	trip := types.Trip{
		ID:            request.ID,
		StartTime:     request.Uses[0].EntryTime,
		EndTime:       request.Uses[len(request.Uses)-1].LeaveTime,
		Submitter:     pair,
		SubmitTime:    time.Now().UTC(),
		UserConfirmed: request.UserConfirmed,
		StationUses:   []*types.StationUse{},
	}

	for _, requestUse := range request.Uses {
		use := types.StationUse{
			EntryTime: requestUse.EntryTime,
			LeaveTime: requestUse.LeaveTime,
			Type:      types.StationUseType(requestUse.TypeString),
			Manual:    requestUse.Manual,
		}

		use.Station, err = types.GetStation(tx, requestUse.StationID)
		if err != nil {
			return &yarf.CustomError{
				HTTPCode:  http.StatusBadRequest,
				ErrorMsg:  "Station use contains invalid station ID",
				ErrorBody: "Station use contains invalid station ID",
			}
		}

		if use.Type == types.Interchange {
			if requestUse.SourceLineID != "" {
				use.SourceLine, err = types.GetLine(tx, requestUse.SourceLineID)
				if err != nil {
					return &yarf.CustomError{
						HTTPCode:  http.StatusBadRequest,
						ErrorMsg:  "Invalid source line in station use",
						ErrorBody: "Invalid source line in station use",
					}
				}
			}
			if requestUse.TargetLineID != "" {
				use.TargetLine, err = types.GetLine(tx, requestUse.TargetLineID)
				if err != nil {
					return &yarf.CustomError{
						HTTPCode:  http.StatusBadRequest,
						ErrorMsg:  "Invalid target line in station use",
						ErrorBody: "Invalid target line in station use",
					}
				}
			}
		}

		trip.StationUses = append(trip.StationUses, &use)
	}

	err = trip.Update(tx)
	if err != nil {
		return err
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	c.Response.WriteHeader(http.StatusCreated)
	c.Response.Header().Set("Location", "/v1/trips/"+trip.ID)
	r.render(c, &trip)

	posplay.RegisterTripSubmission(&trip)
	return nil
}

// Put serves HTTP PUT requests on this resource
func (r *Trip) Put(c *yarf.Context) error {
	pair, err := r.AuthenticateClient(c)
	if err != nil {
		RenderUnauthorized(c)
		return nil
	}

	var request apiTripCreationRequest
	err = r.DecodeRequest(c, &request)
	if err != nil {
		return err
	}

	tx, err := r.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	trip, hadBeenEdited, err := r.getTripToEdit(tx, &request, pair)
	if err != nil {
		return err
	}

	err = trip.Update(tx)
	if err != nil {
		return err
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	r.render(c, &trip)

	if !hadBeenEdited {
		posplay.RegisterTripFirstEdit(&trip)
	}

	return nil
}

func (r *Trip) getTripToEdit(tx sqalx.Node, request *apiTripCreationRequest, pair *types.APIPair) (types.Trip, bool, error) {
	if len(request.Uses) == 0 {
		return types.Trip{}, false, &yarf.CustomError{
			HTTPCode:  http.StatusBadRequest,
			ErrorMsg:  "Trip contains no station uses",
			ErrorBody: "Trip contains no station uses",
		}
	}

	oldtrip, err := types.GetTrip(tx, request.ID)
	if err != nil || oldtrip.Submitter.Key != pair.Key {
		return types.Trip{}, false, &yarf.CustomError{
			HTTPCode:  http.StatusNotFound,
			ErrorMsg:  "A trip with the specified ID was not found.",
			ErrorBody: "A trip with the specified ID was not found.",
		}
	}

	if time.Since(oldtrip.SubmitTime) > 7*24*time.Hour {
		return types.Trip{}, false, &yarf.CustomError{
			HTTPCode:  http.StatusLocked,
			ErrorMsg:  "This trip was submitted over 7 days ago and can no longer be edited.",
			ErrorBody: "This trip was submitted over 7 days ago and can no longer be edited.",
		}
	}

	trip := types.Trip{
		ID:            request.ID,
		StartTime:     request.Uses[0].EntryTime,
		EndTime:       request.Uses[len(request.Uses)-1].LeaveTime,
		Submitter:     pair,
		SubmitTime:    oldtrip.SubmitTime,
		EditTime:      time.Now().UTC(),
		Edited:        true,
		UserConfirmed: request.UserConfirmed,
		StationUses:   []*types.StationUse{},
	}

	maxFuture := time.Now().Add(15 * time.Minute)
	if trip.StartTime.After(maxFuture) || trip.EndTime.After(maxFuture) {
		return types.Trip{}, false, &yarf.CustomError{
			HTTPCode:  http.StatusBadRequest,
			ErrorMsg:  "This trip is from the future. Adjust your clock.",
			ErrorBody: "This trip is from the future. Adjust your clock.",
		}
	}

	if trip.EndTime.Sub(trip.StartTime) > 24*time.Hour {
		// probably the clock of the phone was adjusted (from the default 1970-01-01) between the start and end of the trip
		return types.Trip{}, false, &yarf.CustomError{
			HTTPCode:  http.StatusBadRequest,
			ErrorMsg:  "This trip took way too long.",
			ErrorBody: "This trip took way too long.",
		}
	}

	err = r.buildStationUses(tx, request, &trip)
	if err != nil {
		return types.Trip{}, false, err
	}
	return trip, oldtrip.Edited, nil
}

func (r *Trip) buildStationUses(tx sqalx.Node, request *apiTripCreationRequest, trip *types.Trip) error {
	var err error
	for _, requestUse := range request.Uses {
		use := types.StationUse{
			EntryTime: requestUse.EntryTime,
			LeaveTime: requestUse.LeaveTime,
			Type:      types.StationUseType(requestUse.TypeString),
			Manual:    requestUse.Manual,
		}
		use.Station, err = types.GetStation(tx, requestUse.StationID)
		if err != nil {
			return &yarf.CustomError{
				HTTPCode:  http.StatusBadRequest,
				ErrorMsg:  "Station use contains invalid station ID",
				ErrorBody: "Station use contains invalid station ID",
			}
		}

		if use.Type == types.Interchange {
			if requestUse.SourceLineID != "" {
				use.SourceLine, err = types.GetLine(tx, requestUse.SourceLineID)
				if err != nil {
					return &yarf.CustomError{
						HTTPCode:  http.StatusBadRequest,
						ErrorMsg:  "Invalid source line in station use",
						ErrorBody: "Invalid source line in station use",
					}
				}
			}
			if requestUse.TargetLineID != "" {
				use.TargetLine, err = types.GetLine(tx, requestUse.TargetLineID)
				if err != nil {
					return &yarf.CustomError{
						HTTPCode:  http.StatusBadRequest,
						ErrorMsg:  "Invalid target line in station use",
						ErrorBody: "Invalid target line in station use",
					}
				}
			}
		}

		trip.StationUses = append(trip.StationUses, &use)
	}
	return nil
}

func (r *Trip) render(c *yarf.Context, trip *types.Trip) {
	data := apiTripWrapper{
		apiTrip:        apiTrip(*trip),
		APIstationUses: []apiStationUseWrapper{},
	}

	for _, use := range trip.StationUses {
		sw := apiStationUseWrapper{
			apiStationUse: apiStationUse(*use),
			StationID:     use.Station.ID,
			TypeString:    string(use.Type),
		}
		if use.SourceLine != nil {
			sw.SourceLineID = use.SourceLine.ID
		}
		if use.TargetLine != nil {
			sw.TargetLineID = use.TargetLine.ID
		}
		data.APIstationUses = append(data.APIstationUses, sw)
	}

	RenderData(c, data, "private")
}
