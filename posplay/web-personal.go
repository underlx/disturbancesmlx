package posplay

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/underlx/disturbancesmlx/types"
	"github.com/underlx/disturbancesmlx/discordbot"
)

func dashboardPage(w http.ResponseWriter, r *http.Request, session *Session) {
	tx, err := config.Node.Beginx()
	if err != nil {
		config.Log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer tx.Commit() // read-only tx

	player, err := types.GetPPPlayer(tx, uidConvS(session.DiscordInfo.ID))
	if err != nil {
		config.Log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	type xpItem struct {
		Type  string
		Value int
	}
	p := struct {
		pageCommons

		XPTransactions     []*types.PPXPTransaction
		PairedDevice       bool
		XPBreakdownSeason  []xpItem
		XPBreakdownAllTime []xpItem
	}{}
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

	_, err = types.GetPPPair(tx, player.DiscordID)
	p.PairedDevice = err == nil

	types := [][]string{
		[]string{"TRIPS", "TRIP_SUBMIT_REWARD", "TRIP_CONFIRM_REWARD"},
		[]string{"DISCORD_EVENTS", "DISCORD_REACTION_EVENT", "DISCORD_CHALLENGE_EVENT"},
		[]string{"DISCORD_PARTICIPATION", "DISCORD_PARTICIPATION"},
		[]string{"ACHIEVEMENTS", "ACHIEVEMENT_REWARD"},
	}

	seasonStart := WeekStart()
	now := time.Now()
	typesTotal := 0
	typesSeasonTotal := 0
	for _, typeGroup := range types {
		typeTotal := 0
		typeSeasonTotal := 0

		for _, t := range typeGroup[1:] {
			s, err := player.XPBalanceWithType(tx, t)
			if err != nil {
				config.Log.Println(err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			typeTotal += s

			s, err = player.XPBalanceWithTypeBetween(tx, t, seasonStart, now)
			if err != nil {
				config.Log.Println(err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			typeSeasonTotal += s
		}

		p.XPBreakdownAllTime = append(p.XPBreakdownAllTime, xpItem{
			Type:  typeGroup[0],
			Value: typeTotal,
		})
		typesTotal += typeTotal
		p.XPBreakdownSeason = append(p.XPBreakdownSeason, xpItem{
			Type:  typeGroup[0],
			Value: typeSeasonTotal,
		})
		typesSeasonTotal += typeSeasonTotal
	}

	p.XPBreakdownAllTime = append(p.XPBreakdownAllTime, xpItem{
		Type:  "OTHER",
		Value: p.XP - typesTotal,
	})
	p.XPBreakdownSeason = append(p.XPBreakdownSeason, xpItem{
		Type:  "OTHER",
		Value: p.XPthisWeek - typesSeasonTotal,
	})

	p.Dependencies.Charts = true
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

	player, err := types.GetPPPlayer(tx, discordID)
	if err != nil {
		config.Log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	p := struct {
		pageCommons
		PairProcess *pairProcess
		CurrentPair *types.PPPair
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

	p.CurrentPair, _ = types.GetPPPair(tx, discordID)

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
	settingsLikePage(w, r, false)
}

func onboardingPage(w http.ResponseWriter, r *http.Request) {
	session, _, err := GetSession(r, w, false)
	if err != nil {
		config.Log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if session == nil || !session.GoToOnboarding {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	settingsLikePage(w, r, true)
}

func settingsLikePage(w http.ResponseWriter, r *http.Request, isOnboarding bool) {
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

	player, err := types.GetPPPlayer(tx, uidConvS(session.DiscordInfo.ID))
	if err != nil {
		config.Log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	p := struct {
		pageCommons
		GuildMember   *discordgo.Member
		SavedSettings bool

		NotifSettings map[string]map[string]bool
		NotifTypes    []string
		NotifMethods  []string
		HasPair       bool
	}{
		NotifTypes:    []string{NotificationTypeGuildEventWon, NotificationTypeAchievementAchieved},
		NotifMethods:  []string{NotificationMethodDiscordDM, NotificationMethodAppNotif},
		NotifSettings: make(map[string]map[string]bool),
	}
	if isOnboarding {
		p.pageCommons, err = initPageCommons(tx, w, r, "Boas vindas", session, player)
	} else {
		p.pageCommons, err = initPageCommons(tx, w, r, "Definições", session, player)
	}

	if err != nil {
		config.Log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	p.SidebarSelected = "settings"
	p.GuildMember, err = discordbot.ProjectGuildMember(session.DiscordInfo.ID)
	if err != nil {
		p.GuildMember = nil
		player.InGuild = false
	}

	if !isOnboarding {
		_, err := types.GetPPPair(tx, player.DiscordID)
		p.HasPair = err == nil

		for _, notifType := range p.NotifTypes {
			p.NotifSettings[notifType] = make(map[string]bool)
			for _, notifMethod := range p.NotifMethods {
				p.NotifSettings[notifType][notifMethod], err = types.GetPPNotificationSetting(tx, player.DiscordID, notifType, notifMethod, NotificationDefaults)
				if err != nil {
					config.Log.Println(err)
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
			}
		}
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

		switch r.Form.Get("profileprivacy-preference") {
		case "public":
			player.ProfilePrivacy = PublicProfilePrivacy
		case "players-only":
			player.ProfilePrivacy = PlayersOnlyProfilePrivacy
		case "private":
			player.ProfilePrivacy = PrivateProfilePrivacy
		}

		if !isOnboarding {
			for _, notifType := range p.NotifTypes {
				for _, notifMethod := range p.NotifMethods {
					p.NotifSettings[notifType][notifMethod] = r.Form.Get(fmt.Sprintf("notif-%s-%s", notifType, notifMethod)) != ""
					err = types.SetPPNotificationSetting(tx, player.DiscordID, notifType, notifMethod,
						p.NotifSettings[notifType][notifMethod])
					if err != nil {
						config.Log.Println(err)
						w.WriteHeader(http.StatusInternalServerError)
						return
					}
				}
			}
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

	err = tx.Commit()
	if err != nil {
		config.Log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
	}

	if isOnboarding && p.SavedSettings {
		http.Redirect(w, r, BaseURL()+"/", http.StatusTemporaryRedirect)
		return
	} else if isOnboarding {
		err = webtemplate.ExecuteTemplate(w, "onboarding.html", p)
	} else {
		err = webtemplate.ExecuteTemplate(w, "settings.html", p)
	}
	if err != nil {
		config.Log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
	}
}

func xpTransactionHistoryPage(w http.ResponseWriter, r *http.Request) {
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
	defer tx.Commit() // read-only tx

	player, err := types.GetPPPlayer(tx, uidConvS(session.DiscordInfo.ID))
	if err != nil {
		config.Log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	p := struct {
		pageCommons

		XPTransactions []*types.PPXPTransaction
	}{}
	p.pageCommons, err = initPageCommons(tx, w, r, "Histórico de recompensas", session, player)
	if err != nil {
		config.Log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	p.XPTransactions, err = player.XPTransactions(tx)
	if err != nil {
		config.Log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	err = webtemplate.ExecuteTemplate(w, "xptransactions.html", p)
	if err != nil {
		config.Log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
	}
}
