package main

import (
	"fmt"
	"math/rand"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
	"unicode"

	"github.com/underlx/disturbancesmlx/dataobjects"
	"github.com/underlx/disturbancesmlx/resource"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"

	"github.com/bwmarrin/discordgo"
)

var discordWordMap map[string]wordType
var discordLightTriggersMap map[string]lightTrigger
var discordLightTriggersLastUsage map[discordLastUsageKey]time.Time // maps lightTrigger IDs to the last time they were used
var discordStopShutUp map[string]time.Time                          // maps channel IDs to the time when the bot can talk again

type lightTrigger struct {
	wordType wordType
	id       string
}

type discordLastUsageKey struct {
	id        string
	channelId string
}

var discordFooterMessages = []string{
	"$mute para me mandar ir dar uma volta de Metro",
	"$mute para me calar por 15 minutos",
	"$mute e fico caladinho",
	"Estou a ser chato? Simimimimimim? Então $mute",
	"$mute e também faço greve",
	"$mute e vou fazer queixinhas ao sindicato",
	"Inoportuno? Então $mute",
	"Pareço uma idiotice artificial? $mute nisso",
	"Chato para caraças? Diga $mute",
	"A tentar ter uma conversa séria? $mute e calo-me",
	"Estou demasiado extrovertido? $mute",
	"$mute para me pôr no silêncio",
	"$mute para me mandar para o castigo",
	"$mute para me mandar ver se está a chover",
}

// wordType corresponds to a type of bot trigger word
type wordType int

const (
	wordTypeNetwork = iota
	wordTypeLine    = iota
	wordTypeStation = iota
	wordTypeLobby   = iota
	wordTypePOI     = iota
)

// DiscordBot starts the Discord bot if it is enabled in the settings
func DiscordBot() {
	rand.Seed(time.Now().Unix())
	discordToken, present := secrets.Get("discordToken")
	if !present {
		discordLog.Println("Discord token not found, Discord functions disabled")
		return
	}
	dg, err := discordgo.New("Bot " + discordToken)
	if err != nil {
		discordLog.Println(err)
		return
	}

	err = builddiscordWordMap()
	if err != nil {
		discordLog.Println(err)
		return
	}

	discordStopShutUp = make(map[string]time.Time)

	user, err := dg.User("@me")
	if err != nil {
		discordLog.Println(err)
		return
	}
	if user.Username != "UnderLX" {
		_, err := dg.UserUpdate("", "", "UnderLX", "", "")
		if err != nil {
			discordLog.Println(err)
			return
		}
	}
	dg.AddHandler(discordMessageCreate)
	// Open a websocket connection to Discord and begin listening.
	err = dg.Open()
	if err != nil {
		discordLog.Println("error opening connection,", err)
		return
	}

	// Wait here until CTRL-C or other term signal is received.
	discordLog.Println("Bot is now running.  Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-sc

	// Cleanly close down the Discord session.
	dg.Close()
	os.Exit(0)
}

func builddiscordWordMap() error {
	discordWordMap = make(map[string]wordType)
	discordLightTriggersMap = make(map[string]lightTrigger)
	discordLightTriggersLastUsage = make(map[discordLastUsageKey]time.Time)

	tx, err := rootSqalxNode.Beginx()
	if err != nil {
		return err
	}
	defer tx.Commit() // read-only tx

	networks, err := dataobjects.GetNetworks(tx)
	if err != nil {
		return err
	}
	for _, network := range networks {
		discordWordMap[network.ID] = wordTypeNetwork
	}

	lines, err := dataobjects.GetLines(tx)
	if err != nil {
		return err
	}
	for _, line := range lines {
		discordWordMap[line.ID] = wordTypeLine
		discordLightTriggersMap["linha "+line.Name] = lightTrigger{
			wordType: wordTypeLine,
			id:       line.ID}
	}

	stations, err := dataobjects.GetStations(tx)
	if err != nil {
		return err
	}
	for _, station := range stations {
		discordWordMap[station.ID] = wordTypeStation
		discordLightTriggersMap[station.Name] = lightTrigger{
			wordType: wordTypeStation,
			id:       station.ID}
	}

	lobbies, err := dataobjects.GetLobbies(tx)
	if err != nil {
		return err
	}
	for _, lobby := range lobbies {
		discordWordMap[lobby.ID] = wordTypeLobby
	}

	pois, err := dataobjects.GetPOIs(tx)
	if err != nil {
		return err
	}
	for _, poi := range pois {
		discordWordMap[poi.ID] = wordTypePOI
	}

	return nil
}

func isMn(r rune) bool {
	return unicode.Is(unicode.Mn, r) // Mn: nonspacing marks
}

// This function will be called (due to AddHandler above) every time a new
// message is created on any channel that the autenticated bot has access to.
func discordMessageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {

	// Ignore all messages created by the bot itself
	if m.Author.ID == s.State.User.ID {
		return
	}

	if m.Content == "$mute" {
		discordStopShutUp[m.ChannelID] = time.Now().Add(15 * time.Minute)
		s.ChannelMessageSend(m.ChannelID, "🤐 por 15 minutos")
	}

	if m.Content == "$unmute" {
		discordStopShutUp[m.ChannelID] = time.Time{}
		s.ChannelMessageSend(m.ChannelID, "🤗")
	}

	if !time.Now().After(discordStopShutUp[m.ChannelID]) {
		return
	}

	words := strings.Split(m.Content, " ")
	for _, word := range words {
		if wordType, ok := discordWordMap[word]; ok {
			discordSendReply(s, m, word, word, wordType)
		}
	}

	for lightTrigger, triggerInfo := range discordLightTriggersMap {
		t := transform.Chain(norm.NFD, transform.RemoveFunc(isMn), norm.NFC)
		noDiacriticsResult, _, _ := transform.String(t, lightTrigger)
		triggerWord := ""
		if strings.Contains(m.Content, lightTrigger) {
			triggerWord = lightTrigger
		} else if strings.Contains(m.Content, strings.ToLower(lightTrigger)) {
			triggerWord = strings.ToLower(lightTrigger)
		} else if strings.Contains(m.Content, strings.ToLower(noDiacriticsResult)) {
			triggerWord = strings.ToLower(noDiacriticsResult)
		}
		if triggerWord != "" {
			key := discordLastUsageKey{
				channelId: m.ChannelID,
				id:        triggerInfo.id}
			if t, ok := discordLightTriggersLastUsage[key]; ok && time.Since(t) < 10*time.Minute {
				continue
			}
			discordLightTriggersLastUsage[key] = time.Now()
			discordSendReply(s, m, triggerInfo.id, triggerWord, triggerInfo.wordType)
		}
	}

}

func discordSendReply(s *discordgo.Session, m *discordgo.MessageCreate, trigger, origTrigger string, triggerType wordType) {
	var embed *Embed
	var err error
	switch triggerType {
	case wordTypeNetwork:
		embed, err = discordBuildNetworkMessage(trigger)
	case wordTypeLine:
		embed, err = discordBuildLineMessage(trigger)
	case wordTypeStation:
		embed, err = discordBuildStationMessage(trigger)
	case wordTypeLobby:
		embed, err = discordBuildLobbyMessage(trigger)
	}

	if err == nil && embed != nil {
		embed.SetFooter(origTrigger + " | " + discordFooterMessages[rand.Intn(len(discordFooterMessages))])
		embed.Timestamp = time.Now().Format(time.RFC3339Nano)
		s.ChannelMessageSendEmbed(m.ChannelID, embed.MessageEmbed)
	} else {
		discordLog.Println(err)
	}
}

func discordBuildNetworkMessage(id string) (*Embed, error) {
	tx, err := rootSqalxNode.Beginx()
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
		linesStr += "[" + discordGetEmojiForLine(line.ID) + " " + line.Name + "](" + websiteURL + "/l/" + line.ID + ")" + " (`" + line.ID + "`)"
		if i < len(lines)-1 {
			linesStr += "\n"
		}
	}
	embed.AddField(fmt.Sprintf("%d linhas", len(lines)), linesStr)

	return embed, nil
}

func discordBuildLineMessage(id string) (*Embed, error) {
	tx, err := rootSqalxNode.Beginx()
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
		SetTitle(discordGetEmojiForLine(line.ID)+" Linha "+line.Name).
		SetDescription("Linha do "+line.Network.Name+" (`"+line.Network.ID+"`)").
		SetURL(websiteURL+"/l/"+line.ID).
		AddField("Disponibilidade últimos 7 dias", weekAvString).
		AddField("Disponibilidade últimos 30 dias", monthAvString)

	stations, err := line.Stations(tx)
	if err != nil {
		return nil, err
	}
	stationsStr := ""
	origStations := stations
	if len(stations) > 15 {
		stations = stations[:10]
	}
	collapsed := false
	if len(stations) < len(origStations) {
		rand.Shuffle(len(origStations), func(i, j int) {
			origStations[i], origStations[j] = origStations[j], origStations[i]
		})
		stations = origStations[:10]
		collapsed = true
	}
	for i, station := range stations {
		name := station.Name
		if closed, err := station.Closed(tx); err == nil && closed {
			name = "~~" + name + "~~"
		}
		stationsStr += "[" + name + "](" + websiteURL + "/s/" + station.ID + ")" + " (`" + station.ID + "`)"

		if i < len(stations)-1 {
			if collapsed {
				stationsStr += ", "
			} else {
				stationsStr += "\n"
			}
		}
	}
	if collapsed {
		stationsStr += fmt.Sprintf(" e %d outras...", len(origStations)-len(stations))
	}
	embed.AddField(fmt.Sprintf("%d estações", len(origStations)), stationsStr)

	return embed, nil
}

func discordBuildStationMessage(id string) (*Embed, error) {
	tx, err := rootSqalxNode.Beginx()
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
		linesStr += "[" + discordGetEmojiForLine(line.ID) + " " + line.Name + "](" + websiteURL + "/l/" + line.ID + ")"
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

func discordBuildLobbyMessage(id string) (*Embed, error) {
	tx, err := rootSqalxNode.Beginx()
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

	scheduleLines := schedulesToLines(schedules)
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

func discordGetEmojiForLine(id string) string {
	switch id {
	case "pt-ml-azul":
		return "<:ml_azul:410577265420795904>"
	case "pt-ml-amarela":
		return "<:ml_amarela:410566925114933250>"
	case "pt-ml-verde":
		return "<:ml_verde:410577778862325764>"
	case "pt-ml-vermelha":
		return "<:ml_vermelha:410579362773991424>"
	}
	return ""
}
