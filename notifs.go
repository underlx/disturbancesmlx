package main

import (
	fcm "github.com/NaySoftware/go-fcm"
	"github.com/underlx/disturbancesmlx/dataobjects"
)

// SendNotificationForDisturbance sends a FCM notification for this disturbance about the specified status
func SendNotificationForDisturbance(d *dataobjects.Disturbance, s *dataobjects.Status) {
	downtimeStr := "false"
	if s.IsDowntime {
		downtimeStr = "true"
	}
	data := map[string]string{
		"network":     d.Line.Network.ID,
		"line":        d.Line.ID,
		"disturbance": d.ID,
		"status":      s.Status,
		"downtime":    downtimeStr,
	}

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
	data := map[string]string{
		"network": a.Network.ID,
		"title":   a.Title,
		"body":    a.Body,
		"url":     a.URL,
		"source":  a.Source,
	}

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
