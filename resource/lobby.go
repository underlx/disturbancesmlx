package resource

import (
	"github.com/gbl08ma/disturbancesmlx/dataobjects"
	"github.com/heetch/sqalx"
	"github.com/yarf-framework/yarf"
)

// Lobby composites resource
type Lobby struct {
	resource
}

type apiLobby struct {
	ID      string               `msgpack:"id" json:"id"`
	Name    string               `msgpack:"name" json:"name"`
	Station *dataobjects.Station `msgpack:"-" json:"-"`
}

type apiLobbySchedule struct {
	Lobby        *dataobjects.Lobby   `msgpack:"-" json:"-"`
	Holiday      bool                 `msgpack:"holiday" json:"holiday"`
	Day          int                  `msgpack:"day" json:"day"`
	Open         bool                 `msgpack:"open" json:"open"`
	OpenTime     dataobjects.Time     `msgpack:"openTime" json:"openTime"`
	OpenDuration dataobjects.Duration `msgpack:"duration" json:"duration"`
}

type exitWrapper struct {
	ID         int        `msgpack:"id" json:"id"`
	WorldCoord [2]float64 `msgpack:"worldCoord" json:"worldCoord"`
	Streets    []string   `msgpack:"streets" json:"streets"`
}

type apiLobbyWrapper struct {
	apiLobby  `msgpack:",inline"`
	NetworkID string             `msgpack:"network" json:"network"`
	StationID string             `msgpack:"station" json:"station"`
	Exits     []exitWrapper      `msgpack:"exits" json:"exits"`
	Schedule  []apiLobbySchedule `msgpack:"schedule" json:"schedule"`
}

func (r *Lobby) WithNode(node sqalx.Node) *Lobby {
	r.node = node
	return r
}

func (n *Lobby) Get(c *yarf.Context) error {
	tx, err := n.Beginx()
	if err != nil {
		return err
	}
	defer tx.Commit() // read-only tx

	if c.Param("id") != "" {
		lobby, err := dataobjects.GetLobby(tx, c.Param("id"))
		if err != nil {
			return err
		}
		data := apiLobbyWrapper{
			apiLobby:  apiLobby(*lobby),
			NetworkID: lobby.Station.Network.ID,
			StationID: lobby.Station.ID,
		}

		data.Exits = []exitWrapper{}
		exits, err := lobby.Exits(tx)
		if err != nil {
			return err
		}
		for _, exit := range exits {
			data.Exits = append(data.Exits, exitWrapper{
				ID:         exit.ID,
				WorldCoord: exit.WorldCoord,
				Streets:    exit.Streets,
			})
		}

		data.Schedule = []apiLobbySchedule{}
		schedules, err := lobby.Schedules(tx)
		if err != nil {
			return err
		}
		for _, s := range schedules {
			data.Schedule = append(data.Schedule, apiLobbySchedule(*s))
		}

		RenderData(c, data)
	} else {
		var lobbies []*dataobjects.Lobby
		var err error
		if c.Param("sid") != "" {
			lobbies, err = dataobjects.GetLobbiesForStation(tx, c.Param("sid"))
		} else {
			lobbies, err = dataobjects.GetLobbies(tx)
		}
		if err != nil {
			return err
		}
		apilobbies := make([]apiLobbyWrapper, len(lobbies))
		for i := range lobbies {
			apilobbies[i] = apiLobbyWrapper{
				apiLobby:  apiLobby(*lobbies[i]),
				NetworkID: lobbies[i].Station.Network.ID,
				StationID: lobbies[i].Station.ID,
			}

			apilobbies[i].Exits = []exitWrapper{}
			exits, err := lobbies[i].Exits(tx)
			if err != nil {
				return err
			}
			for _, exit := range exits {
				apilobbies[i].Exits = append(apilobbies[i].Exits, exitWrapper{
					ID:         exit.ID,
					WorldCoord: exit.WorldCoord,
					Streets:    exit.Streets,
				})
			}

			apilobbies[i].Schedule = []apiLobbySchedule{}
			schedules, err := lobbies[i].Schedules(tx)
			if err != nil {
				return err
			}
			for _, s := range schedules {
				apilobbies[i].Schedule = append(apilobbies[i].Schedule, apiLobbySchedule(*s))
			}
		}
		RenderData(c, apilobbies)
	}
	return nil
}
