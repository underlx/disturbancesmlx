package posplay

import (
	"encoding/gob"
	"errors"
	"html/template"
	"log"
	"strconv"
	"time"

	"github.com/gbl08ma/sqalx"
	"github.com/gorilla/sessions"

	"github.com/gbl08ma/keybox"
	"golang.org/x/oauth2"
)

var oauthConfig *oauth2.Config
var webtemplate *template.Template

const (
	// PrivateLBPrivacy is used when users don't want to appear in leaderboards
	PrivateLBPrivacy string = "PRIVATE"
	// PublicLBPrivacy is used when users want to appear in leaderboards
	PublicLBPrivacy string = "PUBLIC"
)

const (
	// UsernameDenominatorNameType is used when users want to appear like gbl08ma#3988
	UsernameDenominatorNameType string = "USERNAME_DENOMINATOR"
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

// Initialize initializes the PosPlay subsystem
func Initialize(ppconfig Config) error {
	// register Session with gob so it can be saved in cookies
	gob.Register(Session{})

	config = ppconfig
	clientID, present := config.Keybox.Get("oauthClientId")
	if !present {
		return errors.New("OAuth client ID not present in web keybox")
	}

	clientSecret, present := config.Keybox.Get("oauthClientSecret")
	if !present {
		return errors.New("OAuth client secret not present in web keybox")
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

	webReloadTemplate()

	return nil
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
