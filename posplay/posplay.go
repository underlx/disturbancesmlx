package posplay

import (
	"encoding/gob"
	"errors"
	"fmt"
	"html/template"
	"log"
	"strconv"
	"time"

	"github.com/bwmarrin/discordgo"
	cache "github.com/patrickmn/go-cache"
	uuid "github.com/satori/go.uuid"
	"github.com/underlx/disturbancesmlx/discordbot"
	"github.com/underlx/disturbancesmlx/types"

	"github.com/gbl08ma/sqalx"
	"github.com/gorilla/csrf"
	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"

	sq "github.com/Masterminds/squirrel"
	"github.com/gbl08ma/keybox"
	"golang.org/x/oauth2"
)

var oauthConfig *oauth2.Config
var webtemplate *template.Template
var websiteURL string

type lightXPTXinfo struct {
	id         string
	actualDiff int
}

var tripSubmissionsChan = make(chan string, 100)
var tripEditsChan = make(chan string, 100)
var reportsChan = make(chan types.Report, 100)
var xpTxChan = make(chan lightXPTXinfo, 100)

const (
	// PrivateLBPrivacy is used when users don't want to appear in leaderboards
	PrivateLBPrivacy string = "PRIVATE"
	// PublicLBPrivacy is used when users want to appear in leaderboards
	PublicLBPrivacy string = "PUBLIC"

	// PrivateProfilePrivacy is used when users don't want to show their profile to anyone
	PrivateProfilePrivacy string = "PRIVATE"
	// PlayersOnlyProfilePrivacy is used when users only want to show their profile to other PosPlay players
	PlayersOnlyProfilePrivacy string = "PLAYERS_ONLY"
	// PublicProfilePrivacy is used when users want to show their profile to everyone
	PublicProfilePrivacy string = "PUBLIC"
)

const (
	// UsernameDiscriminatorNameType is used when users want to appear like gbl08ma#3988
	UsernameDiscriminatorNameType string = "USERNAME_DISCRIM"
	// UsernameNameType is used when users want to appear like gbl08ma
	UsernameNameType string = "USERNAME"
	// NicknameNameType is used when users want to appear as their nickname in the project's guild
	NicknameNameType string = "NICKNAME"
)

const (
	// NotificationMethodDiscordDM corresponds to sending notifications through Discord DMs
	NotificationMethodDiscordDM string = "DISCORD_DM"

	// NotificationMethodAppNotif corresponds to sending notifications to the paired app
	NotificationMethodAppNotif string = "APP_NOTIF"
)

const (
	// NotificationTypeAchievementAchieved is used when the user achieved an achievement
	NotificationTypeAchievementAchieved string = "ACHIEVEMENT_ACHIEVED"

	// NotificationTypeGuildEventWon is used when the user won a Discord guild event
	NotificationTypeGuildEventWon string = "GUILD_EVENT_WON"
)

// NotificationDefaults contains defaults for notification settings
var NotificationDefaults = map[string]map[string]bool{
	NotificationTypeAchievementAchieved: map[string]bool{
		NotificationMethodDiscordDM: false,
		NotificationMethodAppNotif:  true,
	},
	NotificationTypeGuildEventWon: map[string]bool{
		NotificationMethodDiscordDM: true,
		NotificationMethodAppNotif:  false,
	},
}

// Config contains runtime PosPlay subsystem configuration
type Config struct {
	Keybox              *keybox.Keybox
	Log                 *log.Logger
	Store               *sessions.CookieStore
	Node                sqalx.Node
	GitCommit           string
	SendAppNotification func(pair *types.APIPair, msgType string, data map[string]string)
	guildID             string
	roleID              string
}

var config Config
var csrfMiddleware mux.MiddlewareFunc

// Initialize initializes the PosPlay subsystem
func Initialize(ppconfig Config) error {
	// register Session with gob so it can be saved in cookies
	gob.Register(Session{})

	config = ppconfig

	var present, present2 bool
	websiteURL, present = config.Keybox.Get("websiteURL")
	if !present {
		return errors.New("Website URL not present in posplay keybox")
	}

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

	config.guildID, present = config.Keybox.Get("guildId")
	config.roleID, present2 = config.Keybox.Get("roleId")
	if !present || !present2 {
		config.Log.Println("Guild ID or Role ID not present in posplay keybox, won't add users to role")
	}

	csrfOpts := []csrf.Option{csrf.FieldName(CSRFfieldName), csrf.CookieName(CSRFcookieName)}
	if DEBUG {
		csrfOpts = append(csrfOpts, csrf.Secure(false))
	}
	csrfMiddleware = csrf.Protect([]byte(csrfAuthKey), csrfOpts...)

	oauthConfig = &oauth2.Config{
		RedirectURL:  BaseURL() + "/oauth/callback",
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Scopes:       []string{"identify"},
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://discord.com/api/oauth2/authorize",
			TokenURL: "https://discord.com/api/oauth2/token",
		},
	}

	discordbot.ThePosPlayBridge.OnEventWinCallback = RegisterEventWinCallback
	discordbot.ThePosPlayBridge.OnDiscussionParticipationCallback = RegisterDiscussionParticipationCallback
	discordbot.ThePosPlayBridge.ReloadAchievementsCallback = ReloadAchievements
	discordbot.ThePosPlayBridge.ReloadTemplatesCallback = ReloadTemplates
	discordbot.ThePosPlayBridge.PlayerXPInfo = playerXPinfo

	err := ReloadTemplates()
	if err != nil {
		return err
	}
	err = ReloadAchievements()
	if err != nil {
		return err
	}

	go serialProcessor()

	return nil
}

// BaseURL returns the base URL of the website without trailing slash
func BaseURL() string {
	return websiteURL
}

// RegisterTripSubmission schedules a trip submission for analysis
func RegisterTripSubmission(trip *types.Trip) {
	tripSubmissionsChan <- trip.ID
}

// RegisterTripFirstEdit schedules a trip resubmission (edit, confirmation) for analysis
func RegisterTripFirstEdit(trip *types.Trip) {
	tripEditsChan <- trip.ID
}

// RegisterReport schedules a report for analysis
func RegisterReport(report types.Report) {
	reportsChan <- report
}

// RegisterEventWinCallback gives a user a XP reward for a Discord event, if he has not received a reward for that event yet
func RegisterEventWinCallback(userID, messageID string, XPreward int, eventType string) bool {
	// does the user even exist in PosPlay?
	tx, err := config.Node.Beginx()
	if err != nil {
		config.Log.Println(err)
		return false
	}
	defer tx.Rollback()

	player, err := types.GetPPPlayer(tx, uidConvS(userID))
	if err != nil {
		// this user is not yet a PosPlay player
		discordbot.SendDMtoUser(userID, &discordgo.MessageSend{
			Content: fmt.Sprintf("Para poder receber XP por participar nos eventos no servidor de Discord do UnderLX, tem de se registar no PosPlay primeiro: " + BaseURL()),
		})
		return false
	}

	typeFilter := sq.Eq{"type": eventType}
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

	err = DoXPTransaction(tx, player, time.Now(), XPreward, eventType, map[string]interface{}{
		"event_id": messageID,
	}, false)
	if err != nil {
		config.Log.Println(err)
		return false
	}

	sendDiscordNotif, err := types.GetPPNotificationSetting(tx, player.DiscordID, NotificationTypeGuildEventWon, NotificationMethodDiscordDM, NotificationDefaults)
	if err != nil {
		config.Log.Println(err)
		return false
	}

	sendAppNotif, err := types.GetPPNotificationSetting(tx, player.DiscordID, NotificationTypeGuildEventWon, NotificationMethodAppNotif, NotificationDefaults)
	if err != nil {
		config.Log.Println(err)
		return false
	}
	var appNotifPair *types.APIPair
	if sendAppNotif {
		pair, err := types.GetPPPair(tx, player.DiscordID)
		if err != nil {
			// app notification setting enabled, but the user doesn't have an associated app
			sendAppNotif = false
		} else {
			appNotifPair = pair.Pair
		}
	}

	tx.DeferToCommit(func() {
		if sendDiscordNotif {
			discordbot.SendDMtoUser(userID, &discordgo.MessageSend{
				Content: fmt.Sprintf("Acabou de receber %d XP pela participação num evento no servidor de Discord do UnderLX 👍", XPreward),
			})
		}
		if sendAppNotif {
			data := map[string]string{
				"title": fmt.Sprintf("Recebeu %d XP", XPreward),
				"body":  fmt.Sprintf("Recebeu %d XP pela participação num evento no servidor de Discord do UnderLX", XPreward),
				"url":   BaseURL() + "/xptx",
			}
			config.SendAppNotification(appNotifPair, "posplay-notification", data)
		}
	})

	err = tx.Commit()
	if err != nil {
		config.Log.Println(err)
		return false
	}
	return true
}

// RegisterDiscussionParticipationCallback gives a user a XP reward for participating in the Discord channels
func RegisterDiscussionParticipationCallback(userID string, XPreward int) bool {
	// does the user even exist in PosPlay?
	tx, err := config.Node.Beginx()
	if err != nil {
		config.Log.Println(err)
		return false
	}
	defer tx.Rollback()

	player, err := types.GetPPPlayer(tx, uidConvS(userID))
	if err != nil {
		// this user is not yet a PosPlay player
		return false
	}

	err = DoXPTransaction(tx, player, time.Now(), XPreward, "DISCORD_PARTICIPATION", nil, true)
	if err != nil {
		config.Log.Println(err)
		return false
	}

	err = tx.Commit()
	if err != nil {
		config.Log.Println(err)
		return false
	}
	return true
}

// RegisterXPTransaction is exclusively meant for use when bootstrapping the achievements system on a database with previous XP transactions
func RegisterXPTransaction(tx *types.PPXPTransaction) {
	xpTxChan <- lightXPTXinfo{
		id:         tx.ID,
		actualDiff: tx.Value,
	}
}

// DoXPTransaction adds a XP transaction to a user, performing the necessary checks and
// calling the necessary handlers
func DoXPTransaction(node sqalx.Node, player *types.PPPlayer, when time.Time, value int, txType string, extra map[string]interface{}, attemptMerge bool) error {
	tx, err := node.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	lasttx, err := player.XPTransactionsLimit(tx, 1)
	if err != nil {
		return err
	}
	var newtx *types.PPXPTransaction
	if attemptMerge && len(lasttx) > 0 && lasttx[0].Type == txType &&
		lasttx[0].Time.After(WeekStart()) {
		// to avoid creating many micro-transactions, update the latest transaction, adding the new reward
		// the lasttx[0].Time.After(getWeekStart()) check prevents tx merging with old rewards, "bringing old XP into the current week"
		newtx = lasttx[0]
	} else {
		txid, err := uuid.NewV4()
		if err != nil {
			return err
		}
		newtx = &types.PPXPTransaction{
			ID:        txid.String(),
			DiscordID: player.DiscordID,
			Type:      txType,
		}
	}
	newtx.Time = when
	newtx.Value += value
	if extra != nil {
		newtx.MarshalExtra(extra)
	}

	err = newtx.Update(tx)
	if err != nil {
		return err
	}

	tx.DeferToCommit(func() {
		xpTxChan <- lightXPTXinfo{
			id:         newtx.ID,
			actualDiff: value,
		}
	})

	return tx.Commit()
}

func serialProcessor() {
	for {
		select {
		case id := <-tripSubmissionsChan:
			err := processTripForReward(id)
			if err != nil {
				config.Log.Println(err)
			}
			err = processTripForAchievements(id)
			if err != nil {
				config.Log.Println(err)
			}
		case id := <-tripEditsChan:
			err := processTripEditForReward(id)
			if err != nil {
				config.Log.Println(err)
			}
			err = processTripEditForAchievements(id)
			if err != nil {
				config.Log.Println(err)
			}
		case report := <-reportsChan:
			err := processReportForAchievements(report)
			if err != nil {
				config.Log.Println(err)
			}
		case info := <-xpTxChan:
			err := processXPTxForAchievements(info.id, info.actualDiff)
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

// WeekStart returns when the current competition week started
func WeekStart() time.Time {
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

// MonthStart returns when the current competition month started
func MonthStart() time.Time {
	loc, _ := time.LoadLocation(GameTimezone)
	now := time.Now().In(loc)
	t := time.Date(now.Year(), now.Month(), 1, 2, 0, 0, 0, loc)
	if t.After(now) {
		// it's the first of the month, but it's not 2 AM yet
		t = t.AddDate(0, -1, 0)
	}
	return t
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

func playerXPinfo(userID string) (discordbot.PosPlayXPInfo, error) {
	tx, err := config.Node.Beginx()
	if err != nil {
		return discordbot.PosPlayXPInfo{}, err
	}
	defer tx.Commit() // read-only tx

	return playerXPinfoWithTx(tx, userID)
}

func playerXPinfoWithTx(tx sqalx.Node, userID string) (discordbot.PosPlayXPInfo, error) {
	player, err := types.GetPPPlayer(tx, uidConvS(userID))
	if err != nil {
		return discordbot.PosPlayXPInfo{}, err
	}

	username := player.CachedName
	avatar := userAvatarURL(uidConvS(userID), "256")
	if player.LBPrivacy == PrivateLBPrivacy {
		username = player.AnonymousName()
		avatar = fmt.Sprintf("https://api.adorable.io/avatars/256/%d.png", player.Seed())
	}
	xp, level, progress, err := player.Level(tx)
	if err != nil {
		return discordbot.PosPlayXPInfo{}, err
	}

	xpWeek, err := player.XPBalanceBetween(tx, WeekStart(), time.Now())
	if err != nil {
		xpWeek = 0
	}
	rank, err := player.RankBetween(tx, time.Time{}, time.Now())
	if err != nil {
		rank = 0
	}
	rankWeek, err := player.RankBetween(tx, WeekStart(), time.Now())
	if err != nil {
		rankWeek = 0
	}
	return discordbot.PosPlayXPInfo{
		Username:      username,
		ProfileURL:    BaseURL() + "/users/" + uidConvI(player.DiscordID),
		AvatarURL:     avatar,
		Level:         level,
		LevelProgress: progress,
		XP:            xp,
		XPthisWeek:    xpWeek,
		Rank:          rank,
		RankThisWeek:  rankWeek,
	}, nil
}

var avatarURLcache = cache.New(1*time.Hour, 10*time.Minute)

func userAvatarURL(userID uint64, size string) string {
	url, found := avatarURLcache.Get(uidConvI(userID) + "-" + size)
	if found {
		return url.(string)
	}
	url = ""
	user, err := discordbot.User(uidConvI(userID))
	if err == nil {
		url = user.AvatarURL(size)
	}
	avatarURLcache.SetDefault(uidConvI(userID)+"-"+size, url)
	return url.(string)
}
