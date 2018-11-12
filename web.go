package main

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/underlx/disturbancesmlx/posplay"
	"github.com/underlx/disturbancesmlx/website"
)

// WebServer runs the web server
func WebServer() {
	router := mux.NewRouter().StrictSlash(true)

	webKeybox, present := secrets.GetBox("web")
	if !present {
		webLog.Fatal("Web keybox not present in keybox")
	}

	// main perturbacoes.pt website
	website.Initialize(rootSqalxNode, webKeybox, webLog, reportHandler, vehicleHandler, statsHandler)
	website.ConfigureRouter(router.PathPrefix("/").Subrouter())

	posplayKeybox, present := secrets.GetBox("posplay")
	if !present {
		webLog.Fatal("Posplay keybox not present in keybox")
	}

	// PosPlay sub-website
	posplay.Initialize(posplay.Config{
		Keybox:     posplayKeybox,
		Log:        posplayLog,
		Store:      website.SessionStore(),
		Node:       rootSqalxNode,
		PathPrefix: website.BaseURL() + "/posplay",
		GitCommit:  GitCommit})

	posplay.ConfigureRouter(router.PathPrefix("/posplay").Subrouter())

	webLog.Println("Starting Web server...")

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
