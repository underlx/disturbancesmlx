package posplay

import (
	"html/template"
	"net/http"
	"strings"
	"time"

	"github.com/underlx/disturbancesmlx/dataobjects"

	"github.com/gorilla/mux"
	"github.com/heetch/sqalx"
	uuid "github.com/satori/go.uuid"
	"github.com/underlx/disturbancesmlx/utils"
)

// pageCommons contains information that is required by most page templates
type pageCommons struct {
	PageTitle  string
	DebugBuild bool
	Session    *Session
}

// ConfigureRouter configures a router to handle PosPlay paths
func ConfigureRouter(router *mux.Router) {
	router.HandleFunc("/", homePage)
	router.HandleFunc("/login", forceLogin)
	router.HandleFunc("/logout", forceLogout)
	router.HandleFunc("/oauth/callback", callbackHandler)
	if DEBUG {
		router.Use(templateReloadingMiddleware)
	}
}

// webReloadTemplate reloads the templates for the website
func webReloadTemplate() {
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
		"formatDisturbanceTime": func(t time.Time) string {
			loc, _ := time.LoadLocation("Europe/Lisbon")
			return t.In(loc).Format("02 Jan 2006 15:04")
		},
		"uuid": func() string {
			id, err := uuid.NewV4()
			if err == nil {
				return id.String()
			}
			return ""
		},
		"formatPortugueseMonth": utils.FormatPortugueseMonth,
	}

	webtemplate = template.Must(template.New("index.html").Funcs(funcMap).ParseGlob("web/posplay/*.html"))
}

func templateReloadingMiddleware(next http.Handler) http.Handler {
	webReloadTemplate()
	return next
}

// initPageCommons fills PageCommons with the info that is required by most page templates
func initPageCommons(node sqalx.Node, w http.ResponseWriter, r *http.Request, title string, session *Session) (commons pageCommons, err error) {
	commons.PageTitle = title + " | PosPlay"
	commons.DebugBuild = DEBUG
	commons.Session = session

	return commons, nil
}

func homePage(w http.ResponseWriter, r *http.Request) {
	session, _, err := GetSession(r, w, false)
	if err != nil {
		config.Log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if session != nil {
		dashboardPage(w, r, session)
		return
	}

	p := struct {
		pageCommons
	}{}
	p.pageCommons, err = initPageCommons(nil, w, r, "Página principal", session)
	if err != nil {
		config.Log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	err = webtemplate.ExecuteTemplate(w, "index.html", p)
	if err != nil {
		config.Log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
	}
}

func dashboardPage(w http.ResponseWriter, r *http.Request, session *Session) {
	tx, err := config.Node.Beginx()
	if err != nil {
		config.Log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer tx.Commit() // read-only tx

	player, err := dataobjects.GetPPPlayer(tx, uidConvS(session.DiscordInfo.ID))
	if err != nil {
		config.Log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	p := struct {
		pageCommons
		AvatarURL        string
		XP               int
		Level            int
		LevelProgression float64
		XPthisWeek       int
		JoinedServer     bool
	}{
		AvatarURL:    session.DiscordInfo.AvatarURL("256"),
		JoinedServer: player.InGuild,
	}
	p.pageCommons, err = initPageCommons(nil, w, r, "Página principal", session)
	if err != nil {
		config.Log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	p.XP, p.Level, p.LevelProgression, err = player.Level(tx)
	if err != nil {
		config.Log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	p.XPthisWeek, err = player.XPBalanceBetween(tx, getWeekStart(), time.Now())
	if err != nil {
		config.Log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	err = webtemplate.ExecuteTemplate(w, "dashboard.html", p)
	if err != nil {
		config.Log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
	}
}

func forceLogin(w http.ResponseWriter, r *http.Request) {
	_, redirected, err := GetSession(r, w, true)
	if err != nil {
		config.Log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
	}
	if !redirected {
		http.Redirect(w, r, config.PathPrefix, http.StatusTemporaryRedirect)
	}
}

func forceLogout(w http.ResponseWriter, r *http.Request) {
	// TODO security: make it so that logout requires a POST request and a CSRF token
	session, _, err := GetSession(r, w, false)
	if err != nil {
		config.Log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if session != nil {
		session.Logout(r, w)
	}
	http.Redirect(w, r, config.PathPrefix, http.StatusTemporaryRedirect)
}
