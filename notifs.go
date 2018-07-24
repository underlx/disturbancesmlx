package main

import (
	fcm "github.com/NaySoftware/go-fcm"
	"github.com/underlx/disturbancesmlx/dataobjects"
)

var enableStatusNotifs = true
var enableAnnouncementNotifs = true

func init() {
	go func(newNotifChan <-chan dataobjects.StatusNotification) {
		for sn := range newNotifChan {
			SendNotificationForDisturbance(sn.Disturbance, sn.Status)
		}
	}(dataobjects.NewStatusNotification)
}

// SendNotificationForDisturbance sends a FCM notification for this disturbance about the specified status
func SendNotificationForDisturbance(d *dataobjects.Disturbance, s *dataobjects.Status) {
	if !enableStatusNotifs {
		return
	}
	downtimeStr := "false"
	if s.IsDowntime {
		downtimeStr = "true"
	}
	officialStr := "false"
	if s.Source.Official {
		officialStr = "true"
	}
	data := map[string]string{
		"network":     d.Line.Network.ID,
		"line":        d.Line.ID,
		"disturbance": d.ID,
		"status":      s.Status,
		"downtime":    downtimeStr,
		"official":    officialStr,
		"msgType":     string(s.MsgType),
	}

	if fcmcl == nil {
		// too soon
		return
	}

	mainLog.Println("Sending notification for disturbance " + d.ID + ": " + s.Status)

	if DEBUG {
		fcmcl.NewFcmMsgTo("/topics/disturbances-debug", data)
	} else {
		fcmcl.NewFcmMsgTo("/topics/disturbances", data)
	}

	fcmcl.SetPriority(fcm.Priority_HIGH)
	_, err := fcmcl.Send()
	if err != nil {
		mainLog.Println(err)
	}
}

// SendNotificationForAnnouncement sends a FCM notification for the specified announcement
func SendNotificationForAnnouncement(a *dataobjects.Announcement) {
	if !enableAnnouncementNotifs {
		return
	}
	data := map[string]string{
		"network": a.Network.ID,
		"title":   a.Title,
		"body":    a.Body,
		"url":     a.URL,
		"source":  a.Source,
	}

	if fcmcl == nil {
		// too soon
		return
	}

	mainLog.Println("Sending notification for announcement: " + a.Title)

	if DEBUG {
		fcmcl.NewFcmMsgTo("/topics/announcements-debug-"+a.Source, data)
	} else {
		fcmcl.NewFcmMsgTo("/topics/announcements-"+a.Source, data)
	}
	fcmcl.SetPriority(fcm.Priority_HIGH)
	_, err := fcmcl.Send()
	if err != nil {
		mainLog.Println(err)
	}
}

// SendMetaBroadcast sends a FCM message containing actions to execute on all clients
func SendMetaBroadcast(id, broadcastType, forVersions, forLocales string, args ...[2]string) {
	if fcmcl == nil {
		// too soon
		return
	}

	data := map[string]string{
		"id":   id,
		"type": broadcastType,
	}

	if forVersions != "" {
		data["versions"] = forVersions
	}

	if forLocales != "" {
		data["locales"] = forLocales
	}

	for _, arg := range args {
		data[arg[0]] = arg[1]
	}

	mainLog.Println("Sending meta-broadcast " + id)

	if DEBUG {
		fcmcl.NewFcmMsgTo("/topics/broadcasts-debug", data)
	} else {
		fcmcl.NewFcmMsgTo("/topics/broadcasts", data)
	}
	fcmcl.SetPriority(fcm.Priority_HIGH)
	_, err := fcmcl.Send()
	if err != nil {
		mainLog.Println(err)
	}
}

func handleControlNotifs(notiftype string, enable bool) {
	switch notiftype {
	case "status":
		enableStatusNotifs = enable
	case "announcements":
		enableAnnouncementNotifs = enable
	}
}
