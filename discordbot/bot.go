package discordbot

import (
	"log"
	"math/rand"
	"strings"
	"time"
	"unicode"

	"github.com/bwmarrin/discordgo"
	"github.com/heetch/sqalx"
	"github.com/underlx/disturbancesmlx/dataobjects"
	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

var wordMap map[string]wordType
var lightTriggersMap map[string]lightTrigger
var lightTriggersLastUsage map[lastUsageKey]time.Time // maps lightTrigger IDs to the last time they were used
var stopMute map[string]time.Time                     // maps channel IDs to the time when the bot can talk again

var node sqalx.Node
var websiteURL string
var botLog *log.Logger
var session *discordgo.Session
var schedToLines func(schedules []*dataobjects.LobbySchedule) []string

type lightTrigger struct {
	wordType wordType
	id       string
}

type lastUsageKey struct {
	id        string
	channelID string
}

var botOwnerUserID string

var footerMessages = []string{
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

// Start starts the Discord bot
func Start(snode sqalx.Node, swebsiteURL, discordToken string, log *log.Logger,
	schedulesToLines func(schedules []*dataobjects.LobbySchedule) []string) error {
	node = snode
	websiteURL = swebsiteURL
	botLog = log
	schedToLines = schedulesToLines
	rand.Seed(time.Now().Unix())
	dg, err := discordgo.New("Bot " + discordToken)
	if err != nil {
		return err
	}
	session = dg

	selfApp, err := dg.Application("@me")
	if err != nil {
		return err
	}
	botOwnerUserID = selfApp.Owner.ID

	err = buildWordMap()
	if err != nil {
		return err
	}

	stopMute = make(map[string]time.Time)

	user, err := dg.User("@me")
	if err != nil {
		return err
	}
	if user.Username != "UnderLX" {
		_, err := dg.UserUpdate("", "", "UnderLX", "", "")
		if err != nil {
			return err
		}
	}
	dg.AddHandler(messageCreate)
	// Open a websocket connection to Discord and begin listening.
	return dg.Open()
}

// Stop stops the Discord bot
func Stop() {
	// Cleanly close down the Discord session.
	if session != nil {
		session.Close()
	}
}

func buildWordMap() error {
	wordMap = make(map[string]wordType)
	lightTriggersMap = make(map[string]lightTrigger)
	lightTriggersLastUsage = make(map[lastUsageKey]time.Time)

	tx, err := node.Beginx()
	if err != nil {
		return err
	}
	defer tx.Commit() // read-only tx

	networks, err := dataobjects.GetNetworks(tx)
	if err != nil {
		return err
	}
	for _, network := range networks {
		wordMap[network.ID] = wordTypeNetwork
	}

	lines, err := dataobjects.GetLines(tx)
	if err != nil {
		return err
	}
	for _, line := range lines {
		wordMap[line.ID] = wordTypeLine
		lightTriggersMap["linha "+line.Name] = lightTrigger{
			wordType: wordTypeLine,
			id:       line.ID}
	}

	stations, err := dataobjects.GetStations(tx)
	if err != nil {
		return err
	}
	for _, station := range stations {
		wordMap[station.ID] = wordTypeStation
		triggers := []string{
			"estação do " + station.Name,
			"estação da " + station.Name,
			"estação de " + station.Name,
			"estação " + station.Name,
		}
		for _, trigger := range triggers {
			lightTriggersMap[trigger] = lightTrigger{
				wordType: wordTypeStation,
				id:       station.ID}
		}
	}

	lobbies, err := dataobjects.GetLobbies(tx)
	if err != nil {
		return err
	}
	for _, lobby := range lobbies {
		wordMap[lobby.ID] = wordTypeLobby
	}

	pois, err := dataobjects.GetPOIs(tx)
	if err != nil {
		return err
	}
	for _, poi := range pois {
		wordMap[poi.ID] = wordTypePOI
	}

	return nil
}

// This function will be called (due to AddHandler above) every time a new
// message is created on any channel that the autenticated bot has access to.
func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	// Ignore all messages created by the bot itself
	if m.Author.ID == s.State.User.ID {
		return
	}

	words := strings.Split(m.Content, " ")

	if m.Content == "$mute" {
		stopMute[m.ChannelID] = time.Now().Add(15 * time.Minute)
		s.ChannelMessageSend(m.ChannelID, "🤐 por 15 minutos")
	}

	if m.Content == "$unmute" {
		stopMute[m.ChannelID] = time.Time{}
		s.ChannelMessageSend(m.ChannelID, "🤗")
	}

	if m.Author.ID == botOwnerUserID {
		if words[0] == "$setstatus" {
			handleStatus(s, m, words[1:])
		}
	}

	if !time.Now().After(stopMute[m.ChannelID]) {
		return
	}
	for _, word := range words {
		if wordType, ok := wordMap[word]; ok {
			sendReply(s, m, word, word, wordType)
		}
	}

	for lightTrigger, triggerInfo := range lightTriggersMap {
		if !strings.Contains(lightTrigger, " ") && len(m.Content) > len(lightTrigger) {
			lightTrigger = " " + lightTrigger + " "
		}
		t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
		noDiacriticsResult, _, _ := transform.String(t, lightTrigger)
		noDiacriticsMessage, _, _ := transform.String(t, strings.ToLower(m.Content))
		triggerWord := ""
		if strings.Contains(m.Content, lightTrigger) {
			triggerWord = strings.TrimSpace(lightTrigger)
		} else if needle := strings.ToLower(lightTrigger); strings.Contains(m.Content, needle) {
			triggerWord = strings.TrimSpace(needle)
		} else if needle := strings.ToLower(noDiacriticsResult); strings.Contains(m.Content, needle) {
			triggerWord = strings.TrimSpace(needle)
		} else if needle := strings.ToLower(noDiacriticsResult); strings.Contains(noDiacriticsMessage, needle) {
			triggerWord = strings.TrimSpace(needle)
		} else if needle := strings.ToLower(strings.TrimRight(noDiacriticsResult, " ")); strings.HasSuffix(noDiacriticsMessage, needle) {
			triggerWord = strings.TrimSpace(needle)
		} else if needle := strings.ToLower(strings.TrimLeft(noDiacriticsResult, " ")); strings.HasPrefix(noDiacriticsMessage, needle) {
			triggerWord = strings.TrimSpace(needle)
		}
		if triggerWord != "" {
			key := lastUsageKey{
				channelID: m.ChannelID,
				id:        triggerInfo.id}
			if t, ok := lightTriggersLastUsage[key]; ok && time.Since(t) < 10*time.Minute {
				continue
			}
			lightTriggersLastUsage[key] = time.Now()
			sendReply(s, m, triggerInfo.id, triggerWord, triggerInfo.wordType)
		}
	}

}

func handleStatus(s *discordgo.Session, m *discordgo.MessageCreate, words []string) {
	var err error
	if len(words) == 0 {
		err = s.UpdateStatus(0, "")
	} else if len(words) > 1 {
		usd := &discordgo.UpdateStatusData{
			Status: "online",
		}

		switch words[1] {
		case "playing":
			usd.Game = &discordgo.Game{
				Name: strings.Join(words[2:], " "),
				Type: discordgo.GameTypeGame,
			}
		case "streaming":
			usd.Game = &discordgo.Game{
				Type: discordgo.GameTypeGame,
				URL:  strings.Join(words[2:], " "),
			}
		case "listening":
			usd.Game = &discordgo.Game{
				Name: strings.Join(words[2:], " "),
				Type: discordgo.GameTypeListening,
			}
		case "watching":
			usd.Game = &discordgo.Game{
				Name: strings.Join(words[2:], " "),
				Type: discordgo.GameTypeWatching,
			}
		default:
			usd.Game = &discordgo.Game{
				Name: strings.Join(words[1:], " "),
				Type: discordgo.GameTypeGame,
			}
		}

		err = s.UpdateStatusComplex(*usd)
	}
	if err != nil {
		s.ChannelMessageSend(m.ChannelID, "❌ "+err.Error())
	} else {
		s.ChannelMessageSend(m.ChannelID, "✅")
	}
}

func sendReply(s *discordgo.Session, m *discordgo.MessageCreate, trigger, origTrigger string, triggerType wordType) {
	var embed *Embed
	var err error
	switch triggerType {
	case wordTypeNetwork:
		embed, err = buildNetworkMessage(trigger)
	case wordTypeLine:
		embed, err = buildLineMessage(trigger)
	case wordTypeStation:
		embed, err = buildStationMessage(trigger)
	case wordTypeLobby:
		embed, err = buildLobbyMesage(trigger)
	}

	if err == nil && embed != nil {
		embed.SetFooter(origTrigger + " | " + footerMessages[rand.Intn(len(footerMessages))])
		embed.Timestamp = time.Now().Format(time.RFC3339Nano)
		s.ChannelMessageSendEmbed(m.ChannelID, embed.MessageEmbed)
	} else {
		botLog.Println(err)
	}
}

func getEmojiForLine(id string) string {
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
