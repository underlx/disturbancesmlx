package posplay

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/bwmarrin/discordgo"
	"github.com/underlx/disturbancesmlx/dataobjects"
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

	player, err := dataobjects.GetPPPlayer(tx, uidConvS(session.DiscordInfo.ID))
	if err != nil {
		config.Log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	p := struct {
		pageCommons

		XPTransactions []*dataobjects.PPXPTransaction
		PairedDevice   bool
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

	_, err = dataobjects.GetPPPair(tx, player.DiscordID)
	p.PairedDevice = err == nil

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

	player, err := dataobjects.GetPPPlayer(tx, uidConvS(session.DiscordInfo.ID))
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
		_, err := dataobjects.GetPPPair(tx, player.DiscordID)
		p.HasPair = err == nil

		for _, notifType := range p.NotifTypes {
			p.NotifSettings[notifType] = make(map[string]bool)
			for _, notifMethod := range p.NotifMethods {
				p.NotifSettings[notifType][notifMethod], err = dataobjects.GetPPNotificationSetting(tx, player.DiscordID, notifType, notifMethod, NotificationDefaults)
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
					err = dataobjects.SetPPNotificationSetting(tx, player.DiscordID, notifType, notifMethod,
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

	player, err := dataobjects.GetPPPlayer(tx, uidConvS(session.DiscordInfo.ID))
	if err != nil {
		config.Log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	p := struct {
		pageCommons

		XPTransactions []*dataobjects.PPXPTransaction
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
