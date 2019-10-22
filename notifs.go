package main

import (
	"fmt"
	"strconv"

	fcm "github.com/NaySoftware/go-fcm"
	"github.com/underlx/disturbancesmlx/types"
)

var enableStatusNotifs = true
var enableAnnouncementNotifs = true

func init() {
	go func(newNotifChan <-chan types.StatusNotification) {
		for sn := range newNotifChan {
			SendNotificationForDisturbance(sn.Disturbance, sn.Status)
		}
	}(types.NewStatusNotification)
}

// SendNotificationForDisturbance sends a FCM notification for this disturbance about the specified status
func SendNotificationForDisturbance(d *types.Disturbance, s *types.Status) {
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

	fcmcl.SetCollapseKey(d.ID)
	fcmcl.SetPriority(fcm.Priority_HIGH)
	_, err := fcmcl.Send()
	if err != nil {
		mainLog.Println(err)
	}
}

// SendNotificationForAnnouncement sends a FCM notification for the specified announcement
func SendNotificationForAnnouncement(a *types.Announcement) {
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
func SendMetaBroadcast(shardID, shardMax int, id, broadcastType, forVersions, forLocales string, args ...[2]string) {
	if fcmcl == nil {
		// too soon
		return
	}

	data := map[string]string{
		"id":   id,
		"type": broadcastType,
	}

	if shardID > 0 && shardMax > 0 {
		data["shardID"] = strconv.Itoa(shardID)
		data["shardMax"] = strconv.Itoa(shardMax)
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

// SendPersonalNotification sends a FCM message to a specific user
func SendPersonalNotification(pair *types.APIPair, msgType string, data map[string]string) {
	mainLog.Println("Sending personal notification to pair " + pair.Key)

	data["type"] = msgType
	topicName := fmt.Sprintf("/topics/pair-%s", pair.Key)
	if DEBUG {
		fcmcl.NewFcmMsgTo(topicName+"-debug", data)
	} else {
		fcmcl.NewFcmMsgTo(topicName, data)
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

// SendNotificationForContest does nothing (stub method to be overriden on runtime by our awesome scripting system)
func SendNotificationForContest(a *types.Announcement) {

}
