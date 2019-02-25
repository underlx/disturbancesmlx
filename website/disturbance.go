package website

import (
	"net/http"
	"strconv"
	"strings"
	"time"

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

	p := struct {
		PageCommons
		Disturbances    []*dataobjects.Disturbance
		DowntimePerLine map[string]float32
		AverageSpeed    float64
		CurPageTime     time.Time
		HasPrevPage     bool
		PrevPageTime    time.Time
		HasNextPage     bool
		NextPageTime    time.Time
	}{}

	p.PageCommons, err = InitPageCommons(tx, w, r, "Perturbações do Metro de Lisboa")
	if err != nil {
		webLog.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

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
	p.CurPageTime = startDate
	p.NextPageTime = endDate
	p.PrevPageTime = startDate.AddDate(0, 0, -1)
	p.HasPrevPage = p.PrevPageTime.After(time.Date(2017, 3, 1, 0, 0, 0, 0, loc))
	p.HasNextPage = p.NextPageTime.Before(now)

	p.Disturbances, err = dataobjects.GetDisturbancesBetween(tx, startDate, endDate)
	if err != nil {
		webLog.Println(err)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	p.DowntimePerLine = make(map[string]float32)
	for _, line := range p.Lines {
		totalDuration, _, err := line.DisturbanceDuration(tx, startDate, endDate, p.OfficialOnly)
		if err != nil {
			webLog.Println(err)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		p.DowntimePerLine[line.ID] += float32(totalDuration.Hours())
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
