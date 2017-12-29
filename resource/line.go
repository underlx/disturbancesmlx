package resource

import (
	"github.com/gbl08ma/disturbancesmlx/dataobjects"
	"github.com/heetch/sqalx"
	"github.com/yarf-framework/yarf"
)

// Line composites resource
type Line struct {
	resource
}

type apiLine struct {
	ID          string               `msgpack:"id" json:"id"`
	Name        string               `msgpack:"name" json:"name"`
	Color       string               `msgpack:"color" json:"color"`
	TypicalCars int                  `msgpack:"typCars" json:"typCars"`
	Network     *dataobjects.Network `msgpack:"-" json:"-"`
}

type apiLineSchedule struct {
	Line         *dataobjects.Line    `msgpack:"-" json:"-"`
	Holiday      bool                 `msgpack:"holiday" json:"holiday"`
	Day          int                  `msgpack:"day" json:"day"`
	Open         bool                 `msgpack:"open" json:"open"`
	OpenTime     dataobjects.Time     `msgpack:"openTime" json:"openTime"`
	OpenDuration dataobjects.Duration `msgpack:"duration" json:"duration"`
}

type apiLineWrapper struct {
	apiLine   `msgpack:",inline"`
	NetworkID string            `msgpack:"network" json:"network"`
	Stations  []string          `msgpack:"stations" json:"stations"`
	Schedule  []apiLineSchedule `msgpack:"schedule" json:"schedule"`
}

func (r *Line) WithNode(node sqalx.Node) *Line {
	r.node = node
	return r
}

func (n *Line) Get(c *yarf.Context) error {
	tx, err := n.Beginx()
	if err != nil {
		return err
	}
	defer tx.Commit() // read-only tx

	if c.Param("id") != "" {
		line, err := dataobjects.GetLine(tx, c.Param("id"))
		if err != nil {
			return err
		}
		data := apiLineWrapper{
			apiLine:   apiLine(*line),
			NetworkID: line.Network.ID,
		}

		data.Stations = []string{}
		stations, err := line.Stations(tx)
		if err != nil {
			return err
		}
		for _, station := range stations {
			data.Stations = append(data.Stations, station.ID)
		}

		data.Schedule = []apiLineSchedule{}
		schedules, err := line.Schedules(tx)
		if err != nil {
			return err
		}
		for _, s := range schedules {
			data.Schedule = append(data.Schedule, apiLineSchedule(*s))
		}

		RenderData(c, data)
	} else {
		lines, err := dataobjects.GetLines(tx)
		if err != nil {
			return err
		}
		apilines := make([]apiLineWrapper, len(lines))
		for i := range lines {
			apilines[i] = apiLineWrapper{
				apiLine:   apiLine(*lines[i]),
				NetworkID: lines[i].Network.ID,
			}

			apilines[i].Stations = []string{}
			stations, err := lines[i].Stations(tx)
			if err != nil {
				return err
			}
			for _, station := range stations {
				apilines[i].Stations = append(apilines[i].Stations, station.ID)
			}

			apilines[i].Schedule = []apiLineSchedule{}
			schedules, err := lines[i].Schedules(tx)
			if err != nil {
				return err
			}
			for _, s := range schedules {
				apilines[i].Schedule = append(apilines[i].Schedule, apiLineSchedule(*s))
			}
		}
		RenderData(c, apilines)
	}
	return nil
}
