package resource

import (
	"time"

	"github.com/heetch/sqalx"
	"github.com/yarf-framework/yarf"
	"github.com/gbl08ma/disturbancesmlx/interfaces"
)

// Disturbance composites resource
type Disturbance struct {
	resource
}

type apiDisturbance struct {
	ID          string               `msgpack:"id" json:"id"`
	StartTime   time.Time            `msgpack:"startTime" json:"startTime"`
	EndTime     time.Time            `msgpack:"endTime" json:"endTime"`
	Ended       bool                 `msgpack:"ended" json:"ended"`
	Line        *interfaces.Line     `msgpack:"-" json:"-"`
	Description string               `msgpack:"description" json:"description"`
	Statuses    []*interfaces.Status `msgpack:"-" json:"-"`
}

type apiDisturbanceWrapper struct {
	apiDisturbance `msgpack:",inline"`
	NetworkID      string             `msgpack:"network" json:"network"`
	LineID         string             `msgpack:"line" json:"line"`
	APIstatuses    []apiStatusWrapper `msgpack:"statuses" json:"statuses"`
}

type apiStatus struct {
	ID         string             `msgpack:"id" json:"id"`
	Time       time.Time          `msgpack:"time" json:"time"`
	Line       *interfaces.Line   `msgpack:"-" json:"-"`
	IsDowntime bool               `msgpack:"downtime" json:"downtime"`
	Status     string             `msgpack:"status" json:"status"`
	Source     *interfaces.Source `msgpack:"-" json:"-"`
}

type apiStatusWrapper struct {
	apiStatus `msgpack:",inline"`
	SourceID  string `msgpack:"source" json:"source"`
}

func (r *Disturbance) WithNode(node sqalx.Node) *Disturbance {
	r.node = node
	return r
}

func (n *Disturbance) Get(c *yarf.Context) error {
	tx, err := n.Beginx()
	if err != nil {
		return err
	}
	defer tx.Commit() // read-only tx

	omitDuplicateStatus := c.Request.URL.Query().Get("omitduplicatestatus") == "true"

	if c.Param("id") != "" {
		disturbance, err := interfaces.GetDisturbance(tx, c.Param("id"))
		if err != nil {
			return err
		}
		data := apiDisturbanceWrapper{
			apiDisturbance: apiDisturbance(*disturbance),
			NetworkID:      disturbance.Line.Network.ID,
			LineID:         disturbance.Line.ID,
		}

		data.APIstatuses = []apiStatusWrapper{}
		prevStatusText := ""
		for _, status := range disturbance.Statuses {
			sw := apiStatusWrapper{
				apiStatus: apiStatus(*status),
				SourceID:  status.Source.ID,
			}
			if !omitDuplicateStatus || prevStatusText != status.Status {
				prevStatusText = status.Status
				data.APIstatuses = append(data.APIstatuses, sw)
			}
		}

		RenderData(c, data)
	} else {
		var disturbances []*interfaces.Disturbance
		var err error
		start := c.Request.URL.Query().Get("start")
		if start == "" {
			disturbances, err = interfaces.GetDisturbances(tx)
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
			disturbances, err = interfaces.GetDisturbancesBetween(tx, startTime, endTime)
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
			}

			apidisturbances[i].APIstatuses = []apiStatusWrapper{}
			prevStatusText := ""
			for _, status := range disturbances[i].Statuses {
				sw := apiStatusWrapper{
					apiStatus: apiStatus(*status),
					SourceID:  status.Source.ID,
				}
				if !omitDuplicateStatus || prevStatusText != status.Status {
					prevStatusText = status.Status
					apidisturbances[i].APIstatuses = append(apidisturbances[i].APIstatuses, sw)
				}
			}
		}
		RenderData(c, apidisturbances)
	}
	return nil
}
