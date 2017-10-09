package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"text/template"
	"time"

	"github.com/gbl08ma/disturbancesmlx/dataobjects"
	"github.com/heetch/sqalx"

	"encoding/json"

	"sort"

	"github.com/gorilla/mux"
	"github.com/rickb777/date"
)

var webtemplate *template.Template

type PageCommons struct {
	PageTitle string
	Lines     []struct {
		*dataobjects.Line
		Down    bool
		Minutes int
	}
	LastChangeAgoMin  int
	LastChangeAgoHour int
	LastUpdateAgoMin  int
	LastUpdateAgoHour int
}

type ConnectionData struct {
	ID   string
	HTML string
}

// WebServer starts the web server
func WebServer() {
	router := mux.NewRouter().StrictSlash(true)

	webLog.Println("Starting Web server...")

	router.HandleFunc("/", HomePage)
	router.HandleFunc("/lookingglass", LookingGlass)
	router.HandleFunc("/lookingglass/heatmap", Heatmap)
	router.HandleFunc("/d/{id:[0-9A-Fa-f]{8}-[0-9A-Fa-f]{4}-4[0-9A-Fa-f]{3}-[89ABab][0-9A-Fa-f]{3}-[0-9A-Fa-f]{12}}", DisturbancePage)
	router.HandleFunc("/s/{id:[-0-9A-Za-z]{1,36}}", StationPage)
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

func InitPageCommons(node sqalx.Node, title string) (commons PageCommons, err error) {
	tx, err := node.Beginx()
	if err != nil {
		return commons, err
	}
	defer tx.Commit() // read-only tx

	commons.PageTitle = title
	commons.LastChangeAgoMin = int(time.Now().Sub(lastChange).Minutes()) % 60
	commons.LastChangeAgoHour = int(time.Now().Sub(lastChange).Hours())
	commons.LastUpdateAgoMin = int(time.Now().Sub(mlxscr.LastUpdate()).Minutes()) % 60
	commons.LastUpdateAgoHour = int(time.Now().Sub(mlxscr.LastUpdate()).Hours())

	n, err := dataobjects.GetNetwork(tx, MLnetworkID)
	if err != nil {
		return commons, err
	}
	lines, err := n.Lines(tx)
	if err != nil {
		return commons, err
	}

	commons.Lines = make([]struct {
		*dataobjects.Line
		Down    bool
		Minutes int
	}, len(lines))

	for i := range lines {
		commons.Lines[i].Line = lines[i]
		d, err := lines[i].LastOngoingDisturbance(tx)
		commons.Lines[i].Down = err == nil
		if err == nil {
			commons.Lines[i].Minutes = int(time.Now().Sub(d.StartTime).Minutes())
		}
	}

	return commons, nil
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
		PageCommons
		PageTitle  string
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

	p.PageCommons, err = InitPageCommons(tx, "Perturbações do Metro de Lisboa")
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

	lastDisturbanceTime, err := MLlastDisturbanceTime(tx)
	if err != nil {
		webLog.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	p.Days = int(date.NewAt(time.Now().In(loc)).Sub(date.NewAt(lastDisturbanceTime.In(loc))))
	p.Hours = int(time.Since(lastDisturbanceTime).Hours())

	n, err := dataobjects.GetNetwork(tx, MLnetworkID)
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

	p.LinesExtra = make([]struct {
		DayCounts       []int
		HourCounts      []int
		LastDisturbance *dataobjects.Disturbance
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

		p.LinesExtra[i].LastDisturbance, err = lines[i].LastDisturbance(tx)
		if err != nil {
			webLog.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		sort.Slice(p.LinesExtra[i].LastDisturbance.Statuses, func(j, k int) bool {
			return p.LinesExtra[i].LastDisturbance.Statuses[j].Time.Before(p.LinesExtra[i].LastDisturbance.Statuses[k].Time)
		})

		p.LinesExtra[i].DayCounts, err = lines[i].CountDisturbancesByDay(tx, time.Now().In(loc).AddDate(0, 0, -6), time.Now().In(loc))
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
			p.LinesExtra[i].HourCounts = append(p.LinesExtra[i].HourCounts, hourCounts[j])
		}
		p.LinesExtra[i].HourCounts = append(p.LinesExtra[i].HourCounts, hourCounts[0])

		availability, avgd, err := MLlineAvailability(tx, lines[i], time.Now().In(loc).Add(-24*7*time.Hour), time.Now().In(loc))
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

// LookingGlass serves the looking glass page
func LookingGlass(w http.ResponseWriter, r *http.Request) {
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

	p, err := InitPageCommons(tx, "Perturbações do Metro de Lisboa")
	if err != nil {
		webLog.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	err = webtemplate.ExecuteTemplate(w, "lg.html", p)
	if err != nil {
		webLog.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
	}
}

// Heatmap serves data for the disturbance heatmap
func Heatmap(w http.ResponseWriter, r *http.Request) {
	startTime, err := time.Parse(time.RFC3339Nano, r.URL.Query().Get("start"))
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	endTime, err := time.Parse(time.RFC3339Nano, r.URL.Query().Get("stop"))
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	tx, err := rootSqalxNode.Beginx()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		webLog.Println(err)
		return
	}
	defer tx.Commit()

	network, err := dataobjects.GetNetwork(tx, MLnetworkID)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		webLog.Println(err)
		return
	}

	dayCounts, err := network.CountDisturbancesByHour(tx, startTime, endTime)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		webLog.Println(err)
		return
	}

	data := make(map[int64]int)

	for _, count := range dayCounts {
		data[startTime.Unix()] = count
		startTime = startTime.Add(1 * time.Hour)
	}

	json, err := json.Marshal(data)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		webLog.Println(err)
		return
	}
	w.Write(json)
}

// DisturbancePage serves the page for a specific disturbance
func DisturbancePage(w http.ResponseWriter, r *http.Request) {
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

	p := struct {
		PageCommons
		Disturbance *dataobjects.Disturbance
	}{}

	p.PageCommons, err = InitPageCommons(tx, "Perturbação do Metro de Lisboa")
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

	err = webtemplate.ExecuteTemplate(w, "disturbance.html", p)
	if err != nil {
		webLog.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
	}
}

// StationPage serves the page for a specific disturbance
func StationPage(w http.ResponseWriter, r *http.Request) {
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

	p := struct {
		PageCommons
		Station      *dataobjects.Station
		StationLines []*dataobjects.Line
		Trivia       string
		Connections  []ConnectionData
	}{}

	p.Station, err = dataobjects.GetStation(tx, mux.Vars(r)["id"])
	if err != nil {
		webLog.Println(err)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	p.StationLines, err = p.Station.Lines(tx)
	if err != nil {
		webLog.Println(err)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	p.Trivia, err = ReadStationTrivia(p.Station.ID, "pt")
	if err != nil {
		webLog.Println(err)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	p.Connections, err = ReadStationConnections(p.Station.ID)
	if err != nil {
		webLog.Println(err)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	p.PageCommons, err = InitPageCommons(tx, p.Station.Name+" - Estação do "+p.Station.Network.Name)
	if err != nil {
		webLog.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	err = webtemplate.ExecuteTemplate(w, "station.html", p)
	if err != nil {
		webLog.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
	}
}

func ReadStationTrivia(stationID, locale string) (string, error) {
	buf, err := ioutil.ReadFile("stationkb/" + locale + "/trivia/" + stationID + ".html")
	if err != nil {
		return "", err
	}
	return string(buf), nil
}

func ReadStationConnections(stationID string) (data []ConnectionData, err error) {
	connections := []string{"boat", "bus", "train"}
	for _, connection := range connections {
		path := "stationkb/en/connections/" + connection + "/" + stationID + ".html"
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			buf, err := ioutil.ReadFile(path)
			if err != nil {
				return data, err
			}
			data = append(data, ConnectionData{
				ID:   connection,
				HTML: strings.Replace(strings.Replace(string(buf), "</p>", "", -1), "<p>", "", -1),
			})
		}
	}
	return data, nil
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
