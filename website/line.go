package website

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/underlx/disturbancesmlx/dataobjects"
)

// LinePage serves the page for a specific line
func LinePage(w http.ResponseWriter, r *http.Request) {
	tx, err := rootSqalxNode.Beginx()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		webLog.Println(err)
		return
	}
	defer tx.Commit()

	p := struct {
		PageCommons
		Line        *dataobjects.Line
		Stations    []*dataobjects.Station
		StationInfo []struct {
			Closed              bool
			LeftLine, RightLine *dataobjects.Line
		}
		WeekAvailability  float64
		WeekDuration      time.Duration
		MonthAvailability float64
		MonthDuration     time.Duration
		Disturbances      []*dataobjects.Disturbance
	}{}

	p.Line, err = dataobjects.GetLine(tx, mux.Vars(r)["id"])
	if err != nil {
		webLog.Println(err)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	p.PageCommons, err = InitPageCommons(tx, w, r, fmt.Sprintf("Linha %s do %s", p.Line.Name, p.Line.Network.Name))
	if err != nil {
		webLog.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	p.Stations, err = p.Line.Stations(tx)
	if err != nil {
		webLog.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	p.StationInfo = make([]struct {
		Closed              bool
		LeftLine, RightLine *dataobjects.Line
	}, len(p.Stations))

	for i, station := range p.Stations {
		if closed, err := station.Closed(tx); err == nil && closed {
			p.StationInfo[i].Closed = true
		}

		lines, err := station.Lines(tx)
		if err != nil {
			webLog.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		for _, line := range lines {
			if line.ID != p.Line.ID {
				p.StationInfo[i].RightLine = line
				stations, err := line.Stations(tx)
				if err != nil {
					webLog.Println(err)
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				if len(stations) > 0 && station.ID != stations[0].ID && station.ID != stations[len(stations)-1].ID {
					p.StationInfo[i].LeftLine = line
				}
			}
		}
	}

	loc, err := time.LoadLocation(p.Line.Network.Timezone)
	if err != nil {
		webLog.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	now := time.Now().In(loc)

	p.MonthAvailability, p.MonthDuration, err = p.Line.Availability(tx, now.AddDate(0, -1, 0), now, p.OfficialOnly)
	if err != nil {
		webLog.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	p.MonthAvailability *= 100

	p.WeekAvailability, p.WeekDuration, err = p.Line.Availability(tx, now.AddDate(0, 0, -7), now, p.OfficialOnly)
	if err != nil {
		webLog.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	p.WeekAvailability *= 100

	weekAgo := now.AddDate(0, 0, -7)
	weekAgo = time.Date(weekAgo.Year(), weekAgo.Month(), weekAgo.Day(), 0, 0, 0, 0, loc)
	p.Disturbances, err = p.Line.DisturbancesBetween(tx, weekAgo, now, p.OfficialOnly)
	if err != nil {
		webLog.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	err = webtemplate.ExecuteTemplate(w, "line.html", p)
	if err != nil {
		webLog.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
	}
}
