package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/gbl08ma/ssoclient"
	"github.com/heetch/sqalx"
	"github.com/medicalwei/recaptcha"
	"github.com/underlx/disturbancesmlx/dataobjects"

	"encoding/json"

	"sort"

	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	"github.com/rickb777/date"
)

var webtemplate *template.Template
var sessionStore *sessions.CookieStore
var daClient *ssoclient.SSOClient
var webcaptcha *recaptcha.R
var websiteURL string

// PageCommons contains information that is required by most page templates
type PageCommons struct {
	PageTitle string
	Lines     []struct {
		*dataobjects.Line
		Down     bool
		Official bool
		Minutes  int
	}
	LastChangeAgoMin  int
	LastChangeAgoHour int
	LastUpdateAgoMin  int
	LastUpdateAgoHour int
	OfficialOnly      bool
	DebugBuild        bool
}

// ConnectionData contains the HTML with the connection information for the station with ID ID
type ConnectionData struct {
	ID   string
	HTML string
}

// TrainETA contains information about the estimated arrival time of a train for a station and direction
type TrainETA struct {
	Station   *dataobjects.Station
	Direction *dataobjects.Station
	ETA       string
}

// WebServer starts the web server
func WebServer() {
	authKey, present := secrets.Get("cookieAuthKey")
	cipherKey, present2 := secrets.Get("cookieCipherKey")
	if !present || !present2 {
		mainLog.Fatal("Cookie auth/cipher keys not present in keybox")
	}

	websiteURL, present = secrets.Get("websiteURL")
	if !present {
		mainLog.Fatal("Website URL not present in keybox")
	}

	sessionStore = sessions.NewCookieStore(
		[]byte(authKey),
		[]byte(cipherKey))

	ssoEndpointURL, present := secrets.Get("ssoEndpoint")
	if !present {
		mainLog.Fatal("SSO Endpoint URL not present in keybox")
	}
	ssoAPIkey, present := secrets.Get("ssoAPIkey")
	if !present {
		mainLog.Fatal("SSO API key not present in keybox")
	}

	ssoAPIsecret, present := secrets.Get("ssoAPIsecret")
	if !present {
		mainLog.Fatal("SSO API secret not present in keybox")
	}

	var err error
	daClient, err = ssoclient.NewSSOClient(ssoEndpointURL, ssoAPIkey, ssoAPIsecret)
	if err != nil {
		mainLog.Fatalf("Failed to create SSO client: %s\n", err)
	}

	recaptchakey, present := secrets.Get("recaptchaKey")
	if !present {
		mainLog.Fatal("reCAPTCHA key not present in keybox")
	}

	webcaptcha = &recaptcha.R{
		Secret:             recaptchakey,
		TrustXForwardedFor: true,
	}

	router := mux.NewRouter().StrictSlash(true)

	webLog.Println("Starting Web server...")

	router.HandleFunc("/", HomePage)
	router.HandleFunc("/report", ReportPage)
	router.HandleFunc("/lookingglass", LookingGlass)
	router.HandleFunc("/lookingglass/heatmap", Heatmap)
	router.HandleFunc("/internal", InternalPage)
	router.HandleFunc("/d/{id:[0-9A-Fa-f]{8}-[0-9A-Fa-f]{4}-4[0-9A-Fa-f]{3}-[89ABab][0-9A-Fa-f]{3}-[0-9A-Fa-f]{12}}", DisturbancePage)
	router.HandleFunc("/disturbances/{id:[0-9A-Fa-f]{8}-[0-9A-Fa-f]{4}-4[0-9A-Fa-f]{3}-[89ABab][0-9A-Fa-f]{3}-[0-9A-Fa-f]{12}}", DisturbancePage)
	router.HandleFunc("/d/{year:[0-9]{4}}/{month:[0-9]{2}}", DisturbanceListPage)
	router.HandleFunc("/disturbances/{year:[0-9]{4}}/{month:[0-9]{2}}", DisturbanceListPage)
	router.HandleFunc("/d", DisturbanceListPage)
	router.HandleFunc("/disturbances", DisturbanceListPage)
	router.HandleFunc("/s/{id:[-0-9A-Za-z]{1,36}}", StationPage)
	router.HandleFunc("/stations/{id:[-0-9A-Za-z]{1,36}}", StationPage)
	router.HandleFunc("/l/{id:[-0-9A-Za-z]{1,36}}", LinePage)
	router.HandleFunc("/lines/{id:[-0-9A-Za-z]{1,36}}", LinePage)
	router.HandleFunc("/map", MapPage)
	router.HandleFunc("/about", AboutPage)
	router.HandleFunc("/donate", DonatePage)
	router.HandleFunc("/privacy", PrivacyPolicyPage)
	router.HandleFunc("/privacy/{lang:[a-z]{2}}", PrivacyPolicyPage)
	router.HandleFunc("/terms", TermsPage)
	router.HandleFunc("/terms/{lang:[a-z]{2}}", TermsPage)
	router.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("static/"))))

	router.HandleFunc("/auth", AuthHandler)
	router.HandleFunc("/auth/logout", AuthLogoutHandler)
	router.HandleFunc("/dotAccount64Logo.png", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "static/img/logo.png")
	})

	WebReloadTemplate()

	server := http.Server{
		Addr:    ":8089",
		Handler: router,
	}

	err = server.ListenAndServe()
	if err != nil {
		webLog.Println(err)
	}
	webLog.Println("Web server terminated")
}

// InitPageCommons fills PageCommons with the info that is required by most page templates
func InitPageCommons(node sqalx.Node, w http.ResponseWriter, r *http.Request, title string) (commons PageCommons, err error) {
	tx, err := node.Beginx()
	if err != nil {
		return commons, err
	}
	defer tx.Commit() // read-only tx

	commons.PageTitle = title + " | Perturbações.pt"
	commons.LastChangeAgoMin = int(time.Since(lastChange).Minutes()) % 60
	commons.LastChangeAgoHour = int(time.Since(lastChange).Hours())
	commons.LastUpdateAgoMin = int(time.Since(mlxscr.LastUpdate()).Minutes()) % 60
	commons.LastUpdateAgoHour = int(time.Since(mlxscr.LastUpdate()).Hours())
	commons.OfficialOnly = ShowOfficialDataOnly(w, r)
	commons.DebugBuild = DEBUG

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
		Down     bool
		Official bool
		Minutes  int
	}, len(lines))

	for i := range lines {
		commons.Lines[i].Line = lines[i]
		d, err := lines[i].LastOngoingDisturbance(tx, false)
		commons.Lines[i].Down = err == nil
		commons.Lines[i].Official = err == nil && d.Official
		if err == nil {
			commons.Lines[i].Minutes = int(time.Since(d.UStartTime).Minutes())
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

	lastDisturbanceTime, err := MLlastDisturbanceTime(tx, p.OfficialOnly)
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

// ReportPage serves the disturbance reporting page
func ReportPage(w http.ResponseWriter, r *http.Request) {
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
		Message         string
		MessageIsError  bool
		ReportableLines []*dataobjects.Line
	}{}

	p.PageCommons, err = InitPageCommons(tx, w, r, "Comunicar problemas na circulação")
	if err != nil {
		webLog.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	for _, line := range p.Lines {
		if closed, err := line.CurrentlyClosed(tx); err == nil && !closed {
			p.ReportableLines = append(p.ReportableLines, line.Line)
		}
	}

	if r.Method == http.MethodPost {
		err := r.ParseForm()
		if err != nil {
			webLog.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if !webcaptcha.Verify(*r) {
			p.Message = "A verificação do reCAPTCHA falhou."
			p.MessageIsError = true
		} else {
			oneSucceeded := false
			for _, value := range r.Form["lines"] {
				line, err := dataobjects.GetLine(tx, value)
				if err != nil {
					webLog.Println(err)
					w.WriteHeader(http.StatusBadRequest)
					return
				}

				report := dataobjects.NewLineDisturbanceReport(GetClientIP(r), line, "general")

				err = reportHandler.HandleLineDisturbanceReport(report)
				if err == nil {
					oneSucceeded = true
				}
			}

			if len(r.Form["lines"]) == 0 {
				p.Message = "Seleccione as linhas em que verifica problemas. Se não verifica problemas em nenhuma linha, não comunique nada. Agradecemos a sua participação."
				p.MessageIsError = true
			} else if oneSucceeded {
				p.Message = "Relato registado. Agradecemos a sua participação."
				p.MessageIsError = false
			} else {
				p.Message = "O seu relato para este problema já tinha sido registado recentemente. Agradecemos a sua participação."
				p.MessageIsError = true
			}
		}
	}

	err = webtemplate.ExecuteTemplate(w, "report.html", p)
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

	p, err := InitPageCommons(tx, w, r, "Perturbações do Metro de Lisboa")
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
	if mux.Vars(r)["month"] == "" {
		startDate = time.Date(time.Now().Year(), time.Now().Month(), 1, 0, 0, 0, 0, loc)
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
	p.HasNextPage = p.NextPageTime.Before(time.Now())

	p.Disturbances, err = dataobjects.GetDisturbancesBetween(tx, startDate, endDate)
	if err != nil {
		webLog.Println(err)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	p.DowntimePerLine = make(map[string]float32)
	for _, line := range p.Lines {
		totalDuration, err := line.DisturbanceDuration(tx, startDate, endDate, p.OfficialOnly)
		if err != nil {
			webLog.Println(err)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		p.DowntimePerLine[line.ID] += float32(totalDuration.Hours())
	}

	p.AverageSpeed, err = ComputeAverageSpeedCached(tx, startDate, endDate.Truncate(24*time.Hour))
	if err == ErrInfoNotReady {
		p.AverageSpeed = 0
	} else if err != nil {
		webLog.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	err = webtemplate.ExecuteTemplate(w, "disturbancelist.html", p)
	if err != nil {
		webLog.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
	}
}

// StationPage serves the page for a specific station
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
		Station        *dataobjects.Station
		StationLines   []*dataobjects.Line
		Lobbies        []*dataobjects.Lobby
		LobbySchedules [][]string
		LobbyExits     [][]*dataobjects.Exit
		Trivia         string
		Connections    []ConnectionData
		POIs           []*dataobjects.POI
		Closed         bool
		PrevNext       []struct {
			Prev *dataobjects.Station
			Next *dataobjects.Station
		}
	}{}

	p.Station, err = dataobjects.GetStation(tx, mux.Vars(r)["id"])
	if err != nil {
		webLog.Println(err)
		w.WriteHeader(http.StatusNotFound)
		return
	}
	p.Closed, err = p.Station.Closed(tx)
	if err != nil {
		webLog.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	p.StationLines, err = p.Station.Lines(tx)
	if err != nil {
		webLog.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	for _, line := range p.StationLines {
		stations, err := line.Stations(tx)
		if err != nil {
			webLog.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		for i, station := range stations {
			if station.ID == p.Station.ID {
				pn := struct {
					Prev *dataobjects.Station
					Next *dataobjects.Station
				}{}
				if i > 0 {
					pn.Prev = stations[i-1]
				}
				if i < len(stations)-1 {
					pn.Next = stations[i+1]
				}
				p.PrevNext = append(p.PrevNext, pn)
				break
			}
		}
	}

	p.POIs, err = p.Station.POIs(tx)
	if err != nil {
		webLog.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	p.Lobbies, err = p.Station.Lobbies(tx)
	if err != nil {
		webLog.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	for _, lobby := range p.Lobbies {
		schedules, err := lobby.Schedules(tx)
		if err != nil {
			webLog.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		p.LobbySchedules = append(p.LobbySchedules, schedulesToLines(schedules))

		exits, err := lobby.Exits(tx)
		if err != nil {
			webLog.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		p.LobbyExits = append(p.LobbyExits, exits)
	}

	p.Trivia, err = ReadStationTrivia(p.Station.ID, "pt")
	if err != nil {
		webLog.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	p.Connections, err = ReadStationConnections(p.Station.ID)
	if err != nil {
		webLog.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	p.PageCommons, err = InitPageCommons(tx, w, r, p.Station.Name+" - Estação do "+p.Station.Network.Name)
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

func schedulesToLines(schedules []*dataobjects.LobbySchedule) []string {
	schedulesByDay := make(map[int]*dataobjects.LobbySchedule)
	for _, schedule := range schedules {
		if schedule.Holiday && schedule.Day == 0 {
			schedulesByDay[-1] = schedule
		} else if !schedule.Holiday {
			schedulesByDay[schedule.Day] = schedule
		}
	}

	weekdaysAllTheSame := true
	for i := 2; i < 6; i++ {
		if !schedulesByDay[1].Compare(schedulesByDay[i]) {
			weekdaysAllTheSame = false
		}
	}

	holidaysAllTheSame := schedulesByDay[-1].Compare(schedulesByDay[0]) && schedulesByDay[6].Compare(schedulesByDay[0])
	allDaysTheSame := weekdaysAllTheSame && holidaysAllTheSame && schedulesByDay[-1].Compare(schedulesByDay[2])

	if allDaysTheSame {
		return []string{"Todos os dias: " + scheduleToString(schedulesByDay[0])}
	}
	scheduleString := []string{}
	if weekdaysAllTheSame {
		scheduleString = append(scheduleString, "Dias úteis: "+scheduleToString(schedulesByDay[1]))
	} else {
		for i := 2; i < 6; i++ {
			scheduleString = append(scheduleString, time.Weekday(i).String()+": "+scheduleToString(schedulesByDay[i]))
		}
	}

	if holidaysAllTheSame {
		scheduleString = append(scheduleString, "Fins de semana e feriados: "+scheduleToString(schedulesByDay[0]))
	} else {
		scheduleString = append(scheduleString, time.Weekday(0).String()+": "+scheduleToString(schedulesByDay[0]))
		scheduleString = append(scheduleString, time.Weekday(6).String()+": "+scheduleToString(schedulesByDay[6]))
		scheduleString = append(scheduleString, "Feriados: "+scheduleToString(schedulesByDay[-1]))
	}

	return scheduleString
}
func scheduleToString(schedule *dataobjects.LobbySchedule) string {
	if !schedule.Open {
		return "encerrado"
	}
	openString := time.Time(schedule.OpenTime).Format("15:04")
	closeString := time.Time(schedule.OpenTime).
		Add(time.Duration(schedule.OpenDuration)).Format("15:04")
	return fmt.Sprintf("%s - %s", openString, closeString)
}

// ReadStationTrivia returns the contents of the HTML file
// containing trivia for the specified station ID
func ReadStationTrivia(stationID, locale string) (string, error) {
	buf, err := ioutil.ReadFile("stationkb/" + locale + "/trivia/" + stationID + ".html")
	if err != nil {
		return "", err
	}
	return string(buf), nil
}

// ReadStationConnections returns the contents of HTML files
// containing connection information for the specified station ID
func ReadStationConnections(stationID string) (data []ConnectionData, err error) {
	connections := []string{"boat", "bus", "train", "park", "bike"}
	// try pt and use en as fallback
	for _, connection := range connections {
		path := "stationkb/pt/connections/" + connection + "/" + stationID + ".html"
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			buf, err := ioutil.ReadFile(path)
			if err != nil {
				return data, err
			}
			html := string(buf)
			if connection != "park" && connection != "bike" {
				html = strings.Replace(strings.Replace(string(buf), "</p>", "", -1), "<p>", "", -1)
			}
			data = append(data, ConnectionData{
				ID:   connection,
				HTML: html,
			})
		} else {
			path := "stationkb/en/connections/" + connection + "/" + stationID + ".html"
			if info, err := os.Stat(path); err == nil && !info.IsDir() {
				buf, err := ioutil.ReadFile(path)
				if err != nil {
					return data, err
				}
				html := string(buf)
				if connection != "park" {
					html = strings.Replace(strings.Replace(string(buf), "</p>", "", -1), "<p>", "", -1)
				}
				data = append(data, ConnectionData{
					ID:   connection,
					HTML: html,
				})
			}
		}
	}
	return data, nil
}

// LinePage serves the page for a specific line
func LinePage(w http.ResponseWriter, r *http.Request) {
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
		Line              *dataobjects.Line
		Stations          []*dataobjects.Station
		WeekAvailability  float64
		WeekDuration      time.Duration
		MonthAvailability float64
		MonthDuration     time.Duration
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

	loc, _ := time.LoadLocation(p.Line.Network.Timezone)

	p.MonthAvailability, p.MonthDuration, err = p.Line.Availability(tx, time.Now().In(loc).AddDate(0, -1, 0), time.Now().In(loc), p.OfficialOnly)
	if err != nil {
		webLog.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	p.MonthAvailability *= 100

	p.WeekAvailability, p.WeekDuration, err = p.Line.Availability(tx, time.Now().In(loc).AddDate(0, 0, -7), time.Now().In(loc), p.OfficialOnly)
	if err != nil {
		webLog.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	p.WeekAvailability *= 100

	err = webtemplate.ExecuteTemplate(w, "line.html", p)
	if err != nil {
		webLog.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
	}
}

// MapPage serves the network map page
func MapPage(w http.ResponseWriter, r *http.Request) {
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
	}{}

	p.PageCommons, err = InitPageCommons(tx, w, r, "Mapa de rede")
	if err != nil {
		webLog.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	err = webtemplate.ExecuteTemplate(w, "map.html", p)
	if err != nil {
		webLog.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
	}
}

// AboutPage serves the about page
func AboutPage(w http.ResponseWriter, r *http.Request) {
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
	}{}

	p.PageCommons, err = InitPageCommons(tx, w, r, "Sobre nós")
	if err != nil {
		webLog.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	err = webtemplate.ExecuteTemplate(w, "about.html", p)
	if err != nil {
		webLog.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
	}
}

// DonatePage serves the donations page
func DonatePage(w http.ResponseWriter, r *http.Request) {
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
	}{}

	p.PageCommons, err = InitPageCommons(tx, w, r, "Donativos")
	if err != nil {
		webLog.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	err = webtemplate.ExecuteTemplate(w, "donate.html", p)
	if err != nil {
		webLog.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
	}
}

// PrivacyPolicyPage serves the privacy policy page
func PrivacyPolicyPage(w http.ResponseWriter, r *http.Request) {
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
	}{}

	if mux.Vars(r)["lang"] != "en" {
		p.PageCommons, err = InitPageCommons(tx, w, r, "Política de privacidade")
	} else {
		p.PageCommons, err = InitPageCommons(tx, w, r, "Privacy Policy")
	}
	if err != nil {
		webLog.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if mux.Vars(r)["lang"] != "en" {
		err = webtemplate.ExecuteTemplate(w, "privacy.html", p)
	} else {
		err = webtemplate.ExecuteTemplate(w, "privacy-en.html", p)
	}
	if err != nil {
		webLog.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
	}
}

// TermsPage serves the terms and conditions page
func TermsPage(w http.ResponseWriter, r *http.Request) {
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
	}{}

	if mux.Vars(r)["lang"] != "en" {
		p.PageCommons, err = InitPageCommons(tx, w, r, "Termos e Condições")
	} else {
		p.PageCommons, err = InitPageCommons(tx, w, r, "Terms")
	}
	if err != nil {
		webLog.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if mux.Vars(r)["lang"] != "en" {
		err = webtemplate.ExecuteTemplate(w, "terms.html", p)
	} else {
		err = webtemplate.ExecuteTemplate(w, "terms-en.html", p)
	}
	if err != nil {
		webLog.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
	}
}

// InternalPage serves a internal page
func InternalPage(w http.ResponseWriter, r *http.Request) {
	if DEBUG {
		WebReloadTemplate()
	} else if !RequestIsTLS(r) {
		w.WriteHeader(http.StatusUpgradeRequired)
		return
	}

	hasSession, session, err := AuthGetSession(w, r, true)
	if err != nil {
		webLog.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	} else if !hasSession {
		return
	} else if !session.IsAdmin {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	message := ""
	if r.Method == http.MethodPost && r.ParseForm() == nil {
		if r.Form.Get("action") == "reloadTemplates" {
			connectionDurationCache = make(map[string]int)
			WebReloadTemplate()
			message = "Templates reloaded"
		}
	}

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

	p := struct {
		PageCommons
		StartTime  time.Time
		EndTime    time.Time
		LinesExtra []struct {
			TotalTime    string
			TotalHours   float32
			Availability string
			AvgDuration  string
		}
		AverageSpeed         float64
		Message              string
		UserID               string
		Username             string
		PassengerReadings    []PassengerReading
		TrainETAs            []TrainETA
		UsersOnlineInNetwork int
	}{
		Message:              message,
		UserID:               session.UserID,
		Username:             session.DisplayName,
		PassengerReadings:    vehicleHandler.GetReadings(),
		UsersOnlineInNetwork: statsHandler.OITInNetwork(n, 0),
		TrainETAs:            []TrainETA{},
	}

	p.PageCommons, err = InitPageCommons(tx, w, r, "Página interna")
	if err != nil {
		webLog.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	loc, _ := time.LoadLocation(n.Timezone)
	now := time.Now().In(loc)
	daysSinceMonday := now.Weekday() - time.Monday
	if daysSinceMonday < 0 {
		// it's Sunday, last Monday was 6 days ago
		daysSinceMonday = 6
	}
	p.EndTime = time.Date(now.Year(), now.Month(), now.Day()-int(daysSinceMonday), 2, 0, 0, 0, loc)
	if p.EndTime.After(now) {
		// it's Monday, but it's not 2 AM yet
		p.EndTime = p.EndTime.AddDate(0, 0, -7)
	}
	p.StartTime = p.EndTime.AddDate(0, 0, -7)

	lines, err := n.Lines(tx)
	if err != nil {
		webLog.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	p.LinesExtra = make([]struct {
		TotalTime    string
		TotalHours   float32
		Availability string
		AvgDuration  string
	}, len(lines))

	for i := range lines {
		availability, avgd, err := lines[i].Availability(tx, p.StartTime, p.EndTime, p.OfficialOnly)
		if err != nil {
			webLog.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		p.LinesExtra[i].Availability = fmt.Sprintf("%.03f%%", availability*100)
		p.LinesExtra[i].AvgDuration = fmt.Sprintf("%.01f", avgd.Minutes())
		totalDuration, err := lines[i].DisturbanceDuration(tx, p.StartTime, p.EndTime, p.OfficialOnly)
		if err != nil {
			webLog.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		p.LinesExtra[i].TotalTime = totalDuration.String()
		p.LinesExtra[i].TotalHours = float32(totalDuration.Hours())
	}

	p.AverageSpeed, err = ComputeAverageSpeedCached(tx, p.StartTime, p.EndTime)
	if err == ErrInfoNotReady {
		p.AverageSpeed = 0
	} else if err != nil {
		webLog.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// adjust time for display
	p.EndTime = p.EndTime.AddDate(0, 0, -1)

	// train ETA debugging
	p.TrainETAs, err = ComputeAllTrainETAs(tx)
	if err != nil {
		webLog.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
	}

	err = webtemplate.ExecuteTemplate(w, "internal.html", p)
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
		"stringContains": func(s, substr string) bool {
			return strings.Contains(s, substr)
		},
		"formatDisturbanceTime": func(t time.Time) string {
			loc, _ := time.LoadLocation("Europe/Lisbon")
			return t.In(loc).Format("02 Jan 2006 15:04")
		},
		"formatPortugueseMonth": func(month time.Month) string {
			switch month {
			case time.January:
				return "Janeiro"
			case time.February:
				return "Fevereiro"
			case time.March:
				return "Março"
			case time.April:
				return "Abril"
			case time.May:
				return "Maio"
			case time.June:
				return "Junho"
			case time.July:
				return "Julho"
			case time.August:
				return "Agosto"
			case time.September:
				return "Setembro"
			case time.October:
				return "Outubro"
			case time.November:
				return "Novembro"
			case time.December:
				return "Dezembro"
			default:
				return ""
			}
		},
	}

	webtemplate, _ = template.New("index.html").Funcs(funcMap).ParseGlob("web/*.html")
}

// RequestIsTLS returns whether a request was made over a HTTPS channel
// Looks at the appropriate headers if the server is behind a proxy
func RequestIsTLS(r *http.Request) bool {
	if r.Header.Get("X-Forwarded-Proto") == "https" || r.Header.Get("X-Forwarded-Proto") == "HTTPS" {
		return true
	}
	return r.TLS != nil
}

// ShowOfficialDataOnly analyzes and modifies a request to check if the user wants to see
// official disturbance data only or not, and returns the result
func ShowOfficialDataOnly(w http.ResponseWriter, r *http.Request) bool {
	value, present := r.URL.Query()["officialonly"]
	if present {
		ret := false
		if n, err := strconv.Atoi(value[0]); value[0] == "true" || (err == nil && n != 0) {
			ret = true
		}
		// this is a silly preference, HttpOnly can be false
		cookie := &http.Cookie{
			Name:     "officialonly",
			Value:    strconv.FormatBool(ret),
			HttpOnly: false,
			MaxAge:   60 * 60 * 24 * 365,
		}
		http.SetCookie(w, cookie)
		return ret
	}
	cookie, err := r.Cookie("officialonly")
	if err != nil {
		return false
	}

	return cookie.Value == "true"
}

// GetClientIP retrieves the client IP address from the request information.
// It detects common proxy headers to return the actual client's IP and not the proxy's.
func GetClientIP(r *http.Request) (ip string) {
	var pIPs string
	var pIPList []string

	if pIPs = r.Header.Get("X-Real-Ip"); pIPs != "" {
		pIPList = strings.Split(pIPs, ",")
		ip = strings.TrimSpace(pIPList[0])

	} else if pIPs = r.Header.Get("Real-Ip"); pIPs != "" {
		pIPList = strings.Split(pIPs, ",")
		ip = strings.TrimSpace(pIPList[0])

	} else if pIPs = r.Header.Get("X-Forwarded-For"); pIPs != "" {
		pIPList = strings.Split(pIPs, ",")
		ip = strings.TrimSpace(pIPList[0])

	} else if pIPs = r.Header.Get("X-Forwarded"); pIPs != "" {
		pIPList = strings.Split(pIPs, ",")
		ip = strings.TrimSpace(pIPList[0])

	} else if pIPs = r.Header.Get("Forwarded-For"); pIPs != "" {
		pIPList = strings.Split(pIPs, ",")
		ip = strings.TrimSpace(pIPList[0])

	} else if pIPs = r.Header.Get("Forwarded"); pIPs != "" {
		pIPList = strings.Split(pIPs, ",")
		ip = strings.TrimSpace(pIPList[0])

	} else {
		ip = r.RemoteAddr
	}

	return strings.Split(ip, ":")[0]
}
