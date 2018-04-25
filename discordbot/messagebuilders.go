package discordbot

import (
	"fmt"
	"math/rand"
	"net/url"
	"time"

	"github.com/underlx/disturbancesmlx/dataobjects"
	"github.com/underlx/disturbancesmlx/resource"
)

func buildNetworkMessage(id string) (*Embed, error) {
	tx, err := node.Beginx()
	if err != nil {
		return nil, err
	}
	defer tx.Commit() // read-only tx

	network, err := dataobjects.GetNetwork(tx, id)
	if err != nil {
		return nil, err
	}

	// TODO can probably evolve to handle NetworkSchedules
	openString := time.Time(network.OpenTime).Format("15:04")
	closeString := time.Time(network.OpenTime).
		Add(time.Duration(network.OpenDuration)).Format("15:04")
	schedule := fmt.Sprintf("%s - %s", openString, closeString)

	// TODO we're using the old feed URL because we don't have anything else...
	parsedURL, err := url.Parse(network.NewsURL)
	if err != nil {
		return nil, err
	}
	mainURL := parsedURL.Scheme + "://" + parsedURL.Host

	loc, _ := time.LoadLocation(network.Timezone)

	embed := NewEmbed().
		SetTitle("Rede "+network.Name).
		AddField("Mapa de rede", websiteURL+"/map"). // TODO needs un-hardcoding
		AddField("Horário", schedule).
		AddField("Fuso horário", loc.String()+" ("+time.Now().In(loc).Format(time.RFC3339)+")").
		AddField("Website", mainURL)

	stations, err := network.Stations(tx)
	if err != nil {
		return nil, err
	}
	rand.Shuffle(len(stations), func(i, j int) {
		stations[i], stations[j] = stations[j], stations[i]
	})
	selectedStations := stations
	if len(stations) > 5 {
		selectedStations = stations[:5]
	}

	stationsStr := ""
	for i, station := range selectedStations {
		name := station.Name
		if closed, err := station.Closed(tx); err == nil && closed {
			name = "~~" + name + "~~"
		}
		stationsStr += "[" + name + "](" + websiteURL + "/s/" + station.ID + ")" + " (`" + station.ID + "`)"
		if i < len(stations)-1 {
			stationsStr += ", "
		}
	}
	if len(selectedStations) < len(stations) {
		stationsStr += fmt.Sprintf("e %d outras...", len(stations)-len(selectedStations))
	}
	embed.AddField(fmt.Sprintf("%d estações", len(stations)), stationsStr)

	lines, err := network.Lines(tx)
	if err != nil {
		return nil, err
	}
	linesStr := ""
	for i, line := range lines {
		linesStr += "[" + getEmojiForLine(line.ID) + " " + line.Name + "](" + websiteURL + "/l/" + line.ID + ")" + " (`" + line.ID + "`)"
		if i < len(lines)-1 {
			linesStr += "\n"
		}
	}
	embed.AddField(fmt.Sprintf("%d linhas", len(lines)), linesStr)

	return embed, nil
}

func buildLineMessage(id string) (*Embed, error) {
	tx, err := node.Beginx()
	if err != nil {
		return nil, err
	}
	defer tx.Commit() // read-only tx

	line, err := dataobjects.GetLine(tx, id)
	if err != nil {
		return nil, err
	}

	loc, _ := time.LoadLocation(line.Network.Timezone)

	monthAvailability, monthDuration, err := line.Availability(tx, time.Now().In(loc).AddDate(0, -1, 0), time.Now().In(loc))
	if err != nil {
		return nil, err
	}
	monthAvailability *= 100
	monthAvString := fmt.Sprintf("%.03f%%", monthAvailability)
	if monthAvailability < 100 {
		monthAvString += fmt.Sprintf(", as perturbações duraram em média %.01f minutos", monthDuration.Minutes())
	}

	weekAvailability, weekDuration, err := line.Availability(tx, time.Now().In(loc).AddDate(0, 0, -7), time.Now().In(loc))
	if err != nil {
		return nil, err
	}
	weekAvailability *= 100
	weekAvString := fmt.Sprintf("%.03f%%", weekAvailability)
	if weekAvailability < 100 {
		weekAvString += fmt.Sprintf(", as perturbações duraram em média %.01f minutos", weekDuration.Minutes())
	}

	embed := NewEmbed().
		SetTitle(getEmojiForLine(line.ID)+" Linha "+line.Name).
		SetDescription("Linha do "+line.Network.Name+" (`"+line.Network.ID+"`)").
		SetURL(websiteURL+"/l/"+line.ID).
		AddField("Disponibilidade últimos 7 dias", weekAvString).
		AddField("Disponibilidade últimos 30 dias", monthAvString)

	stations, err := line.Stations(tx)
	if err != nil {
		return nil, err
	}
	stationsStrs := []string{}
	for _, station := range stations {
		name := station.Name
		if closed, err := station.Closed(tx); err == nil && closed {
			name = "~~" + name + "~~"
		}
		stationsStrs = append(stationsStrs, "["+name+"]("+websiteURL+"/s/"+station.ID+")"+" (`"+station.ID+"`)")
	}
	stationsStrPart := ""
	firstStationEmbed := true
	for i, str := range stationsStrs {
		if len(stationsStrPart)+len(str)+2 > 1024 {
			if firstStationEmbed {
				firstStationEmbed = false
				embed.AddField(fmt.Sprintf("%d estações", len(stations)), stationsStrPart)
			} else {
				embed.AddField("(continuação)", stationsStrPart)
			}
			stationsStrPart = ""
		}
		stationsStrPart += str
		if i < len(stations)-1 {
			stationsStrPart += "\n"
		}
	}
	if firstStationEmbed {
		embed.AddField(fmt.Sprintf("%d estações", len(stations)), stationsStrPart)
	} else {
		embed.AddField("(continuação)", stationsStrPart)
	}

	return embed, nil
}

func buildStationMessage(id string) (*Embed, error) {
	tx, err := node.Beginx()
	if err != nil {
		return nil, err
	}
	defer tx.Commit() // read-only tx

	station, err := dataobjects.GetStation(tx, id)
	if err != nil {
		return nil, err
	}

	lines, err := station.Lines(tx)
	if err != nil {
		return nil, err
	}

	lobbies, err := station.Lobbies(tx)
	if err != nil {
		return nil, err
	}

	description := "Estação do " + station.Network.Name + " (`" + station.Network.ID + "`)"
	if closed, err := station.Closed(tx); err == nil && closed {
		description += "\n**Esta estação encontra-se encerrada por tempo indeterminado.**"
	}

	embed := NewEmbed().
		SetTitle("Estação " + station.Name).
		SetDescription(description).
		SetURL(websiteURL + "/s/" + station.ID)

	linesStr := fmt.Sprintf("Esta estação é servida por %d linha", len(lines))
	if len(lines) != 1 {
		linesStr += "s"
	}
	linesStr += " ("
	for i, line := range lines {
		linesStr += "[" + getEmojiForLine(line.ID) + " " + line.Name + "](" + websiteURL + "/l/" + line.ID + ")"
		if i == len(lines)-2 {
			linesStr += " e "
		} else if i < len(lines)-2 {
			linesStr += ", "
		}
	}
	linesStr += ")."
	embed.AddField("Linhas", linesStr)

	lobbiesStr := fmt.Sprintf("Esta estação tem %d átrio", len(lobbies))
	if len(lobbies) != 1 {
		lobbiesStr += "s"
	}
	lobbiesStr += ":\n"
	for i, lobby := range lobbies {
		closed, err := lobby.Closed(tx)
		if err != nil {
			return nil, err
		}
		if closed {
			closed = true
			lobbiesStr += "~~"
		}
		lobbiesStr += "**" + lobby.Name + "**" + " (`" + lobby.ID + "`)"

		exits, err := lobby.Exits(tx)
		if err != nil {
			return nil, err
		}
		lobbiesStr += fmt.Sprintf(", com %d acesso", len(exits))
		if len(exits) != 1 {
			lobbiesStr += "s"
		}
		liftCount := 0
		for _, exit := range exits {
			if exit.Type == "lift" {
				liftCount++
			}
		}
		if liftCount > 0 {
			lobbiesStr += fmt.Sprintf(", %d dos quais ", liftCount)
			if liftCount != 1 {
				lobbiesStr += "são elevadores"
			} else {
				lobbiesStr += "é elevador"
			}
		}

		if closed {
			lobbiesStr += "~~ (encerrado por tempo indeterminado)"
		}
		if i < len(lobbies)-1 {
			lobbiesStr += ";\n"
		} else if i == len(lobbies)-2 {
			lobbiesStr += " e \n"
		} else {
			lobbiesStr += "."
		}
	}
	embed.AddField("Átrios", lobbiesStr)

	connectionURLs := resource.ComputeStationConnectionURLs(station)

	if len(connectionURLs) != 0 {
		connectionStr := "Esta estação tem ligação a "
		i := 0
		for key := range connectionURLs {
			name := ""
			switch key {
			case "boat":
				name = "transporte fluvial"
			case "bus":
				name = "carreiras urbanas"
			case "train":
				name = "comboios"
			case "park":
				name = "parques de estacionamento"
			case "bike":
				name = "postos de bicicletas partilhadas"
			}
			connectionStr += "[" + name + "](" + websiteURL + "/s/" + station.ID + "#" + key + ")"
			if i == len(connectionURLs)-2 {
				connectionStr += " e "
			} else if i < len(connectionURLs)-2 {
				connectionStr += ", "
			}
			i++
		}
		connectionStr += "."
		embed.AddField("Ligações", connectionStr)
	}

	return embed, nil
}

func buildLobbyMesage(id string) (*Embed, error) {
	tx, err := node.Beginx()
	if err != nil {
		return nil, err
	}
	defer tx.Commit() // read-only tx

	lobby, err := dataobjects.GetLobby(tx, id)
	if err != nil {
		return nil, err
	}

	exits, err := lobby.Exits(tx)
	if err != nil {
		return nil, err
	}

	schedules, err := lobby.Schedules(tx)
	if err != nil {
		return nil, err
	}

	description := "Átrio da estação " + lobby.Station.Name + " (`" + lobby.Station.ID + "`)"
	if closed, err := lobby.Closed(tx); err == nil && closed {
		description += "\n**Este átrio encontra-se encerrado por tempo indeterminado.**"
	}

	embed := NewEmbed().
		SetTitle("Átrio " + lobby.Name + " de " + lobby.Station.Name).
		SetDescription(description).
		SetURL(websiteURL + "/s/" + lobby.Station.ID)

	scheduleLines := schedToLines(schedules)
	scheduleStr := ""
	for i, line := range scheduleLines {
		scheduleStr += line
		if i < len(exits)-1 {
			scheduleStr += "\n"
		}
	}
	embed.AddField("Horário", scheduleStr)

	exitsStr := ""
	for i, exit := range exits {
		exitsStr += "["
		for j, street := range exit.Streets {
			exitsStr += street
			if j < len(exit.Streets)-1 {
				exitsStr += ", "
			}
		}
		exitsStr += fmt.Sprintf("](https://www.google.com/maps/search/?api=1&query=%f,%f)", exit.WorldCoord[0], exit.WorldCoord[1])
		switch exit.Type {
		case "stairs":
			exitsStr += " (escadas)"
		case "escalator":
			exitsStr += " (escadas rolantes)"
		case "ramp":
			exitsStr += " (rampa / saída nivelada)"
		case "lift":
			exitsStr += " (elevador)"
		}
		if i < len(exits)-1 {
			exitsStr += "\n"
		}
	}
	embed.AddField(fmt.Sprintf("%d acessos", len(exits)), exitsStr)

	return embed, nil
}
