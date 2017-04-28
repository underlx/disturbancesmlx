package main

import (
	fcm "github.com/NaySoftware/go-fcm"
	"github.com/gbl08ma/disturbancesmlx/dataobjects"
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

	fcmcl.NewFcmMsgTo("/topics/disturbances", data)
	fcmcl.SetPriority(fcm.Priority_HIGH)
	_, err := fcmcl.Send()
	if err != nil {
		mainLog.Println(err)
	}
}
