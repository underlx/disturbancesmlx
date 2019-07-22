package website

import (
	"fmt"
	"log"
	"net/http"
	"net/http/pprof"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/underlx/disturbancesmlx/ankiddie"
	"github.com/underlx/disturbancesmlx/compute"
	"github.com/underlx/disturbancesmlx/utils"

	"github.com/gbl08ma/keybox"
	"github.com/gbl08ma/sqalx"
	"github.com/gbl08ma/ssoclient"
	"github.com/medicalwei/recaptcha"
	"github.com/underlx/disturbancesmlx/dataobjects"

	"encoding/json"

	"github.com/gorilla/csrf"
	"github.com/gorilla/feeds"
	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
)

var webtemplate *template.Template
var sessionStore *sessions.CookieStore
var daClient *ssoclient.SSOClient
var webcaptcha *recaptcha.R
var websiteURL string
var webLog *log.Logger
var rootSqalxNode sqalx.Node
var vehicleHandler *compute.VehicleHandler
var reportHandler *compute.ReportHandler
var statsHandler *compute.StatsHandler
var parentAnkiddie *ankiddie.Ankiddie
var csrfMiddleware mux.MiddlewareFunc

// PageCommons contains information that is required by most page templates
type PageCommons struct {
	CSRFfield string
	PageTitle string
	Lines     []struct {
		*dataobjects.Line
		Down     bool
		Official bool
		Minutes  int
	}
	OfficialOnly bool
	DebugBuild   bool
	Dependencies PageDependencies
}

// PageDependencies is used by the header template to include certain dependencies
type PageDependencies struct {
	Leaflet     bool
	Recaptcha   bool
	Charts      bool
	Flipcounter bool
}

// ConnectionData contains the HTML with the connection information for the station with ID ID
type ConnectionData struct {
	ID   string
	HTML string
}

// ConfigureRouter configures a router to handle website paths
func ConfigureRouter(router *mux.Router) {
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
	router.HandleFunc("/meta/stats", MetaStatsPage)
	router.HandleFunc("/map", MapPage)
	router.HandleFunc("/about", AboutPage)
	router.HandleFunc("/donate", DonatePage)
	router.HandleFunc("/privacy", PrivacyPolicyPage)
	router.HandleFunc("/privacy/{lang:[a-z]{2}}", PrivacyPolicyPage)
	router.HandleFunc("/terms", TermsPage)
	router.HandleFunc("/terms/{lang:[a-z]{2}}", TermsPage)
	router.HandleFunc("/feed", RSSFeed)
	router.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("static/"))))

	router.HandleFunc("/auth", AuthHandler)
	router.HandleFunc("/auth/logout", AuthLogoutHandler)
	router.HandleFunc("/dotAccount64Logo.png", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "static/img/logo.png")
	})

	router.HandleFunc("/debug/pprof/", pprof.Index)
	router.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	router.HandleFunc("/debug/pprof/profile", pprof.Profile)
	router.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	router.HandleFunc("/debug/pprof/trace", pprof.Trace)
	router.PathPrefix("/debug/pprof/").HandlerFunc(pprof.Index)

	if DEBUG {
		router.Use(templateReloadingMiddleware)
	}
	router.Use(csrfMiddleware)
}

// Initialize initializes the package
func Initialize(snode sqalx.Node, webKeybox *keybox.Keybox, log *log.Logger,
	rh *compute.ReportHandler, vh *compute.VehicleHandler, sh *compute.StatsHandler,
	a *ankiddie.Ankiddie) {
	webLog = log
	rootSqalxNode = snode
	reportHandler = rh
	vehicleHandler = vh
	statsHandler = sh
	parentAnkiddie = a

	authKey, present := webKeybox.Get("cookieAuthKey")
	cipherKey, present2 := webKeybox.Get("cookieCipherKey")
	if !present || !present2 {
		webLog.Fatal("Cookie auth/cipher keys not present in web keybox")
	}

	websiteURL, present = webKeybox.Get("websiteURL")
	if !present {
		webLog.Fatal("Website URL not present in web keybox")
	}

	recaptchakey, present := webKeybox.Get("recaptchaKey")
	if !present {
		webLog.Fatal("reCAPTCHA key not present in web keybox")
	}

	webcaptcha = &recaptcha.R{
		Secret:             recaptchakey,
		TrustXForwardedFor: true,
	}

	csrfAuthKey, present := webKeybox.Get("csrfAuthKey")
	if !present {
		webLog.Fatal("CSRF auth key not present in web keybox")
	}

	csrfOpts := []csrf.Option{csrf.FieldName(CSRFfieldName), csrf.CookieName(CSRFcookieName)}
	if DEBUG {
		csrfOpts = append(csrfOpts, csrf.Secure(false))
	}
	csrfMiddleware = csrf.Protect([]byte(csrfAuthKey), csrfOpts...)

	sessionStore = sessions.NewCookieStore(
		[]byte(authKey),
		[]byte(cipherKey))

	ssoKeybox, present := webKeybox.GetBox("sso")
	if !present {
		webLog.Fatal("SSO keybox not present in web keybox")
	}

	ssoEndpointURL, present := ssoKeybox.Get("endpoint")
	if !present {
		webLog.Fatal("SSO Endpoint URL not present in keybox")
	}
	ssoAPIkey, present := ssoKeybox.Get("key")
	if !present {
		webLog.Fatal("SSO API key not present in keybox")
	}

	ssoAPIsecret, present := ssoKeybox.Get("secret")
	if !present {
		webLog.Fatal("SSO API secret not present in keybox")
	}

	var err error
	daClient, err = ssoclient.NewSSOClient(ssoEndpointURL, ssoAPIkey, ssoAPIsecret)
	if err != nil {
		webLog.Fatalf("Failed to create SSO client: %s\n", err)
	}

	ReloadTemplates()
}

// SessionStore returns the session store used by the website
func SessionStore() *sessions.CookieStore {
	return sessionStore
}

// BaseURL returns the base URL of the website without trailing slash
func BaseURL() string {
	return websiteURL
}

func templateReloadingMiddleware(next http.Handler) http.Handler {
	ReloadTemplates()
	return next
}

// InitPageCommons fills PageCommons with the info that is required by most page templates
func InitPageCommons(node sqalx.Node, w http.ResponseWriter, r *http.Request, title string) (commons PageCommons, err error) {
	tx, err := node.Beginx()
	if err != nil {
		return commons, err
	}
	defer tx.Commit() // read-only tx

	commons.CSRFfield = string(csrf.TemplateField(r))
	commons.PageTitle = title + " | Perturbações.pt"
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

// LookingGlass serves the looking glass page
func LookingGlass(w http.ResponseWriter, r *http.Request) {
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

	p.Dependencies.Charts = true
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

// MapPage serves the network map page
func MapPage(w http.ResponseWriter, r *http.Request) {
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

// RSSFeed serves the RSS feed
func RSSFeed(w http.ResponseWriter, r *http.Request) {
	tx, err := rootSqalxNode.Beginx()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		webLog.Println(err)
		return
	}
	defer tx.Commit()

	feed := &feeds.Feed{
		Title:       "Perturbações do Metro de Lisboa",
		Link:        &feeds.Link{Href: websiteURL},
		Description: "Últimas perturbações do Metro de Lisboa conforme registadas pelo Perturbações.pt",
		Author:      &feeds.Author{Name: "Equipa do UnderLX", Email: "underlx@tny.im"},
		Updated:     time.Now(),
	}

	feed.Items = []*feeds.Item{}

	disturbances, err := dataobjects.GetLatestNDisturbances(tx, 20)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		webLog.Println(err)
		return
	}

	loc, _ := time.LoadLocation("Europe/Lisbon")
	for _, disturbance := range disturbances {
		description := ""
		for _, status := range disturbance.Statuses {
			description += status.Time.In(loc).Format("02 Jan 2006 15:04")
			if !status.Source.Official {
				description += " (dados da comunidade)"
			}
			description += " - " + status.Status + "\n\n"
		}
		feed.Items = append(feed.Items, &feeds.Item{
			Title:       "Perturbação do " + disturbance.Line.Network.Name + " - Linha " + disturbance.Line.Name,
			Link:        &feeds.Link{Href: websiteURL + "/d/" + disturbance.ID},
			Description: description,
			Created:     disturbance.UStartTime,
		})
	}

	rss, err := feed.ToRss()
	if err != nil {
		webLog.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
	}
	w.Write([]byte(rss))
}

// ReloadTemplates reloads the templates for the website
func ReloadTemplates() {
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
		"formatTrainFrequency": func(dd dataobjects.Duration) string {
			d := time.Duration(dd)
			d = d.Round(time.Second)
			m := d / time.Minute
			d -= m * time.Minute
			s := d / time.Second
			return fmt.Sprintf("%02d:%02d", m, s)
		},
		"formatPortugueseMonth":        utils.FormatPortugueseMonth,
		"formatPortugueseDurationLong": utils.FormatPortugueseDurationLong,
	}

	webtemplate = template.Must(template.New("index.html").Funcs(funcMap).ParseGlob("templates/*.html"))
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
