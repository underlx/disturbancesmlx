package website

import (
	"io/ioutil"
	"net/http"
	"os"
	"strings"

	"github.com/gorilla/mux"
	"github.com/underlx/disturbancesmlx/dataobjects"
	"github.com/underlx/disturbancesmlx/utils"
)

// StationPage serves the page for a specific station
func StationPage(w http.ResponseWriter, r *http.Request) {
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

		p.LobbySchedules = append(p.LobbySchedules, utils.SchedulesToLines(schedules))

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

	p.Dependencies.Leaflet = true
	err = webtemplate.ExecuteTemplate(w, "station.html", p)
	if err != nil {
		webLog.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
	}
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
