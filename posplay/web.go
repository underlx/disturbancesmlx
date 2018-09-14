package posplay

import (
	"encoding/json"
	"html/template"
	"net/http"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/underlx/disturbancesmlx/dataobjects"
	"github.com/underlx/disturbancesmlx/discordbot"

	"github.com/gbl08ma/sqalx"
	"github.com/gorilla/csrf"
	"github.com/gorilla/mux"
	uuid "github.com/satori/go.uuid"
	"github.com/underlx/disturbancesmlx/utils"
)

// pageCommons contains information that is required by most page templates
type pageCommons struct {
	PageTitle  string
	DebugBuild bool
	Session    *Session
	Player     *dataobjects.PPPlayer
	CSRFfield  template.HTML

	// sidebar
	SidebarSelected  string
	AvatarURL        string
	XP               int
	Level            int
	LevelProgression float64
	XPthisWeek       int
}

// ConfigureRouter configures a router to handle PosPlay paths
func ConfigureRouter(router *mux.Router) {
	router.HandleFunc("/", homePage)
	router.HandleFunc("/pair", pairPage)
	router.HandleFunc("/pair/status", pairStatus)
	router.HandleFunc("/settings", settingsPage)
	router.HandleFunc("/login", forceLogin)
	router.HandleFunc("/logout", forceLogout)
	router.HandleFunc("/oauth/callback", callbackHandler)
	if DEBUG {
		router.Use(templateReloadingMiddleware)
	}
	router.Use(csrfMiddleware)
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
		"formatTime": func(t time.Time) string {
			loc, _ := time.LoadLocation(GameTimezone)
			return t.In(loc).Format("02 Jan 2006 15:04")
		},
		"uuid": func() string {
			id, err := uuid.NewV4()
			if err == nil {
				return id.String()
			}
			return ""
		},
		"xpTxDescription":            descriptionForXPTransaction,
		"formatPortugueseMonth":      utils.FormatPortugueseMonth,
		"getDisplayNameFromNameType": getDisplayNameFromNameType,
	}

	webtemplate = template.Must(template.New("index.html").Funcs(funcMap).ParseGlob("web/posplay/*.html"))
}

func templateReloadingMiddleware(next http.Handler) http.Handler {
	webReloadTemplate()
	return next
}

// initPageCommons fills PageCommons with the info that is required by most page templates
func initPageCommons(node sqalx.Node, w http.ResponseWriter, r *http.Request, title string, session *Session, player *dataobjects.PPPlayer) (commons pageCommons, err error) {
	commons.PageTitle = title + " | PosPlay"
	commons.DebugBuild = DEBUG
	commons.Session = session
	commons.Player = player
	commons.CSRFfield = csrf.TemplateField(r)

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

		commons.XPthisWeek, err = player.XPBalanceBetween(tx, getWeekStart(), time.Now())
		if err != nil {
			return commons, err
		}

		commons.AvatarURL = session.DiscordInfo.AvatarURL("256")
	}

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
	p.pageCommons, err = initPageCommons(nil, w, r, "Página principal", session, nil)
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

		XPTransactions []*dataobjects.PPXPTransaction
		JoinedServer   bool
	}{
		JoinedServer: player.InGuild,
	}
	p.pageCommons, err = initPageCommons(tx, w, r, "Página principal", session, player)
	if err != nil {
		config.Log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	p.SidebarSelected = "home"

	p.XPTransactions, err = player.XPTransactionsLimit(tx, 10)
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

func pairPage(w http.ResponseWriter, r *http.Request) {
	session, redirected, err := GetSession(r, w, true)
	if err != nil {
		config.Log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if redirected {
		return
	}

	tx, err := config.Node.Beginx()
	if err != nil {
		config.Log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer tx.Commit() // read-only

	discordID := uidConvS(session.DiscordInfo.ID)

	player, err := dataobjects.GetPPPlayer(tx, discordID)
	if err != nil {
		config.Log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	p := struct {
		pageCommons
		PairProcess *pairProcess
		CurrentPair *dataobjects.PPPair
	}{
		PairProcess: TheConnectionHandler.getProcess(discordID),
	}
	p.pageCommons, err = initPageCommons(tx, w, r, "Associação com dispositivo", session, player)
	if err != nil {
		config.Log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	p.SidebarSelected = "pair"

	p.CurrentPair, _ = dataobjects.GetPPPair(tx, discordID)

	err = webtemplate.ExecuteTemplate(w, "pair.html", p)
	if err != nil {
		config.Log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
	}
}

func pairStatus(w http.ResponseWriter, r *http.Request) {
	session, _, err := GetSession(r, w, false)
	if err != nil {
		config.Log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if session == nil {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	pairProcess := TheConnectionHandler.getProcess(uidConvS(session.DiscordInfo.ID))

	data := make(map[string]interface{})
	data["completed"] = pairProcess.Completed
	data["expiry"] = pairProcess.Expires.Unix()
	data["code"] = pairProcess.Code

	b, err := json.Marshal(data)
	if err != nil {
		config.Log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Write(b)
}

func settingsPage(w http.ResponseWriter, r *http.Request) {
	session, redirected, err := GetSession(r, w, true)
	if err != nil {
		config.Log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if redirected {
		return
	}

	tx, err := config.Node.Beginx()
	if err != nil {
		config.Log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	player, err := dataobjects.GetPPPlayer(tx, uidConvS(session.DiscordInfo.ID))
	if err != nil {
		config.Log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	p := struct {
		pageCommons
		JoinedServer  bool
		GuildMember   *discordgo.Member
		SavedSettings bool
	}{
		JoinedServer: player.InGuild,
	}
	p.pageCommons, err = initPageCommons(tx, w, r, "Definições", session, player)
	if err != nil {
		config.Log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	p.SidebarSelected = "settings"
	p.GuildMember, err = discordbot.ProjectGuildMember(session.DiscordInfo.ID)
	if err != nil {
		p.GuildMember = nil
	}

	if r.Method == http.MethodPost {
		r.ParseForm()
		switch r.Form.Get("name-preference") {
		case "username-discriminator":
			player.NameType = UsernameDiscriminatorNameType
		case "username":
			player.NameType = UsernameNameType
		case "nickname":
			player.NameType = NicknameNameType
		}

		switch r.Form.Get("lbprivacy-preference") {
		case "public":
			player.LBPrivacy = PublicLBPrivacy
		case "private":
			player.LBPrivacy = PrivateLBPrivacy
		}

		player.CachedName = getDisplayNameFromNameType(player.NameType, session.DiscordInfo, p.GuildMember)

		err = refreshSession(r, w, session, p.GuildMember, player)
		if err != nil {
			config.Log.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		err = player.Update(tx)
		if err != nil {
			config.Log.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}

	p.SavedSettings = r.Method == http.MethodPost

	err = webtemplate.ExecuteTemplate(w, "settings.html", p)
	if err != nil {
		config.Log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
	}

	err = tx.Commit()
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
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
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
