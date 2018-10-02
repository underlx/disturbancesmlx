package posplay

import (
	"encoding/gob"
	"errors"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/bwmarrin/discordgo"
	uuid "github.com/satori/go.uuid"
	"github.com/underlx/disturbancesmlx/dataobjects"
	"github.com/underlx/disturbancesmlx/discordbot"

	"github.com/gbl08ma/sqalx"
	"github.com/gorilla/csrf"
	"github.com/gorilla/sessions"

	"github.com/gbl08ma/keybox"
	sq "github.com/gbl08ma/squirrel"
	"golang.org/x/oauth2"
)

var oauthConfig *oauth2.Config
var webtemplate *template.Template

var tripSubmissionsChan = make(chan string, 100)
var tripEditsChan = make(chan string, 100)

const (
	// PrivateLBPrivacy is used when users don't want to appear in leaderboards
	PrivateLBPrivacy string = "PRIVATE"
	// PublicLBPrivacy is used when users want to appear in leaderboards
	PublicLBPrivacy string = "PUBLIC"
)

const (
	// UsernameDiscriminatorNameType is used when users want to appear like gbl08ma#3988
	UsernameDiscriminatorNameType string = "USERNAME_DISCRIM"
	// UsernameNameType is used when users want to appear like gbl08ma
	UsernameNameType string = "USERNAME"
	// NicknameNameType is used when users want to appear as their nickname in the project's guild
	NicknameNameType string = "NICKNAME"
)

// Config contains runtime PosPlay subsystem configuration
type Config struct {
	Keybox     *keybox.Keybox
	Log        *log.Logger
	Store      *sessions.CookieStore
	Node       sqalx.Node
	PathPrefix string
	GitCommit  string
}

var config Config
var csrfMiddleware func(http.Handler) http.Handler

// Initialize initializes the PosPlay subsystem
func Initialize(ppconfig Config) error {
	// register Session with gob so it can be saved in cookies
	gob.Register(Session{})

	config = ppconfig
	clientID, present := config.Keybox.Get("oauthClientId")
	if !present {
		return errors.New("OAuth client ID not present in posplay keybox")
	}

	clientSecret, present := config.Keybox.Get("oauthClientSecret")
	if !present {
		return errors.New("OAuth client secret not present in posplay keybox")
	}

	csrfAuthKey, present := config.Keybox.Get("csrfAuthKey")
	if !present {
		return errors.New("CSRF auth key not present in posplay keybox")
	}

	oauthConfig = &oauth2.Config{
		RedirectURL:  config.PathPrefix + "/oauth/callback",
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Scopes:       []string{"identify"},
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://discordapp.com/api/oauth2/authorize",
			TokenURL: "https://discordapp.com/api/oauth2/token",
		},
	}

	csrfOpts := []csrf.Option{}
	csrfOpts = append(csrfOpts, csrf.FieldName(CSRFfieldName))
	if DEBUG {
		csrfOpts = append(csrfOpts, csrf.Secure(false))
	}
	csrfMiddleware = csrf.Protect([]byte(csrfAuthKey), csrfOpts...)

	discordbot.ThePosPlayEventManager.OnReactionCallback = RegisterReactionCallback

	webReloadTemplate()

	go serialProcessor()

	return nil
}

// RegisterTripSubmission schedules a trip submission for analysis
func RegisterTripSubmission(trip *dataobjects.Trip) {
	tripSubmissionsChan <- trip.ID
}

// RegisterTripFirstEdit schedules a trip resubmission (edit, confirmation) for analysis
func RegisterTripFirstEdit(trip *dataobjects.Trip) {
	tripEditsChan <- trip.ID
}

// RegisterReactionCallback gives a user a XP reward for a Discord event, if he has not received a reward for that event yet
func RegisterReactionCallback(userID, messageID string, XPreward int) bool {
	// does the user even exist in PosPlay?
	tx, err := config.Node.Beginx()
	if err != nil {
		config.Log.Println(err)
		return false
	}
	defer tx.Rollback()

	player, err := dataobjects.GetPPPlayer(tx, uidConvS(userID))
	if err != nil {
		// this user is not yet a PosPlay player
		discordbot.SendDMtoUser(userID, &discordgo.MessageSend{
			Content: fmt.Sprintf("Para poder receber XP por participar nos eventos no servidor de Discord do UnderLX, tem de se registar no PosPlay primeiro: " + config.PathPrefix),
		})
		return false
	}

	typeFilter := sq.Eq{"type": "DISCORD_REACTION_EVENT"}
	eventFilter := sq.Expr("extra::json ->> 'event_id' = ?", messageID)

	transactions, err := player.XPTransactionsCustomFilter(tx, typeFilter, eventFilter)
	if err != nil {
		config.Log.Println(err)
		return false
	}
	if len(transactions) > 0 {
		// user already received rewards for this event
		return false
	}

	txid, err := uuid.NewV4()
	if err != nil {
		config.Log.Println(err)
		return false
	}

	xptx := &dataobjects.PPXPTransaction{
		ID:        txid.String(),
		DiscordID: player.DiscordID,
		Time:      time.Now(),
		Type:      "DISCORD_REACTION_EVENT",
		Value:     XPreward,
	}
	xptx.MarshalExtra(map[string]interface{}{
		"event_id": messageID,
	})

	err = xptx.Update(tx)
	if err != nil {
		config.Log.Println(err)
		return false
	}

	err = tx.Commit()
	if err != nil {
		config.Log.Println(err)
		return false
	}

	discordbot.SendDMtoUser(userID, &discordgo.MessageSend{
		Content: fmt.Sprintf("Acabou de receber %d XP pela participação num evento no servidor de Discord do UnderLX 👍", XPreward),
	})

	return true
}

func serialProcessor() {
	for {
		select {
		case id := <-tripSubmissionsChan:
			err := processTripForReward(id)
			if err != nil {
				config.Log.Println(err)
			}
		case id := <-tripEditsChan:
			err := processTripEditForReward(id)
			if err != nil {
				config.Log.Println(err)
			}
		}
	}
}

func uidConvS(uid string) uint64 {
	v, _ := strconv.ParseUint(uid, 10, 64)
	return v
}

func uidConvI(uid uint64) string {
	return strconv.FormatUint(uid, 10)
}

func getWeekStart() time.Time {
	loc, _ := time.LoadLocation(GameTimezone)
	now := time.Now().In(loc)
	daysSinceMonday := now.Weekday() - time.Monday
	if daysSinceMonday < 0 {
		// it's Sunday, last Monday was 6 days ago
		daysSinceMonday = 6
	}
	endTime := time.Date(now.Year(), now.Month(), now.Day()-int(daysSinceMonday), 2, 0, 0, 0, loc)
	if endTime.After(now) {
		// it's Monday, but it's not 2 AM yet
		endTime = endTime.AddDate(0, 0, -7)
	}
	return endTime
}

func descriptionForXPTransaction(tx *dataobjects.PPXPTransaction) string {
	extra := tx.UnmarshalExtra()
	switch tx.Type {
	case "SIGNUP_BONUS":
		return "Oferta de boas-vindas"
	case "PAIR_BONUS":
		return "Associação de dispositivo"
	case "TRIP_SUBMIT_REWARD":
		numstations, ok := extra["station_count"].(float64)
		numexchanges, ok2 := extra["interchange_count"].(float64)
		offpeak, ok3 := extra["offpeak"].(bool)
		if ok && ok2 && ok3 {
			excstr := ""
			switch int(numexchanges) {
			case 0:
				excstr = ""
			case 1:
				excstr = ", com 1 troca de linha"
			default:
				excstr = fmt.Sprintf(", com %d trocas de linha", int(numexchanges))
			}
			ofpstr := ""
			if offpeak {
				ofpstr = ", fora das horas de ponta"
			}
			return fmt.Sprintf("Viagem por %d estações%s%s", int(numstations), excstr, ofpstr)
		}
		return "Viagem"
	case "TRIP_CONFIRM_REWARD":
		return "Verificação de registo de viagem"
	case "DISCORD_REACTION_EVENT":
		return "Participação em evento no Discord do UnderLX"
	default:
		// ideally this should never show
		return "Bónus genérico"
	}
}

func getDisplayNameFromNameType(nameType string, user *discordgo.User, guildMember *discordgo.Member) string {
	switch nameType {
	case NicknameNameType:
		if guildMember != nil && guildMember.Nick != "" {
			return guildMember.Nick
		}
		fallthrough
	case UsernameNameType:
		return user.Username
	case UsernameDiscriminatorNameType:
		fallthrough
	default:
		return user.Username + "#" + user.Discriminator
	}
}
