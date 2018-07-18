package discordbot

import (
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

type lightTrigger struct {
	wordType wordType
	id       string
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
	handledCount           int
	actedUponCount         int
	wordMap                map[string]wordType
	lightTriggersMap       map[string]lightTrigger
	lightTriggersLastUsage map[lastUsageKey]time.Time // maps lightTrigger IDs to the last time they were used
	node                   sqalx.Node
}

// NewInfoHandler returns a new InfoHandler
func NewInfoHandler(snode sqalx.Node) (*InfoHandler, error) {
	i := &InfoHandler{
		wordMap:                make(map[string]wordType),
		lightTriggersMap:       make(map[string]lightTrigger),
		lightTriggersLastUsage: make(map[lastUsageKey]time.Time),
		node: snode,
	}
	err := i.buildWordMap()
	if err != nil {
		return nil, err
	}
	return i, nil
}

// Handle attempts to handle the provided message;
// always returns false as this is a non-authoritative handler
func (i *InfoHandler) Handle(s *discordgo.Session, m *discordgo.MessageCreate, muted bool) bool {
	i.handledCount++
	if muted {
		return false
	}

	words := strings.Split(m.Content, " ")
	for _, word := range words {
		if wordType, ok := i.wordMap[word]; ok {
			i.sendReply(s, m, word, word, wordType, false)
		}
	}

	for lightTrigger, triggerInfo := range i.lightTriggersMap {
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
			if t, ok := i.lightTriggersLastUsage[key]; ok && time.Since(t) < 10*time.Minute {
				continue
			}
			i.lightTriggersLastUsage[key] = time.Now()
			i.sendReply(s, m, triggerInfo.id, triggerWord, triggerInfo.wordType, true)
		}
	}

	return false
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

func (i *InfoHandler) buildWordMap() error {
	tx, err := i.node.Beginx()
	if err != nil {
		return err
	}
	defer tx.Commit() // read-only tx

	networks, err := dataobjects.GetNetworks(tx)
	if err != nil {
		return err
	}
	for _, network := range networks {
		i.wordMap[network.ID] = wordTypeNetwork
	}

	lines, err := dataobjects.GetLines(tx)
	if err != nil {
		return err
	}
	for _, line := range lines {
		i.wordMap[line.ID] = wordTypeLine
		i.lightTriggersMap["linha "+line.Name] = lightTrigger{
			wordType: wordTypeLine,
			id:       line.ID}
	}

	stations, err := dataobjects.GetStations(tx)
	if err != nil {
		return err
	}
	for _, station := range stations {
		i.wordMap[station.ID] = wordTypeStation
		triggers := []string{
			"estação do " + station.Name,
			"estação da " + station.Name,
			"estação de " + station.Name,
			"estação " + station.Name,
		}
		for _, trigger := range triggers {
			i.lightTriggersMap[trigger] = lightTrigger{
				wordType: wordTypeStation,
				id:       station.ID}
		}
	}

	lobbies, err := dataobjects.GetLobbies(tx)
	if err != nil {
		return err
	}
	for _, lobby := range lobbies {
		i.wordMap[lobby.ID] = wordTypeLobby
	}

	pois, err := dataobjects.GetPOIs(tx)
	if err != nil {
		return err
	}
	for _, poi := range pois {
		i.wordMap[poi.ID] = wordTypePOI
	}

	return nil
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
	}

	if err == nil && embed != nil {
		embed.SetFooter(origTrigger + " | " +
			strings.Replace(
				footerMessages[rand.Intn(len(footerMessages))],
				"{prefix}", commandLib.prefix, -1))
		embed.Timestamp = time.Now().Format(time.RFC3339Nano)
		msgSend := &discordgo.MessageSend{
			Embed: embed.MessageEmbed,
		}
		if isTemp {
			msgSend.Content = "Irei **eliminar** esta mensagem dentro de **10 segundos** a menos que um humano lhe adicione uma **reação** ⏰"
		}

		message, err := s.ChannelMessageSendComplex(m.ChannelID, msgSend)
		if err == nil && isTemp {
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
			tempMessages.Store(message.ID, ch)
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
				tempMessages.Delete(message.ID)
			}()
		}

		i.actedUponCount++
	} else {
		botLog.Println(err)
	}
}
