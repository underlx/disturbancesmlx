package main

import (
	"fmt"
	"net/http"
	"text/template"
	"time"

	"tny.im/disturbancesmlx/interfaces"

	"github.com/gorilla/mux"
	"github.com/rickb777/date"
)

var webtemplate *template.Template

// WebServer starts the web server
func WebServer() {
	router := mux.NewRouter().StrictSlash(true)

	webLog.Println("Starting Web server...")

	router.HandleFunc("/", HomePage)
	router.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("static/"))))

	WebReloadTemplate()

	server := http.Server{
		Addr:    ":8089",
		Handler: router,
	}

	err := server.ListenAndServe()
	if err != nil {
		webLog.Println(err)
	}
	webLog.Println("Web server terminated")
}

// HomePage serves the home page
func HomePage(w http.ResponseWriter, r *http.Request) {
	if DEBUG {
		WebReloadTemplate()
	}
	tx, err := rootSqalxNode.Beginx()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		webLog.Println(err)
		return
	}
	defer tx.Commit()

	loc, _ := time.LoadLocation("Europe/Lisbon")
	p := struct {
		Hours int
		Days  int
		Lines []struct {
			*interfaces.Line
			Down            bool
			Minutes         int
			DayCounts       []int
			HourCounts      []int
			LastDisturbance *interfaces.Disturbance
			Availability    string
			AvgDuration     string
		}
		DayNames          []string
		LastChangeAgoMin  int
		LastChangeAgoHour int
		LastUpdateAgoMin  int
		LastUpdateAgoHour int
	}{
		LastChangeAgoMin:  int(time.Now().Sub(lastChange).Minutes()) % 60,
		LastChangeAgoHour: int(time.Now().Sub(lastChange).Hours()),
		LastUpdateAgoMin:  int(time.Now().Sub(mlxscr.LastUpdate()).Minutes()) % 60,
		LastUpdateAgoHour: int(time.Now().Sub(mlxscr.LastUpdate()).Hours()),
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

	lastDisturbanceTime, err := MLlastDisturbanceTime(tx)
	if err != nil {
		webLog.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	p.Days = int(date.NewAt(time.Now().In(loc)).Sub(date.NewAt(lastDisturbanceTime.In(loc))))
	p.Hours = int(time.Since(lastDisturbanceTime).Hours())

	n, err := interfaces.GetNetwork(tx, MLnetworkID)
	if err != nil {
		webLog.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	lines, err := n.Lines(tx)
	if err != nil {
		webLog.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	p.Lines = make([]struct {
		*interfaces.Line
		Down            bool
		Minutes         int
		DayCounts       []int
		HourCounts      []int
		LastDisturbance *interfaces.Disturbance
		Availability    string
		AvgDuration     string
	}, len(lines))

	for i := range lines {
		p.Lines[i].Line = lines[i]
		d, err := lines[i].LastOngoingDisturbance(tx)
		p.Lines[i].Down = err == nil
		if err == nil {
			p.Lines[i].Minutes = int(time.Now().Sub(d.StartTime).Minutes())
		}

		p.Lines[i].LastDisturbance, err = lines[i].LastDisturbance(tx)
		if err != nil {
			webLog.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		p.Lines[i].DayCounts, err = lines[i].CountDisturbancesByDay(tx, time.Now().In(loc).AddDate(0, 0, -6), time.Now().In(loc))
		if err != nil {
			webLog.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		hourCounts, err := lines[i].CountDisturbancesByHourOfDay(tx, time.Now().In(loc).AddDate(0, 0, -6), time.Now().In(loc))
		if err != nil {
			webLog.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		// Metro de Lisboa starts operating at 06:30 AM and stops at 01:00 AM
		for j := 6; j < 24; j++ {
			p.Lines[i].HourCounts = append(p.Lines[i].HourCounts, hourCounts[j])
		}
		p.Lines[i].HourCounts = append(p.Lines[i].HourCounts, hourCounts[0])

		availability, avgd, err := MLlineAvailability(tx, lines[i], time.Now().In(loc).Add(-24*7*time.Hour), time.Now().In(loc))
		if err != nil {
			webLog.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		p.Lines[i].Availability = fmt.Sprintf("%.03f%%", availability*100)
		p.Lines[i].AvgDuration = fmt.Sprintf("%.01f", avgd.Minutes())
	}

	err = webtemplate.ExecuteTemplate(w, "index.html", p)
	if err != nil {
		webLog.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
	}
}

// WebReloadTemplate reloads the templates for the website
func WebReloadTemplate() {
	funcMap := template.FuncMap{
		"minus": func(a, b int) int {
			return a - b
		},
		"plus": func(a, b int) int {
			return a + b
		},
		"minus64": func(a, b int64) int64 {
			return a - b
		},
		"plus64": func(a, b int64) int64 {
			return a + b
		},
		"formatDisturbanceTime": func(t time.Time) string {
			loc, _ := time.LoadLocation("Europe/Lisbon")
			return t.In(loc).Format("02 Jan 2006 15:04")
		},
	}

	webtemplate, _ = template.New("index.html").Funcs(funcMap).ParseGlob("web/*.html")
}
