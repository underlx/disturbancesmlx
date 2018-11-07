package discordbot

import (
	"math/rand"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/thoas/go-funk"

	"github.com/bwmarrin/discordgo"

	"github.com/gbl08ma/sqalx"
	cedar "github.com/iohub/Ahocorasick"
	"github.com/underlx/disturbancesmlx/dataobjects"
	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

type trigger struct {
	wordType wordType
	id       string
	light    bool
	needle   string
	original string
}

type lastUsageKey struct {
	id        string
	channelID string
}

var footerMessages = []string{
	"{prefix}mute para me mandar ir dar uma volta de Metro",
	"{prefix}mute para me calar por 15 minutos",
	"{prefix}mute e fico caladinho",
	"Estou a ser chato? Simimimimimim? Então {prefix}mute",
	"{prefix}mute e também faço greve",
	"{prefix}mute e vou fazer queixinhas ao sindicato",
	"Inoportuno? Então {prefix}mute",
	"Pareço uma idiotice artificial? {prefix}mute nisso",
	"Chato para caraças? Diga {prefix}mute",
	"A tentar ter uma conversa séria? {prefix}mute e calo-me",
	"Estou demasiado extrovertido? {prefix}mute",
	"{prefix}mute para me pôr no silêncio",
	"{prefix}mute para me mandar para o castigo",
	"{prefix}mute para me mandar ver se está a chover",
}

// wordType corresponds to a type of bot trigger word
type wordType int

const (
	wordTypeNetwork = iota
	wordTypeLine
	wordTypeStation
	wordTypeLobby
	wordTypePOI
)

// A InfoHandler parses Discord messages for references to database entities
// (both natural language based and ID-based) and replies with
// information messages
type InfoHandler struct {
	handledCount            int
	actedUponCount          int
	reactionsHandledCount   int
	reactionsActedUponCount int
	triggerMatcher          *cedar.Matcher
	lightTriggersLastUsage  map[lastUsageKey]time.Time // maps lightTrigger IDs to the last time they were used
	node                    sqalx.Node
	tempMessages            sync.Map
}

// NewInfoHandler returns a new InfoHandler
func NewInfoHandler(snode sqalx.Node) (*InfoHandler, error) {
	i := &InfoHandler{
		lightTriggersLastUsage: make(map[lastUsageKey]time.Time),
		node:                   snode,
		triggerMatcher:         cedar.NewMatcher(),
	}

	err := i.buildWordMap()
	if err != nil {
		return nil, err
	}
	return i, nil
}

// HandleMessage attempts to handle the provided message;
// always returns false as this is a non-authoritative handler
func (i *InfoHandler) HandleMessage(s *discordgo.Session, m *discordgo.MessageCreate, muted bool) bool {
	i.handledCount++
	if muted {
		return false
	}
	actedUpon := false

	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	content, _, err := transform.String(t, strings.ToLower(m.Content))
	if err != nil {
		botLog.Println(err)
		return false
	}
	cbytes := []byte(content)
	matches := i.triggerMatcher.Match(cbytes)
	for _, match := range matches {
		trigger := match.Value.(trigger)
		startIdx := match.At - match.KLen + 1
		endIdx := match.At + 1

		if startIdx > 0 && !i.isWordSeparator(string(cbytes[startIdx-1:startIdx])) {
			// case like "abcpt-ml"
			continue
		}
		if endIdx < len(cbytes) && !i.isWordSeparator(string(cbytes[endIdx:endIdx+1])) {
			// case like "pt-mlabc" or "pt-ml-verde" (we want to trigger on pt-ml-verde, not just pt-ml)
			continue
		}

		if trigger.light {
			key := lastUsageKey{
				channelID: m.ChannelID,
				id:        trigger.id}
			if t, ok := i.lightTriggersLastUsage[key]; ok && time.Since(t) < 10*time.Minute {
				continue
			}
			i.lightTriggersLastUsage[key] = time.Now()
		}

		i.sendReply(s, m, trigger.id, trigger.original, trigger.wordType, trigger.light)
		actedUpon = true
	}
	if actedUpon {
		i.actedUponCount++
	}

	return false
}

func (i *InfoHandler) isWordSeparator(seq string) bool {
	return funk.ContainsString([]string{" ", ".", ",", ":", "!", "?", "\n", "\""}, seq)
}

// MessagesHandled returns the number of messages handled by this InfoHandler
func (i *InfoHandler) MessagesHandled() int {
	return i.handledCount
}

// MessagesActedUpon returns the number of messages acted upon by this InfoHandler
func (i *InfoHandler) MessagesActedUpon() int {
	return i.actedUponCount
}

// Name returns the name of this message handler
func (i *InfoHandler) Name() string {
	return "InfoHandler"
}

// HandleReaction attempts to handle the provided reaction
// always returns false as this is a non-authoritative handler
func (i *InfoHandler) HandleReaction(s *discordgo.Session, m *discordgo.MessageReactionAdd) bool {
	i.reactionsHandledCount++
	v, ok := i.tempMessages.Load(m.MessageID)
	if !ok {
		return false
	}

	ch := v.(chan interface{})
	ch <- true
	i.reactionsActedUponCount++
	return false
}

// ReactionsHandled returns the number of reactions handled by this InfoHandler
func (i *InfoHandler) ReactionsHandled() int {
	return i.reactionsHandledCount
}

// ReactionsActedUpon returns the number of reactions acted upon by this InfoHandler
func (i *InfoHandler) ReactionsActedUpon() int {
	return i.reactionsActedUponCount
}

func (i *InfoHandler) buildWordMap() error {
	tx, err := i.node.Beginx()
	if err != nil {
		return err
	}
	defer tx.Commit() // read-only tx

	// POIs before stations so all stations work (there's a POI named "estação de santa apolónia")
	// otherwise the POI keys would overwrite some station keys
	pois, err := dataobjects.GetPOIs(tx)
	if err != nil {
		return err
	}
	for _, poi := range pois {
		i.populateTriggers(trigger{
			wordType: wordTypePOI,
			id:       poi.ID},
			poi.ID)
		i.populateTriggers(trigger{
			wordType: wordTypePOI,
			id:       poi.ID,
			light:    true},
			poi.Names[poi.MainLocale])
	}

	networks, err := dataobjects.GetNetworks(tx)
	if err != nil {
		return err
	}

	for _, network := range networks {
		i.populateTriggers(trigger{
			wordType: wordTypeNetwork,
			id:       network.ID},
			network.ID)
	}

	lines, err := dataobjects.GetLines(tx)
	if err != nil {
		return err
	}
	for _, line := range lines {
		i.populateTriggers(trigger{
			wordType: wordTypeLine,
			id:       line.ID},
			line.ID)

		i.populateTriggers(trigger{
			wordType: wordTypeLine,
			id:       line.ID,
			light:    true},
			"linha "+line.Name)
	}

	stations, err := dataobjects.GetStations(tx)
	if err != nil {
		return err
	}
	for _, station := range stations {
		i.populateTriggers(trigger{
			wordType: wordTypeStation,
			id:       station.ID},
			station.ID)

		wtriggers := []string{
			"estação do " + station.Name,
			"estação da " + station.Name,
			"estação de " + station.Name,
			"estação " + station.Name,
		}
		i.populateTriggers(trigger{
			wordType: wordTypeStation,
			id:       station.ID,
			light:    true},
			wtriggers...)
	}

	lobbies, err := dataobjects.GetLobbies(tx)
	if err != nil {
		return err
	}
	for _, lobby := range lobbies {
		i.populateTriggers(trigger{
			wordType: wordTypeLobby,
			id:       lobby.ID},
			lobby.ID)
	}

	i.triggerMatcher.Compile()

	return nil
}

func (i *InfoHandler) populateTriggers(t trigger, words ...string) {
	tr := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	for _, word := range words {
		t.needle, _, _ = transform.String(tr, strings.ToLower(word))
		t.original = word
		i.triggerMatcher.Insert([]byte(t.needle), t)
	}
}

func (i *InfoHandler) sendReply(s *discordgo.Session, m *discordgo.MessageCreate, trigger, origTrigger string, triggerType wordType, isTemp bool) {
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
	case wordTypePOI:
		embed, err = buildPOIMessage(trigger)
	}

	if err != nil {
		botLog.Println(err)
		return
	} else if embed == nil {
		botLog.Println("sendReply nil embed")
		return
	}
	embed.SetFooter(origTrigger+" | "+
		strings.Replace(
			footerMessages[rand.Intn(len(footerMessages))],
			"{prefix}", commandLib.prefix, -1), "https://cdn.discordapp.com/emojis/368199195427078144.png")
	embed.Timestamp = time.Now().Format(time.RFC3339Nano)
	msgSend := &discordgo.MessageSend{
		Embed: embed.MessageEmbed,
	}
	if isTemp {
		msgSend.Content = "Irei **eliminar** esta mensagem dentro de **10 segundos** a menos que um humano lhe adicione uma **reação** ⏰"
	}

	message, err := s.ChannelMessageSendComplex(m.ChannelID, msgSend)
	if err != nil {
		botLog.Println(err)
		return
	}
	if !isTemp {
		return
	}
	go func() {
		// pre-add some reactions to make it easier for people to keep the message
		s.MessageReactionAdd(message.ChannelID, message.ID, "🇲")
		s.MessageReactionAdd(message.ChannelID, message.ID, "🇦")
		s.MessageReactionAdd(message.ChannelID, message.ID, "🇳")
		s.MessageReactionAdd(message.ChannelID, message.ID, "🇹")
		s.MessageReactionAdd(message.ChannelID, message.ID, "🇪")
		s.MessageReactionAdd(message.ChannelID, message.ID, "🇷")
	}()
	ch := make(chan interface{}, 1)
	i.tempMessages.Store(message.ID, ch)
	go func() {
		select {
		case <-ch:
			// users reacted, make message permanent
			_, err := s.ChannelMessageEdit(message.ChannelID, message.ID, "")
			if err != nil {
				botLog.Println(err)
			}
			s.MessageReactionAdd(message.ChannelID, message.ID, "🤗")
		case <-time.After(10 * time.Second):
			// delete message and forget this existed
			err := s.ChannelMessageDelete(message.ChannelID, message.ID)
			if err != nil {
				botLog.Println(err)
			}
		}
		i.tempMessages.Delete(message.ID)
	}()
}
