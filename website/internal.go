package website

import (
	"fmt"
	"net/http"
	"time"

	"github.com/underlx/disturbancesmlx/compute"
	"github.com/underlx/disturbancesmlx/dataobjects"
	"github.com/underlx/disturbancesmlx/utils"
)

// InternalPage serves a internal page
func InternalPage(w http.ResponseWriter, r *http.Request) {
	if !utils.RequestIsTLS(r) && !DEBUG {
		w.WriteHeader(http.StatusUpgradeRequired)
		return
	}

	hasSession, session, err := AuthGetSession(w, r, true)
	if err != nil {
		webLog.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	} else if !hasSession {
		return
	} else if !session.IsAdmin {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	tx, err := rootSqalxNode.Beginx()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		webLog.Println(err)
		return
	}
	defer tx.Commit()

	message := ""
	if r.Method == http.MethodPost && r.ParseForm() == nil {
		switch r.Form.Get("action") {
		case "reloadTemplates":
			vehicleHandler.ClearTypicalSecondsCache()
			ReloadTemplates()
			message = "Templates reloaded"
		case "computeMsgTypes":
			compute.UpdateStatusMsgTypes(tx)
			message = "Line status types recomputed"
		}
	}

	n, err := dataobjects.GetNetwork(tx, MLnetworkID)
	if err != nil {
		webLog.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	p := struct {
		PageCommons
		StartTime  time.Time
		EndTime    time.Time
		LinesExtra []struct {
			TotalTime    string
			TotalHours   float32
			Availability string
			AvgDuration  string
		}
		AverageSpeed         float64
		Message              string
		UserID               string
		Username             string
		PassengerReadings    []compute.PassengerReading
		TrainETAs            []compute.TrainETA
		UsersOnlineInNetwork int
	}{
		Message:              message,
		UserID:               session.UserID,
		Username:             session.DisplayName,
		PassengerReadings:    vehicleHandler.Readings(),
		UsersOnlineInNetwork: statsHandler.OITInNetwork(n, 0),
		TrainETAs:            []compute.TrainETA{},
	}

	p.PageCommons, err = InitPageCommons(tx, w, r, "Página interna")
	if err != nil {
		webLog.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	loc, _ := time.LoadLocation(n.Timezone)
	now := time.Now().In(loc)
	daysSinceMonday := now.Weekday() - time.Monday
	if daysSinceMonday < 0 {
		// it's Sunday, last Monday was 6 days ago
		daysSinceMonday = 6
	}
	p.EndTime = time.Date(now.Year(), now.Month(), now.Day()-int(daysSinceMonday), 2, 0, 0, 0, loc)
	if p.EndTime.After(now) {
		// it's Monday, but it's not 2 AM yet
		p.EndTime = p.EndTime.AddDate(0, 0, -7)
	}
	p.StartTime = p.EndTime.AddDate(0, 0, -7)

	lines, err := n.Lines(tx)
	if err != nil {
		webLog.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	p.LinesExtra = make([]struct {
		TotalTime    string
		TotalHours   float32
		Availability string
		AvgDuration  string
	}, len(lines))

	for i := range lines {
		availability, avgd, err := lines[i].Availability(tx, p.StartTime, p.EndTime, p.OfficialOnly)
		if err != nil {
			webLog.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		p.LinesExtra[i].Availability = fmt.Sprintf("%.03f%%", availability*100)
		p.LinesExtra[i].AvgDuration = fmt.Sprintf("%.01f", avgd.Minutes())
		totalDuration, _, err := lines[i].DisturbanceDuration(tx, p.StartTime, p.EndTime, p.OfficialOnly)
		if err != nil {
			webLog.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		p.LinesExtra[i].TotalTime = totalDuration.String()
		p.LinesExtra[i].TotalHours = float32(totalDuration.Hours())
	}

	p.AverageSpeed, err = compute.AverageSpeedCached(tx, p.StartTime, p.EndTime)
	if err == compute.ErrInfoNotReady {
		p.AverageSpeed = 0
	} else if err != nil {
		webLog.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// adjust time for display
	p.EndTime = p.EndTime.AddDate(0, 0, -1)

	// train ETA debugging
	p.TrainETAs, err = vehicleHandler.AllTrainETAs(tx)
	if err != nil {
		webLog.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
	}

	err = webtemplate.ExecuteTemplate(w, "internal.html", p)
	if err != nil {
		webLog.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
	}
}
