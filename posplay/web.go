package posplay

import (
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"strings"
	"time"

	"github.com/underlx/disturbancesmlx/dataobjects"

	"github.com/gbl08ma/sqalx"
	"github.com/gorilla/csrf"
	"github.com/gorilla/mux"
	uuid "github.com/satori/go.uuid"
	"github.com/underlx/disturbancesmlx/utils"
)

// pageCommons contains information that is required by most page templates
type pageCommons struct {
	PageTitle   string
	DebugBuild  bool
	Session     *Session
	Player      *dataobjects.PPPlayer
	CSRFfield   template.HTML
	VersionInfo string

	// header / sidebar
	UserInfoInHeader bool
	SidebarSelected  string
	AvatarURL        string
	XP               int
	Level            int
	LevelProgression float64
	XPthisWeek       int
	RankThisWeek     int
}

// ConfigureRouter configures a router to handle PosPlay paths
func ConfigureRouter(router *mux.Router) {
	router.HandleFunc("/", homePage)
	router.HandleFunc("/pair", pairPage)
	router.HandleFunc("/pair/status", pairStatus)
	router.HandleFunc("/settings", settingsPage)
	router.HandleFunc("/xptx", xpTransactionHistoryPage)
	router.HandleFunc("/achievements", achievementsPage)
	router.HandleFunc("/achievements/{id:[0-9A-Fa-f]{8}-[0-9A-Fa-f]{4}-4[0-9A-Fa-f]{3}-[89ABab][0-9A-Fa-f]{3}-[0-9A-Fa-f]{12}}", achievementPage)
	router.HandleFunc("/leaderboards", leaderboardsPage)
	router.HandleFunc("/leaderboards/weekly", leaderboardsPage)
	router.HandleFunc("/leaderboards/alltime", leaderboardsAllTimePage)
	router.HandleFunc("/users/{id:[0-9]{7,20}}", profilePage) // in theory, discord/twitter snowflakes can be between 7 and 20 digits in length
	router.HandleFunc("/users/{id:[0-9]{7,20}}/achievements", profileAchievementsPage)
	router.HandleFunc("/login", forceLogin)
	router.HandleFunc("/logout", forceLogout)
	router.HandleFunc("/oauth/callback", callbackHandler)
	router.HandleFunc("/welcome", onboardingPage)
	router.HandleFunc("/privacy", privacyPolicyPage)
	router.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("static/"))))
	if DEBUG {
		router.Use(templateReloadingMiddleware)
	}
	router.Use(csrfMiddleware)
}

// ReloadTemplates reloads the templates for the website
func ReloadTemplates() {
	funcMap := template.FuncMap{
		"minus": func(a, b int) int {
			return a - b
		},
		"plus": func(a, b int) int {
			return a + b
		},
		"minus64": func(a, b int64) int64 {
			return a - b
		},
		"plus64": func(a, b int64) int64 {
			return a + b
		},
		"stringContains": func(s, substr string) bool {
			return strings.Contains(s, substr)
		},
		"formatTime": func(t time.Time) string {
			loc, _ := time.LoadLocation(GameTimezone)
			r := t.In(loc).Format("02 Jan 2006 15:04")
			switch r[3:6] {
			case "Feb":
				r = r[:3] + "Fev" + r[6:]
			case "Apr":
				r = r[:3] + "Abr" + r[6:]
			case "May":
				r = r[:3] + "Mai" + r[6:]
			case "Aug":
				r = r[:3] + "Ago" + r[6:]
			case "Sep":
				r = r[:3] + "Set" + r[6:]
			case "Oct":
				r = r[:3] + "Out" + r[6:]
			case "Dec":
				r = r[:3] + "Dez" + r[6:]
			}
			return r
		},
		"formatDate": func(t time.Time) string {
			loc, _ := time.LoadLocation(GameTimezone)
			r := t.In(loc).Format("02 Jan 2006")
			switch r[3:6] {
			case "Feb":
				r = r[:3] + "Fev" + r[6:]
			case "Apr":
				r = r[:3] + "Abr" + r[6:]
			case "May":
				r = r[:3] + "Mai" + r[6:]
			case "Aug":
				r = r[:3] + "Ago" + r[6:]
			case "Sep":
				r = r[:3] + "Set" + r[6:]
			case "Oct":
				r = r[:3] + "Out" + r[6:]
			case "Dec":
				r = r[:3] + "Dez" + r[6:]
			}
			return r
		},
		"uuid": func() string {
			id, err := uuid.NewV4()
			if err == nil {
				return id.String()
			}
			return ""
		},
		"xpTxDescription":            DescriptionForXPTransaction,
		"formatPortugueseMonth":      utils.FormatPortugueseMonth,
		"getDisplayNameFromNameType": getDisplayNameFromNameType,
		"formatLeaderboardWeek": func(start time.Time) string {
			end := start.AddDate(0, 0, 6)
			year, week := start.ISOWeek()
			return fmt.Sprintf("%dª semana de %d (%d %s - %d %s)",
				week, year,
				start.Day(), utils.FormatPortugueseMonthShort(start.Month()),
				end.Day(), utils.FormatPortugueseMonthShort(end.Month()))
		},
		"userAvatarURL":             userAvatarURL,
		"nameForNotificationType":   NameForNotificationType,
		"nameForNotificationMethod": NameForNotificationMethod,
		"dict": func(values ...interface{}) (map[string]interface{}, error) {
			if len(values)%2 != 0 {
				return nil, errors.New("invalid dict call")
			}
			dict := make(map[string]interface{}, len(values)/2)
			for i := 0; i < len(values); i += 2 {
				key, ok := values[i].(string)
				if !ok {
					return nil, errors.New("dict keys must be strings")
				}
				dict[key] = values[i+1]
			}
			return dict, nil
		},
	}

	webtemplate = template.Must(template.New("index.html").Funcs(funcMap).ParseGlob("templates/posplay/*.html"))
}

func templateReloadingMiddleware(next http.Handler) http.Handler {
	ReloadTemplates()
	err := ReloadAchievements()
	if err != nil {
		config.Log.Println(err)
	}
	return next
}

// initPageCommons fills PageCommons with the info that is required by most page templates
func initPageCommons(node sqalx.Node, w http.ResponseWriter, r *http.Request, title string, session *Session, player *dataobjects.PPPlayer) (commons pageCommons, err error) {
	commons.PageTitle = title + " | PosPlay"
	commons.DebugBuild = DEBUG
	commons.Session = session
	commons.Player = player
	commons.CSRFfield = csrf.TemplateField(r)
	commons.VersionInfo = PosPlayVersion + "-" + config.GitCommit

	if player != nil && node != nil {
		tx, err := node.Beginx()
		if err != nil {
			return commons, err
		}
		defer tx.Commit() // read-only tx

		commons.XP, commons.Level, commons.LevelProgression, err = player.Level(tx)
		if err != nil {
			return commons, err
		}

		commons.XPthisWeek, err = player.XPBalanceBetween(tx, WeekStart(), time.Now())
		if err != nil {
			return commons, err
		}

		commons.RankThisWeek, err = player.RankBetween(tx, WeekStart(), time.Now())
		if err != nil {
			return commons, err
		}

		commons.AvatarURL = session.DiscordInfo.AvatarURL("256")
	}

	return commons, nil
}
