package posplay

import (
	"net/http"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/gbl08ma/sqalx"
	uuid "github.com/satori/go.uuid"
	"github.com/underlx/disturbancesmlx/dataobjects"
	"github.com/underlx/disturbancesmlx/discordbot"
	"golang.org/x/oauth2"
)

// Session represents a user session in the PosPlay subsystem
type Session struct {
	DiscordToken *oauth2.Token
	DiscordInfo  *discordgo.User
	DisplayName  string
}

// NewSession initializes a new PosPlay session from a Discord OAuth2 token
func NewSession(node sqalx.Node, r *http.Request, w http.ResponseWriter, discordToken *oauth2.Token) (*Session, error) {
	tx, err := node.Beginx()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	ppsession := Session{
		DiscordToken: discordToken,
	}

	err = ppsession.fetchDiscordInfo()
	if err != nil {
		return nil, err
	}

	guildMember, projectGuildErr := discordbot.ProjectGuildMember(ppsession.DiscordInfo.ID)
	if projectGuildErr != nil {
		guildMember = nil
	}

	player, err := dataobjects.GetPPPlayer(tx, uidConvS(ppsession.DiscordInfo.ID))
	if err != nil {
		// new player
		player, err = addNewPlayer(tx, ppsession.DiscordInfo, projectGuildErr == nil)
		if err != nil {
			return nil, err
		}
	} else {
		player.InGuild = projectGuildErr == nil
		player.CachedName = getDisplayNameFromNameType(player.NameType, ppsession.DiscordInfo, guildMember)
	}
	err = player.Update(tx)
	if err != nil {
		return nil, err
	}

	ppsession.DisplayName = player.CachedName

	session, _ := config.Store.Get(r, SessionName)

	session.Options.MaxAge = int(time.Until(discordToken.Expiry).Seconds())
	session.Options.HttpOnly = true
	session.Options.Secure = !DEBUG
	session.Values["session"] = ppsession

	err = session.Save(r, w)
	if err != nil {
		return nil, err
	}

	tx.Commit()

	return &ppsession, nil
}

func refreshSession(r *http.Request, w http.ResponseWriter, ppsession *Session, guildMember *discordgo.Member, player *dataobjects.PPPlayer) error {
	err := ppsession.fetchDiscordInfo()
	if err != nil {
		return err
	}

	ppsession.DisplayName = getDisplayNameFromNameType(player.NameType, ppsession.DiscordInfo, guildMember)

	session, _ := config.Store.Get(r, SessionName)

	session.Options.MaxAge = int(time.Until(ppsession.DiscordToken.Expiry).Seconds())
	session.Options.HttpOnly = true
	session.Options.Secure = !DEBUG
	session.Values["session"] = ppsession

	return session.Save(r, w)
}

// GetSession retrieves the Session from the specified request, if one exists,
// and if not, optionally redirects the user to the authentication page
func GetSession(r *http.Request, w http.ResponseWriter, doLogin bool) (ppsession *Session, redirected bool, err error) {
	session, _ := config.Store.Get(r, SessionName)

	msession, ok := session.Values["session"].(Session)
	if !ok || session.IsNew || time.Now().After(msession.DiscordToken.Expiry) {
		if !doLogin {
			return nil, false, nil
		}

		err := oauthLogin(r, w)
		if err != nil {
			return nil, false, err
		}
		return nil, true, nil
	}

	return &msession, false, nil
}

func addNewPlayer(node sqalx.Node, discordUser *discordgo.User, inGuild bool) (*dataobjects.PPPlayer, error) {
	tx, err := node.Beginx()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	player := &dataobjects.PPPlayer{
		DiscordID:      uidConvS(discordUser.ID),
		Joined:         time.Now(),
		LBPrivacy:      PrivateLBPrivacy,
		ProfilePrivacy: PrivateProfilePrivacy,
		NameType:       UsernameDiscriminatorNameType,
		InGuild:        inGuild,
		CachedName:     getDisplayNameFromNameType(UsernameDiscriminatorNameType, discordUser, nil),
	}

	err = player.Update(tx)
	if err != nil {
		return nil, err
	}

	err = DoXPTransaction(tx, player, player.Joined, 50, "SIGNUP_BONUS", nil, false)
	if err != nil {
		return nil, err
	}

	return player, tx.Commit()
}

func oauthLogin(r *http.Request, w http.ResponseWriter) error {
	uuid, err := uuid.NewV4()
	if err != nil {
		return err
	}
	url := oauthConfig.AuthCodeURL(uuid.String())

	session, _ := config.Store.Get(r, SessionName)

	session.Values["oauthState"] = uuid.String()

	err = session.Save(r, w)
	if err != nil {
		return err
	}

	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
	return nil
}

// Logout forcefully terminates a session
func (session *Session) Logout(r *http.Request, w http.ResponseWriter) error {
	cookiesession, _ := config.Store.Get(r, SessionName)
	cookiesession.Options.MaxAge = -1
	return cookiesession.Save(r, w)
}

func (session *Session) fetchDiscordInfo() error {
	// The REST API part of discordgo can be used like this
	// (presumably dg.Open and many other things will not work since this is not
	// a bot token, it's merely a OAuth2 token with the 'identify' scope)
	dg, err := discordgo.New(session.DiscordToken.TokenType + " " + session.DiscordToken.AccessToken)
	if err != nil {
		return err
	}

	session.DiscordInfo, err = dg.User("@me")
	return err
}
