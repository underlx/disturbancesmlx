package main

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/underlx/disturbancesmlx/discordbot"
	"github.com/underlx/disturbancesmlx/posplay"
	"github.com/underlx/disturbancesmlx/utils"
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
	website.Initialize(rootSqalxNode, webKeybox, webLog, reportHandler,
		vehicleHandler, vehicleETAHandler, statsHandler, kiddie)

	posplayKeybox, present := secrets.GetBox("posplay")
	if !present {
		webLog.Fatal("Posplay keybox not present in keybox")
	}

	// PosPlay sub-website
	posplayRouter := mux.NewRouter().StrictSlash(true)
	err := posplay.Initialize(posplay.Config{
		Keybox:              posplayKeybox,
		Log:                 posplayLog,
		Store:               website.SessionStore(),
		Node:                rootSqalxNode,
		GitCommit:           GitCommit,
		SendAppNotification: SendPersonalNotification})
	if err != nil {
		posplayLog.Fatal(err)
	}

	// this order is important. see https://github.com/gorilla/mux/issues/411 (still open at the time of writing)
	//posplay.ConfigureRouter(router.PathPrefix("/posplay").Subrouter())
	posplay.ConfigureRouter(posplayRouter.PathPrefix("/").Subrouter())
	website.ConfigureRouter(router.PathPrefix("/").Subrouter())

	channel, present := webKeybox.Get("discordInviteChannel")
	fallbackInvite, present2 := webKeybox.Get("discordFallbackInvite")
	if present && present2 {
		router.HandleFunc("/discord", inviteHandler(channel, fallbackInvite))
	}

	webLog.Println("Starting Web server...")

	server := http.Server{
		Addr:    ":8089",
		Handler: router,
	}
	ppserver := http.Server{
		Addr:    ":8092",
		Handler: posplayRouter,
	}

	go func() {
		err := server.ListenAndServe()
		if err != nil {
			webLog.Println(err)
		}
		webLog.Println("Web server terminated")
	}()
	go func() {
		err := ppserver.ListenAndServe()
		if err != nil {
			posplayLog.Println(err)
		}
		webLog.Println("PosPlay web server terminated")
	}()
}

func inviteHandler(channelID, fallbackInviteURL string) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		invite, err := discordbot.CreateInvite(channelID, utils.GetClientIP(r), r.URL.Query().Get("utm_source"))
		if err != nil {
			http.Redirect(w, r, fallbackInviteURL, http.StatusTemporaryRedirect)
			return
		}
		http.Redirect(w, r, "https://discord.gg/"+invite.Code, http.StatusTemporaryRedirect)
	}
}
