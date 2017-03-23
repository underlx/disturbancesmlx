package main

import (
	"net/http"
	"text/template"
	"time"

	"tny.im/disturbancesmlx/interfaces"

	"github.com/gorilla/mux"
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

	p := struct {
		Hours int
		Days  int
		Lines []struct {
			*interfaces.Line
			Down    bool
			Minutes int
		}
	}{}
	p.Hours, p.Days, err = MLNoDisturbanceUptime(tx)
	if err != nil {
		webLog.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
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
		Down    bool
		Minutes int
	}, len(lines))

	for i := range lines {
		p.Lines[i].Line = lines[i]
		d, err := lines[i].LastOngoingDisturbance(tx)
		p.Lines[i].Down = err == nil
		if err == nil {
			p.Lines[i].Minutes = int(time.Now().Sub(d.StartTime).Minutes())
		}
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
	}

	webtemplate, _ = template.New("index.html").Funcs(funcMap).ParseGlob("web/*.html")
}
