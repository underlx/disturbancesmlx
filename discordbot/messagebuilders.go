package discordbot

import (
	"fmt"
	"math/rand"
	"net/url"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/hako/durafmt"
	"go.tianon.xyz/progress"

	"github.com/underlx/disturbancesmlx/dataobjects"
	"github.com/underlx/disturbancesmlx/utils"
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
		SetTitle("Rede __"+network.Name+"__").
		AddInlineField("Horário", schedule).
		AddInlineField("Fuso horário", loc.String()+" ("+time.Now().In(loc).Format(time.RFC3339)+")").
		AddInlineField("Mapa de rede", websiteURL+"/map"). // TODO needs un-hardcoding
		AddInlineField("Website", mainURL)

	// TODO needs un-hardcoding
	if network.ID == "pt-ml" {
		embed.SetThumbnail("https://cdn.discordapp.com/attachments/334363158661824512/469500166433669120/Metropolitano_Lisboa_logo.png")
	}

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

	loc, err := time.LoadLocation(line.Network.Timezone)
	if err != nil {
		return nil, err
	}

	now := time.Now().In(loc)
	start := now.Add(-1 * time.Hour)
	disturbances, err := line.DisturbancesBetween(tx, start, now, false)
	if err != nil {
		return nil, err
	}
	for i := len(disturbances)/2 - 1; i >= 0; i-- {
		opp := len(disturbances) - 1 - i
		disturbances[i], disturbances[opp] = disturbances[opp], disturbances[i]
	}

	// official month availability
	monthAvailability, monthDuration, err := line.Availability(tx, now.AddDate(0, -1, 0), now, true)
	if err != nil {
		return nil, err
	}
	monthAvailability *= 100
	monthAvString := fmt.Sprintf("%.03f%%", monthAvailability)
	if monthAvailability < 100 {
		monthAvString += fmt.Sprintf(", as perturbações duraram em média %.01f minutos", monthDuration.Minutes())
	}

	// unofficial month availability
	monthAvString += "\n__Com dados da comunidade:__\n"
	monthAvailability, monthDuration, err = line.Availability(tx, now.AddDate(0, -1, 0), now, false)
	if err != nil {
		return nil, err
	}
	monthAvailability *= 100
	monthAvString += fmt.Sprintf("%.03f%%", monthAvailability)
	if monthAvailability < 100 {
		monthAvString += fmt.Sprintf(", as perturbações duraram em média %.01f minutos", monthDuration.Minutes())
	}

	// official week availability
	weekAvailability, weekDuration, err := line.Availability(tx, now.AddDate(0, 0, -7), now, true)
	if err != nil {
		return nil, err
	}
	weekAvailability *= 100
	weekAvString := fmt.Sprintf("%.03f%%", weekAvailability)
	if weekAvailability < 100 {
		weekAvString += fmt.Sprintf(", as perturbações duraram em média %.01f minutos", weekDuration.Minutes())
	}

	// unofficial week availability
	weekAvString += "\n__Com dados da comunidade:__\n"
	weekAvailability, weekDuration, err = line.Availability(tx, now.AddDate(0, 0, -7), now, false)
	if err != nil {
		return nil, err
	}
	weekAvailability *= 100
	weekAvString += fmt.Sprintf("%.03f%%", weekAvailability)
	if weekAvailability < 100 {
		weekAvString += fmt.Sprintf(", as perturbações duraram em média %.01f minutos", weekDuration.Minutes())
	}

	color, err := strconv.ParseInt(line.Color, 16, 32)
	if err != nil {
		return nil, err
	}

	embed := NewEmbed().
		SetTitle("Linha __" + line.Name + "__").
		SetDescription("Linha do " + line.Network.Name + " (`" + line.Network.ID + "`)").
		SetURL(websiteURL + "/l/" + line.ID).
		SetThumbnail(getEmojiURLForLine(line.ID)).
		SetColor(int(color))

	for _, disturbance := range disturbances {
		distStr := ""
		for _, status := range disturbance.Statuses {
			emoji := ""
			if !status.Source.Official {
				emoji = " 👨‍👩‍👧‍👦"
			}
			end := ""
			if !status.IsDowntime {
				end = " ✅"
			}
			distStr += fmt.Sprintf("%s%s - %s%s\n",
				status.Time.In(loc).Format("15:04"),
				emoji, status.Status, end)
		}
		distStr += "[ᴾᴱᴿᴹᴬᴸᴵᴺᴷ](" + websiteURL + "/d/" + disturbance.ID + ")"
		if !disturbance.UEnded {
			embed.AddField("Perturbação actual", distStr)
		} else {
			embed.AddField("Perturbação recente", distStr)
		}
	}

	embed.AddInlineField("Disponibilidade últimos 7 dias", weekAvString).
		AddInlineField("Disponibilidade últimos 30 dias", monthAvString)

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

	emojis := ""
	for _, line := range lines {
		emojis += getEmojiForLine(line.ID)
	}

	embed := NewEmbed().
		SetTitle(emojis + " Estação __" + station.Name + "__").
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

	connectionURLs := utils.StationConnectionURLs(station)

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
		SetTitle("Átrio **" + lobby.Name + "** de " + lobby.Station.Name).
		SetDescription(description).
		SetURL(websiteURL + "/s/" + lobby.Station.ID)

	scheduleLines := utils.SchedulesToLines(schedules)
	scheduleStr := ""
	for i, line := range scheduleLines {
		scheduleStr += line
		if i < len(scheduleLines)-1 {
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

func buildPOIMessage(id string) (*Embed, error) {
	tx, err := node.Beginx()
	if err != nil {
		return nil, err
	}
	defer tx.Commit() // read-only tx

	poi, err := dataobjects.GetPOI(tx, id)
	if err != nil {
		return nil, err
	}

	stations, err := poi.Stations(tx)
	if err != nil {
		return nil, err
	}

	description := ""

	switch poi.Type {
	case "dinning":
		description = "Restauração"
	case "police":
		description = "Esquadra de polícia"
	case "fire-station":
		description = "Quartel de bombeiros"
	case "sports":
		description = "Desporto"
	case "school":
		description = "Escola"
	case "university":
		description = "Universidade"
	case "library":
		description = "Biblioteca"
	case "airport":
		description = "Aeroporto"
	case "embassy":
		description = "Embaixada"
	case "church":
		description = "Local de culto"
	case "business":
		description = "Negócio"
	case "zoo":
		description = "Jardim Zoológico"
	case "court":
		description = "Tribunal"
	case "park":
		description = "Parque"
	case "hospital":
		description = "Hospital"
	case "monument":
		description = "Monumento"
	case "museum":
		description = "Museu"
	case "shopping-center":
		description = "Centro comercial"
	case "health-center":
		description = "Centro de saúde"
	case "bank":
		description = "Banco"
	case "viewpoint":
		description = "Miradouro"
	case "casino":
		description = "Casino"
	case "theater":
		description = "Teatro"
	case "show-room":
		description = "Salão de exposições"
	case "organization":
		description = "Organização"
	case "transportation-hub":
		description = "Hub de transportes"
	case "public-space":
		description = "Espaço público"
	case "government":
		description = "Governo"
	case "market":
		description = "Mercado"
	case "public-service":
		description = "Serviço público"
	case "institute":
		description = "Instituto"
	case "post-office":
		description = "Correios"
	case "cemetery":
		description = "Cemitério"
	case "hotel":
		description = "Alojamento"
	}

	description += fmt.Sprintf("\n[Ver no Google Maps](https://www.google.com/maps/search/?api=1&query=%f,%f)", poi.WorldCoord[0], poi.WorldCoord[1])

	embed := NewEmbed().
		SetTitle("Ponto de interesse __" + poi.Names[poi.MainLocale] + "__").
		SetDescription(description)

	if poi.URL != "" {
		embed.AddField("Website", poi.URL)
	}

	stationsStr := ""
	for _, station := range stations {
		name := station.Name
		if closed, err := station.Closed(tx); err == nil && closed {
			name = "~~" + name + "~~"
		}
		stationsStr += "[" + name + "](" + websiteURL + "/s/" + station.ID + ")" + " (`" + station.ID + "`)\n"
	}
	embed.AddField("Estações de acesso", stationsStr)
	return embed, nil
}

func buildBotStatsMessage(m *discordgo.MessageCreate) (*Embed, error) {
	uptime := time.Now().Sub(botstats.StartTime)
	uptimenice := durafmt.Parse(uptime.Truncate(time.Second))
	uptimestr := uptimenice.String()
	uptimestr = strings.Replace(uptimestr, "year", "ano", 1)
	uptimestr = strings.Replace(uptimestr, "week", "semana", 1)
	uptimestr = strings.Replace(uptimestr, "day", "dia", 1)
	uptimestr = strings.Replace(uptimestr, "hour", "hora", 1)
	uptimestr = strings.Replace(uptimestr, "minute", "minuto", 1)
	uptimestr = strings.Replace(uptimestr, "second", "segundo", 1)

	embed := NewEmbed().
		SetTitle("Estatísticas do bot").
		SetDescription(fmt.Sprintf("A funcionar há %s", uptimestr))

	guildIDlist := []string{}
	guildIDs.Range(func(key, value interface{}) bool {
		guildIDlist = append(guildIDlist, key.(string))
		return true
	})

	serversStr := fmt.Sprintf("%d servidor", len(guildIDlist))
	if len(guildIDlist) != 1 {
		serversStr += "es"
	}
	serversStr += "\n"
	serversStr += fmt.Sprintf("%d utilizador", botstats.UserCount)
	if botstats.UserCount != 1 {
		serversStr += "es"
	}
	serversStr += fmt.Sprintf(", %d dos quais ", botstats.BotCount)
	if botstats.UserCount != 1 {
		serversStr += "são bots"
	} else {
		serversStr += "é bot"
	}
	serversStr += "\n"

	serversStr += fmt.Sprintf("%d canais de texto\n", botstats.TextChannelCount)
	serversStr += fmt.Sprintf("%d canais de voz\n", botstats.VoiceChannelCount)
	serversStr += fmt.Sprintf("%d canais de mensagens directas\n", len(botstats.DMChannels))
	serversStr += fmt.Sprintf("%d canais de grupo\n", botstats.GroupDMChannelCount)

	embed.AddField("Entidades do Discord", serversStr)
	for _, handler := range messageHandlers {
		handled := handler.MessagesHandled()
		actedUpon := handler.MessagesActedUpon()

		statsStr := fmt.Sprintf("%d mensagens processadas (%.02f/minuto)\n%d mensagens atendidas (%.02f/minuto)",
			handled,
			float64(handled)/uptime.Minutes(),
			actedUpon,
			float64(actedUpon)/uptime.Minutes())

		if handled > 0 {
			statsStr += fmt.Sprintf("\n%.02f%% de atendimento", float64(actedUpon)/float64(handled)*100.0)
		}
		embed.AddField("Utilização do processador de mensagens "+handler.Name(), statsStr)
	}
	for _, handler := range reactionHandlers {
		handled := handler.ReactionsHandled()
		actedUpon := handler.ReactionsActedUpon()

		statsStr := fmt.Sprintf("%d reacções processadas (%.02f/minuto)\n%d reacções atendidas (%.02f/minuto)",
			handled,
			float64(handled)/uptime.Minutes(),
			actedUpon,
			float64(actedUpon)/uptime.Minutes())

		if handled > 0 {
			statsStr += fmt.Sprintf("\n%.02f%% de atendimento", float64(actedUpon)/float64(handled)*100.0)
		}
		embed.AddField("Utilização do processador de reacções "+handler.Name(), statsStr)
	}

	embed.Timestamp = time.Now().Format(time.RFC3339Nano)

	addMuteEmbed(embed, m.ChannelID)

	return embed, nil
}

func buildStatsMessage() (*Embed, error) {
	embed := NewEmbed().
		SetTitle("Estatísticas do servidor")

	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	memStr := fmt.Sprintf("Alloc: %d bytes (%.02f MiB)\n", m.Alloc, float64(m.Alloc)/(1024*1024))
	memStr += fmt.Sprintf("TotalAlloc: %d bytes (%.02f MiB)\n", m.TotalAlloc, float64(m.TotalAlloc)/(1024*1024))
	memStr += fmt.Sprintf("Sys: %d bytes (%.02f MiB)\n", m.Sys, float64(m.Sys)/(1024*1024))
	memStr += fmt.Sprintf("PauseTotalNs: %d ns (%.04f s)\n", m.PauseTotalNs, float64(m.PauseTotalNs)/(1000000000))
	memStr += fmt.Sprintf("HeapObjects: %d (%d mallocs, %d frees)", m.HeapObjects, m.Mallocs, m.Frees)

	embed.AddField("MemStats", memStr)

	dbConnections, apiRequests := cmdReceiver.GetStats()

	uptime := time.Now().Sub(botstats.StartTime)

	apiStr := fmt.Sprintf("%d pedidos (%.02f/minuto)", apiRequests, float64(apiRequests)/uptime.Minutes())
	embed.AddField("API", apiStr)

	dbStr := fmt.Sprintf("%d ligações abertas", dbConnections)
	embed.AddField("Database", dbStr)

	embed.Timestamp = time.Now().Format(time.RFC3339Nano)

	return embed, nil
}

func buildAboutMessage(s *discordgo.Session, m *discordgo.MessageCreate) (*Embed, error) {
	tx, err := node.Beginx()
	if err != nil {
		return nil, err
	}
	defer tx.Commit() // read-only tx

	gitCommit, buildDate := cmdReceiver.GetVersion()

	embed := NewEmbed().
		SetTitle("Informação do serviço").
		SetDescription(fmt.Sprintf("Servidor compilado a partir da commit [%s](https://github.com/underlx/disturbancesmlx/commit/%s) em %s.",
			gitCommit, gitCommit, buildDate)).
		SetThumbnail(s.State.User.AvatarURL("128"))

	datasets, err := dataobjects.GetDatasets(tx)
	if err != nil {
		return nil, err
	}

	datasetsStr := ""
	for _, dataset := range datasets {
		datasetsStr += "__" + dataset.Network.Name + "__ (`" + dataset.Network.ID + "`)\n"
		datasetsStr += "\tVersão: " + dataset.Version + "\n"
		datasetsStr += "\tAutores: " + strings.Join(dataset.Authors, ", ") + "\n"
	}

	embed.AddField("Datasets (mapas de rede)", datasetsStr)
	embed.Timestamp = time.Now().Format(time.RFC3339Nano)

	addMuteEmbed(embed, m.ChannelID)

	return embed, nil
}

func buildPosPlayXPMessage(m *discordgo.MessageCreate) (*Embed, error) {
	info, err := ThePosPlayBridge.PlayerXPInfo(m.Author.ID)
	var embed *Embed
	if err != nil {
		embed = NewEmbed().
			SetTitle("⚠ Não é um jogador do <:posplay:499252980273381376> PosPlay").
			SetDescription("[Inscreva-se no PosPlay](https://perturbacoes.pt/posplay/)")
	} else {
		bar := progress.NewBar(nil)
		bar.Min = 0
		bar.Max = 10000
		bar.Val = int64(info.LevelProgress * 100)

		bar.Prefix = func(_ *progress.Bar) string {
			return ""
		}
		bar.Suffix = func(b *progress.Bar) string {
			return ""
		}

		bar.Phases = []string{
			"·",
			"▌",
			"█",
		}

		desc := fmt.Sprintf("Nível **%d**`%s`%d\n", info.Level, bar.TickString(20), info.Level+1)
		desc += fmt.Sprintf("%d XP - **%dº** lugar", info.XP, info.Rank)
		embed = NewEmbed().
			SetTitle(fmt.Sprintf("<:posplay:499252980273381376> **%s** no PosPlay", info.Username)).
			SetDescription(desc).
			SetThumbnail(info.AvatarURL)
		if info.RankThisWeek > 0 {
			embed.AddField("Esta semana", fmt.Sprintf("%d XP - **%d**º lugar", info.XPthisWeek, info.RankThisWeek))
		} else {
			embed.AddField("Esta semana", "Ainda não jogou esta semana.")
		}
	}

	embed.SetFooter("PosPlay, o jogo do UnderLX", "https://cdn.discordapp.com/emojis/499252980273381376.png")
	embed.Timestamp = time.Now().Format(time.RFC3339Nano)

	return embed, nil
}

func addMuteEmbed(embed *Embed, channelID string) {
	if muteManager.MutedPermanently(channelID) {
		embed.AddField("Estou em modo silencioso permanente neste canal", "Apenas irei responder a comandos directos 🤐")
	} else if muteManager.MutedTemporarily(channelID) {
		embed.AddField("Estou em modo silencioso neste canal. Diga `"+commandLib.prefix+"unmute` para me deixar falar mais.", "Apenas irei responder a comandos directos 🤐")
	}
}
