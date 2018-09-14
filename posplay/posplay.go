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
	"github.com/underlx/disturbancesmlx/dataobjects"

	"github.com/gbl08ma/sqalx"
	"github.com/gorilla/csrf"
	"github.com/gorilla/sessions"

	"github.com/gbl08ma/keybox"
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
	return endTime.AddDate(0, 0, -7)
}

func descriptionForXPTransaction(tx *dataobjects.PPXPTransaction) string {
	extra := tx.UnmarshalExtra()
	switch tx.Type {
	case "SIGNUP_BONUS":
		return "Oferta de boas-vindas"
	case "PAIR_BONUS":
		return "Associação de dispositivo"
	case "TRIP_SUBMIT_REWARD":
		numstations, ok := extra["station_count"].(int)
		numexchanges, ok2 := extra["interchange_count"].(int)
		offpeak, ok3 := extra["offpeak"].(bool)
		if ok && ok2 && ok3 {
			excstr := ""
			switch numexchanges {
			case 0:
				excstr = ""
			case 1:
				excstr = ", com 1 troca de linha"
			default:
				excstr = fmt.Sprintf(", com %d trocas de linha", numexchanges)
			}
			ofpstr := ""
			if offpeak {
				ofpstr = ", fora das horas de ponta"
			}
			return fmt.Sprintf("Viagem por %d estações%s%s", numstations, excstr, ofpstr)
		}
		return "Viagem"
	case "TRIP_CONFIRM_REWARD":
		return "Confirmação/correcção de registo de viagem"
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
