package website

import (
	"fmt"
	"net/http"
	"sort"
	"time"

	"github.com/rickb777/date"
	"github.com/underlx/disturbancesmlx/dataobjects"
)

// HomePage serves the home page
func HomePage(w http.ResponseWriter, r *http.Request) {
	tx, err := rootSqalxNode.Beginx()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		webLog.Println(err)
		return
	}
	defer tx.Commit()

	n, err := dataobjects.GetNetwork(tx, MLnetworkID)
	if err != nil {
		webLog.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	loc, _ := time.LoadLocation(n.Timezone)
	p := struct {
		PageCommons
		Hours      int
		Days       int
		LinesExtra []struct {
			DayCounts       []int
			HourCounts      []int
			LastDisturbance *dataobjects.Disturbance
			Availability    string
			AvgDuration     string
		}
		DayNames []string
	}{}

	p.PageCommons, err = InitPageCommons(tx, w, r, "Perturbações do Metro de Lisboa")
	if err != nil {
		webLog.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	t := time.Now().In(loc).AddDate(0, 0, -6)
	for i := 0; i < 7; i++ {
		weekDay := ""
		switch t.Weekday() {
		case time.Sunday:
			weekDay = "dom"
		case time.Monday:
			weekDay = "seg"
		case time.Tuesday:
			weekDay = "ter"
		case time.Wednesday:
			weekDay = "qua"
		case time.Thursday:
			weekDay = "qui"
		case time.Friday:
			weekDay = "sex"
		case time.Saturday:
			weekDay = "sáb"
		}
		name := fmt.Sprintf("%s, %d", weekDay, t.Day())
		p.DayNames = append(p.DayNames, name)
		t = t.AddDate(0, 0, 1)
	}

	lastDisturbanceTime, err := n.LastDisturbanceTime(tx, p.OfficialOnly)
	if err != nil {
		webLog.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	p.Days = int(date.NewAt(time.Now().In(loc)).Sub(date.NewAt(lastDisturbanceTime.In(loc))))
	p.Hours = int(time.Since(lastDisturbanceTime).Hours())

	lines, err := n.Lines(tx)
	if err != nil {
		webLog.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	p.LinesExtra = make([]struct {
		DayCounts       []int
		HourCounts      []int
		LastDisturbance *dataobjects.Disturbance
		Availability    string
		AvgDuration     string
	}, len(lines))

	for i := range lines {
		p.Lines[i].Line = lines[i]
		d, err := lines[i].LastOngoingDisturbance(tx, false)
		p.Lines[i].Down = err == nil
		p.Lines[i].Official = err == nil && d.Official
		if err == nil {
			p.Lines[i].Minutes = int(time.Since(d.UStartTime).Minutes())
		}

		p.LinesExtra[i].LastDisturbance, err = lines[i].LastDisturbance(tx, p.OfficialOnly)
		if err != nil {
			webLog.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		sort.Slice(p.LinesExtra[i].LastDisturbance.Statuses, func(j, k int) bool {
			return p.LinesExtra[i].LastDisturbance.Statuses[j].Time.Before(p.LinesExtra[i].LastDisturbance.Statuses[k].Time)
		})

		p.LinesExtra[i].DayCounts, err = lines[i].CountDisturbancesByDay(tx, time.Now().In(loc).AddDate(0, 0, -6), time.Now().In(loc), p.OfficialOnly)
		if err != nil {
			webLog.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		hourCounts, err := lines[i].CountDisturbancesByHourOfDay(tx, time.Now().In(loc).AddDate(0, 0, -6), time.Now().In(loc), p.OfficialOnly)
		if err != nil {
			webLog.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		for j := 6; j < 24; j++ {
			p.LinesExtra[i].HourCounts = append(p.LinesExtra[i].HourCounts, hourCounts[j])
		}
		p.LinesExtra[i].HourCounts = append(p.LinesExtra[i].HourCounts, hourCounts[0])

		availability, avgd, err := lines[i].Availability(tx, time.Now().In(loc).Add(-24*7*time.Hour), time.Now().In(loc), p.OfficialOnly)
		if err != nil {
			webLog.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		p.LinesExtra[i].Availability = fmt.Sprintf("%.03f%%", availability*100)
		p.LinesExtra[i].AvgDuration = fmt.Sprintf("%.01f", avgd.Minutes())
	}

	err = webtemplate.ExecuteTemplate(w, "index.html", p)
	if err != nil {
		webLog.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
	}
}
