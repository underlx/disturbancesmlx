package website

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/underlx/disturbancesmlx/utils"

	"github.com/goodsign/monday"
	"github.com/gorilla/mux"
	"github.com/underlx/disturbancesmlx/compute"
	"github.com/underlx/disturbancesmlx/dataobjects"
)

// DisturbancePage serves the page for a specific disturbance
func DisturbancePage(w http.ResponseWriter, r *http.Request) {
	tx, err := rootSqalxNode.Beginx()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		webLog.Println(err)
		return
	}
	defer tx.Commit()

	hasSession, session, err := AuthGetSession(w, r, false)
	if err != nil {
		webLog.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	p := struct {
		PageCommons
		Disturbance *dataobjects.Disturbance
		CanEdit     bool
	}{
		CanEdit: hasSession && session.IsAdmin,
	}

	p.PageCommons, err = InitPageCommons(tx, w, r, "Perturbação do Metro de Lisboa")
	if err != nil {
		webLog.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	p.Disturbance, err = dataobjects.GetDisturbance(tx, mux.Vars(r)["id"])
	if err != nil {
		webLog.Println(err)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	p.Description = fmt.Sprintf("Perturbação na linha %s do %s, em %s",
		p.Disturbance.Line.Name, p.Disturbance.Line.Network.Name,
		monday.Format(p.Disturbance.UStartTime, "2 de January de 2006", monday.LocalePtPT))

	reason := utils.DisturbanceReasonString(p.Disturbance, false)
	if reason != "" {
		p.Description += ", " + reason
	}

	if hasSession && session.IsAdmin && r.Method == http.MethodPost {
		p.Disturbance.Notes = strings.Replace(r.FormValue("notes"), "\n", "<br>", -1)
		p.Disturbance.Notes = strings.Replace(p.Disturbance.Notes, "\r", "", -1)
		err = p.Disturbance.Update(tx)
		if err != nil {
			webLog.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}

	err = webtemplate.ExecuteTemplate(w, "disturbance.html", p)
	if err != nil {
		webLog.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
	}
}

// DisturbanceListPage serves a page with a list of disturbances
func DisturbanceListPage(w http.ResponseWriter, r *http.Request) {
	tx, err := rootSqalxNode.Beginx()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		webLog.Println(err)
		return
	}
	defer tx.Commit()

	type perLine struct {
		Line              *dataobjects.Line
		TotalHoursDown    float32
		HoursDown         []float32
		TotalAvailability float32
		Availability      []float32
	}

	p := struct {
		PageCommons
		Disturbances []*dataobjects.Disturbance
		PerLine      []perLine
		Dates        []time.Time
		AverageSpeed float64
		CurPageTime  time.Time
		HasPrevPage  bool
		PrevPageTime time.Time
		HasNextPage  bool
		NextPageTime time.Time
	}{}

	var startDate time.Time
	loc, _ := time.LoadLocation("Europe/Lisbon")
	now := time.Now().In(loc)
	if mux.Vars(r)["month"] == "" {
		startDate = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, loc)
	} else {
		year, err := strconv.Atoi(mux.Vars(r)["year"])
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		month, err := strconv.Atoi(mux.Vars(r)["month"])
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if month > 12 || month < 1 {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		startDate = time.Date(year, time.Month(month), 1, 0, 0, 0, 0, loc)
	}
	endDate := startDate.AddDate(0, 1, 0)

	p.PageCommons, err = InitPageCommons(tx, w, r,
		fmt.Sprintf("Perturbações do Metro de Lisboa em %s",
			monday.Format(startDate, "January de 2006", monday.LocalePtPT)))
	if err != nil {
		webLog.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	p.CurPageTime = startDate
	p.NextPageTime = endDate
	p.PrevPageTime = startDate.AddDate(0, 0, -1)
	p.HasPrevPage = p.PrevPageTime.After(time.Date(2017, 3, 1, 0, 0, 0, 0, loc))
	p.HasNextPage = p.NextPageTime.Before(now)

	p.Description = fmt.Sprintf("Perturbações do %s para %s: estatísticas e histórico completo",
		"Metro de Lisboa", // TODO unhardcode this one day
		monday.Format(startDate, "January de 2006", monday.LocalePtPT))

	p.Disturbances, err = dataobjects.GetDisturbancesBetween(tx, startDate, endDate, p.OfficialOnly)
	if err != nil {
		webLog.Println(err)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	for d := startDate; d.Before(endDate); d = d.AddDate(0, 0, 1) {
		p.Dates = append(p.Dates, d)
	}

	for _, line := range p.Lines {
		lineInfo := perLine{
			Line: line.Line,
		}

		totalDuration, _, err := line.DisturbanceDuration(tx, startDate, endDate, p.OfficialOnly)
		if err != nil {
			webLog.Println(err)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		lineInfo.TotalHoursDown = float32(totalDuration.Hours())

		totalAvailability, _, err := line.Availability(tx, startDate, endDate, p.OfficialOnly)
		lineInfo.TotalAvailability = float32(totalAvailability * 100)

		for d := startDate; d.Before(endDate); d = d.AddDate(0, 0, 1) {
			nd := d.AddDate(0, 0, 1)
			availability, _, err := line.Availability(tx, d, nd, p.OfficialOnly)
			if err != nil {
				webLog.Println(err)
				w.WriteHeader(http.StatusNotFound)
				return
			}
			duration, _, err := line.DisturbanceDuration(tx, d, nd, p.OfficialOnly)
			lineInfo.Availability = append(lineInfo.Availability, float32(availability*100))
			lineInfo.HoursDown = append(lineInfo.HoursDown, float32(duration.Hours()))
		}

		p.PerLine = append(p.PerLine, lineInfo)
	}

	p.AverageSpeed, err = compute.AverageSpeedCached(tx, startDate, endDate.Truncate(24*time.Hour))
	if err == compute.ErrInfoNotReady {
		p.AverageSpeed = 0
	} else if err != nil {
		webLog.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	p.Dependencies.Charts = true
	err = webtemplate.ExecuteTemplate(w, "disturbancelist.html", p)
	if err != nil {
		webLog.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
	}
}
