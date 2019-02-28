package website

import (
	"net/http"
	"time"

	"github.com/gbl08ma/monkey"
	"github.com/underlx/disturbancesmlx/discordbot"

	"github.com/underlx/disturbancesmlx/dataobjects"
)

// MetaStatsPage serves a page with meta-statistics about the service
func MetaStatsPage(w http.ResponseWriter, r *http.Request) {
	tx, err := rootSqalxNode.Beginx()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		webLog.Println(err)
		return
	}
	defer tx.Commit()

	n, err := dataobjects.GetNetwork(tx, MLnetworkID)
	if err != nil {
		webLog.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	p := struct {
		PageCommons
		TripCountDates        []time.Time
		TripUnconfirmedCounts []int
		TripConfirmedCounts   []int
		ActivationCountDates  []time.Time
		ActivationCounts      []int
		OITusers              int
		AnkiddieInstalled     int
		AnkiddieEnvs          int
		AnkiddieRunning       int
		MonkeyPatched         int
		BotStats              *discordbot.Stats
		BotUptime             time.Duration
		BotMessageHandlers    []discordbot.MessageHandler
		BotReactionHandlers   []discordbot.ReactionHandler
	}{
		OITusers:            statsHandler.OITInNetwork(n, 5),
		BotStats:            discordbot.BotStats(),
		BotMessageHandlers:  discordbot.GetMessageHandlers(),
		BotReactionHandlers: discordbot.GetReactionHandlers(),
		MonkeyPatched:       monkey.PatchCount(),
	}
	p.BotUptime = time.Since(p.BotStats.StartTime)

	p.PageCommons, err = InitPageCommons(tx, w, r, "Estatísticas sobre o UnderLX")
	if err != nil {
		webLog.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	loc, _ := time.LoadLocation(n.Timezone)

	p.TripCountDates, p.TripUnconfirmedCounts, p.TripConfirmedCounts, err =
		dataobjects.CountTripsByDay(tx, time.Now().In(loc).AddDate(0, 0, -30), time.Now().In(loc))
	if err != nil {
		webLog.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	p.ActivationCountDates, p.ActivationCounts, err =
		dataobjects.CountPairActivationsByDay(tx, time.Now().In(loc).AddDate(0, 0, -30), time.Now().In(loc))
	if err != nil {
		webLog.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	scripts, err := dataobjects.GetScriptsWithType(tx, "anko")
	if err != nil {
		webLog.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	p.AnkiddieInstalled = len(scripts)

	ankiddieEnvs := parentAnkiddie.Environments()
	p.AnkiddieEnvs = len(ankiddieEnvs)
	for _, env := range ankiddieEnvs {
		if env.Started() && !env.Suspended() {
			p.AnkiddieRunning++
		}
	}

	p.Dependencies.Charts = true
	err = webtemplate.ExecuteTemplate(w, "metastats.html", p)
	if err != nil {
		webLog.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
	}
}
