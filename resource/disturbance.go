package resource

import (
	"time"

	"github.com/gbl08ma/sqalx"
	"github.com/underlx/disturbancesmlx/dataobjects"
	"github.com/yarf-framework/yarf"
)

// Disturbance composites resource
type Disturbance struct {
	resource
}

type apiDisturbance struct {
	ID          string                `msgpack:"id" json:"id"`
	Official    bool                  `msgpack:"official" json:"official"`
	OStartTime  time.Time             `msgpack:"oStartTime" json:"oStartTime"`
	OEndTime    time.Time             `msgpack:"oEndTime" json:"oEndTime"`
	OEnded      bool                  `msgpack:"oEnded" json:"oEnded"`
	UStartTime  time.Time             `msgpack:"startTime" json:"startTime"`
	UEndTime    time.Time             `msgpack:"endTime" json:"endTime"`
	UEnded      bool                  `msgpack:"ended" json:"ended"`
	Line        *dataobjects.Line     `msgpack:"-" json:"-"`
	Description string                `msgpack:"description" json:"description"`
	Notes       string                `msgpack:"notes" json:"notes"`
	Statuses    []*dataobjects.Status `msgpack:"-" json:"-"`
}

type apiDisturbanceWrapper struct {
	apiDisturbance `msgpack:",inline"`
	NetworkID      string                            `msgpack:"network" json:"network"`
	LineID         string                            `msgpack:"line" json:"line"`
	Categories     []dataobjects.DisturbanceCategory `msgpack:"categories" json:"categories"`
	APIstatuses    []apiStatusWrapper                `msgpack:"statuses" json:"statuses"`
}

type apiStatus struct {
	ID         string                        `msgpack:"id" json:"id"`
	Time       time.Time                     `msgpack:"time" json:"time"`
	Line       *dataobjects.Line             `msgpack:"-" json:"-"`
	IsDowntime bool                          `msgpack:"downtime" json:"downtime"`
	Status     string                        `msgpack:"status" json:"status"`
	Source     *dataobjects.Source           `msgpack:"-" json:"-"`
	MsgType    dataobjects.StatusMessageType `msgpack:"msgType" json:"msgType"`
}

type apiStatusWrapper struct {
	apiStatus `msgpack:",inline"`
	SourceID  string `msgpack:"source" json:"source"`
}

// WithNode associates a sqalx Node with this resource
func (r *Disturbance) WithNode(node sqalx.Node) *Disturbance {
	r.node = node
	return r
}

// Get serves HTTP GET requests on this resource
func (r *Disturbance) Get(c *yarf.Context) error {
	tx, err := r.Beginx()
	if err != nil {
		return err
	}
	defer tx.Commit() // read-only tx

	omitDuplicateStatus := c.Request.URL.Query().Get("omitduplicatestatus") == "true"

	if c.Param("id") != "" {
		disturbance, err := dataobjects.GetDisturbance(tx, c.Param("id"))
		if err != nil {
			return err
		}
		data := apiDisturbanceWrapper{
			apiDisturbance: apiDisturbance(*disturbance),
			NetworkID:      disturbance.Line.Network.ID,
			LineID:         disturbance.Line.ID,
			Categories:     disturbance.Categories(),
		}

		data.APIstatuses = []apiStatusWrapper{}
		prevStatusText := ""
		for i, status := range disturbance.Statuses {
			sw := apiStatusWrapper{
				apiStatus: apiStatus(*status),
				SourceID:  status.Source.ID,
			}
			if !omitDuplicateStatus || prevStatusText != status.Status || i == 0 {
				prevStatusText = status.Status
				data.APIstatuses = append(data.APIstatuses, sw)
			}
		}

		RenderData(c, data, "s-maxage=10")
	} else {
		var disturbances []*dataobjects.Disturbance
		var err error
		start := c.Request.URL.Query().Get("start")
		cacheControl := "s-maxage=10"
		if start == "" {
			switch c.Request.URL.Query().Get("filter") {
			case "ongoing":
				disturbances, err = dataobjects.GetOngoingDisturbances(tx)
				cacheControl = "no-cache, no-store, must-revalidate"
			default:
				disturbances, err = dataobjects.GetDisturbances(tx)
			}
		} else {
			startTime, err2 := time.Parse(time.RFC3339, start)
			if err2 != nil {
				return err2
			}
			end := c.Request.URL.Query().Get("end")
			endTime := time.Now()
			if end != "" {
				endTime, err2 = time.Parse(time.RFC3339, end)
				if err2 != nil {
					return err2
				}
			}
			disturbances, err = dataobjects.GetDisturbancesBetween(tx, startTime, endTime)
		}

		if err != nil {
			return err
		}
		apidisturbances := make([]apiDisturbanceWrapper, len(disturbances))
		for i := range disturbances {
			apidisturbances[i] = apiDisturbanceWrapper{
				apiDisturbance: apiDisturbance(*disturbances[i]),
				NetworkID:      disturbances[i].Line.Network.ID,
				LineID:         disturbances[i].Line.ID,
				Categories:     disturbances[i].Categories(),
			}

			apidisturbances[i].APIstatuses = []apiStatusWrapper{}
			prevStatusText := ""
			for j, status := range disturbances[i].Statuses {
				sw := apiStatusWrapper{
					apiStatus: apiStatus(*status),
					SourceID:  status.Source.ID,
				}
				if !omitDuplicateStatus || prevStatusText != status.Status || j == 0 {
					prevStatusText = status.Status
					apidisturbances[i].APIstatuses = append(apidisturbances[i].APIstatuses, sw)
				}
			}
		}
		RenderData(c, apidisturbances, cacheControl)
	}
	return nil
}
