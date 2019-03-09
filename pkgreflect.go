// Code generated by github.com/ungerik/pkgreflect DO NOT EDIT.

package main

import "reflect"

var Types = map[string]reflect.Type{
	"AnnouncementStore":   reflect.TypeOf((*AnnouncementStore)(nil)).Elem(),
	"BotCommandReceiver":  reflect.TypeOf((*BotCommandReceiver)(nil)).Elem(),
	"DelayMiddleware":     reflect.TypeOf((*DelayMiddleware)(nil)).Elem(),
	"Static":              reflect.TypeOf((*Static)(nil)).Elem(),
	"TelemetryMiddleware": reflect.TypeOf((*TelemetryMiddleware)(nil)).Elem(),
}

var Functions = map[string]reflect.Value{
	"APIserver":                       reflect.ValueOf(APIserver),
	"DiscordBot":                      reflect.ValueOf(DiscordBot),
	"SendMetaBroadcast":               reflect.ValueOf(SendMetaBroadcast),
	"SendNotificationForAnnouncement": reflect.ValueOf(SendNotificationForAnnouncement),
	"SendNotificationForDisturbance":  reflect.ValueOf(SendNotificationForDisturbance),
	"SetUpAnnouncements":              reflect.ValueOf(SetUpAnnouncements),
	"SetUpScrapers":                   reflect.ValueOf(SetUpScrapers),
	"StatsSender":                     reflect.ValueOf(StatsSender),
	"TearDownAnnouncements":           reflect.ValueOf(TearDownAnnouncements),
	"TearDownScrapers":                reflect.ValueOf(TearDownScrapers),
	"WebServer":                       reflect.ValueOf(WebServer),
}

var Variables = map[string]reflect.Value{
	"APIrequestTelemetry": reflect.ValueOf(&APIrequestTelemetry),
	"BuildDate":           reflect.ValueOf(&BuildDate),
	"GitCommit":           reflect.ValueOf(&GitCommit),
}

var Consts = map[string]reflect.Value{
	"DEBUG":                   reflect.ValueOf(DEBUG),
	"DefaultClientCertPath":   reflect.ValueOf(DefaultClientCertPath),
	"MLnetworkID":             reflect.ValueOf(MLnetworkID),
	"MaxDBconnectionPoolSize": reflect.ValueOf(MaxDBconnectionPoolSize),
	"SecretsPath":             reflect.ValueOf(SecretsPath),
}
