// Code generated by github.com/ungerik/pkgreflect DO NOT EDIT.

package types

import "reflect"

var Types = map[string]reflect.Type{
	"APIPair":               reflect.TypeOf((*APIPair)(nil)).Elem(),
	"AndroidPairRequest":    reflect.TypeOf((*AndroidPairRequest)(nil)).Elem(),
	"Announcement":          reflect.TypeOf((*Announcement)(nil)).Elem(),
	"AnnouncementStore":     reflect.TypeOf((*AnnouncementStore)(nil)).Elem(),
	"BaseReport":            reflect.TypeOf((*BaseReport)(nil)).Elem(),
	"Connection":            reflect.TypeOf((*Connection)(nil)).Elem(),
	"Dataset":               reflect.TypeOf((*Dataset)(nil)).Elem(),
	"Disturbance":           reflect.TypeOf((*Disturbance)(nil)).Elem(),
	"DisturbanceCategory":   reflect.TypeOf((*DisturbanceCategory)(nil)).Elem(),
	"Duration":              reflect.TypeOf((*Duration)(nil)).Elem(),
	"Exit":                  reflect.TypeOf((*Exit)(nil)).Elem(),
	"Feedback":              reflect.TypeOf((*Feedback)(nil)).Elem(),
	"FeedbackType":          reflect.TypeOf((*FeedbackType)(nil)).Elem(),
	"Line":                  reflect.TypeOf((*Line)(nil)).Elem(),
	"LineCondition":         reflect.TypeOf((*LineCondition)(nil)).Elem(),
	"LineDisturbanceReport": reflect.TypeOf((*LineDisturbanceReport)(nil)).Elem(),
	"LinePath":              reflect.TypeOf((*LinePath)(nil)).Elem(),
	"LineSchedule":          reflect.TypeOf((*LineSchedule)(nil)).Elem(),
	"Lobby":                 reflect.TypeOf((*Lobby)(nil)).Elem(),
	"LobbySchedule":         reflect.TypeOf((*LobbySchedule)(nil)).Elem(),
	"Network":               reflect.TypeOf((*Network)(nil)).Elem(),
	"NetworkSchedule":       reflect.TypeOf((*NetworkSchedule)(nil)).Elem(),
	"POI":                   reflect.TypeOf((*POI)(nil)).Elem(),
	"PPAchievement":         reflect.TypeOf((*PPAchievement)(nil)).Elem(),
	"PPAchievementContext":  reflect.TypeOf((*PPAchievementContext)(nil)).Elem(),
	"PPAchievementStrategy": reflect.TypeOf((*PPAchievementStrategy)(nil)).Elem(),
	"PPLeaderboardEntry":    reflect.TypeOf((*PPLeaderboardEntry)(nil)).Elem(),
	"PPNotificationSetting": reflect.TypeOf((*PPNotificationSetting)(nil)).Elem(),
	"PPPair":                reflect.TypeOf((*PPPair)(nil)).Elem(),
	"PPPlayer":              reflect.TypeOf((*PPPlayer)(nil)).Elem(),
	"PPPlayerAchievement":   reflect.TypeOf((*PPPlayerAchievement)(nil)).Elem(),
	"PPXPTransaction":       reflect.TypeOf((*PPXPTransaction)(nil)).Elem(),
	"PairConnection":        reflect.TypeOf((*PairConnection)(nil)).Elem(),
	"Point":                 reflect.TypeOf((*Point)(nil)).Elem(),
	"Report":                reflect.TypeOf((*Report)(nil)).Elem(),
	"Script":                reflect.TypeOf((*Script)(nil)).Elem(),
	"Source":                reflect.TypeOf((*Source)(nil)).Elem(),
	"Station":               reflect.TypeOf((*Station)(nil)).Elem(),
	"StationTags":           reflect.TypeOf((*StationTags)(nil)).Elem(),
	"StationUse":            reflect.TypeOf((*StationUse)(nil)).Elem(),
	"StationUseType":        reflect.TypeOf((*StationUseType)(nil)).Elem(),
	"Status":                reflect.TypeOf((*Status)(nil)).Elem(),
	"StatusMessageType":     reflect.TypeOf((*StatusMessageType)(nil)).Elem(),
	"StatusNotification":    reflect.TypeOf((*StatusNotification)(nil)).Elem(),
	"Time":                  reflect.TypeOf((*Time)(nil)).Elem(),
	"Transfer":              reflect.TypeOf((*Transfer)(nil)).Elem(),
	"Trip":                  reflect.TypeOf((*Trip)(nil)).Elem(),
	"WiFiAP":                reflect.TypeOf((*WiFiAP)(nil)).Elem(),
}

var Functions = map[string]reflect.Value{
	"ComputeAPISecretHash":               reflect.ValueOf(ComputeAPISecretHash),
	"CountPPPlayerAchievementsAchieved":  reflect.ValueOf(CountPPPlayerAchievementsAchieved),
	"CountPPPlayers":                     reflect.ValueOf(CountPPPlayers),
	"CountPPXPTransactionsWithType":      reflect.ValueOf(CountPPXPTransactionsWithType),
	"CountPairActivationsByDay":          reflect.ValueOf(CountPairActivationsByDay),
	"CountTripsByDay":                    reflect.ValueOf(CountTripsByDay),
	"GenerateAPIKey":                     reflect.ValueOf(GenerateAPIKey),
	"GenerateAPISecret":                  reflect.ValueOf(GenerateAPISecret),
	"GetAutorunScriptsWithType":          reflect.ValueOf(GetAutorunScriptsWithType),
	"GetConnection":                      reflect.ValueOf(GetConnection),
	"GetConnections":                     reflect.ValueOf(GetConnections),
	"GetDataset":                         reflect.ValueOf(GetDataset),
	"GetDatasets":                        reflect.ValueOf(GetDatasets),
	"GetDisturbance":                     reflect.ValueOf(GetDisturbance),
	"GetDisturbances":                    reflect.ValueOf(GetDisturbances),
	"GetDisturbancesBetween":             reflect.ValueOf(GetDisturbancesBetween),
	"GetExit":                            reflect.ValueOf(GetExit),
	"GetExits":                           reflect.ValueOf(GetExits),
	"GetFeedbacks":                       reflect.ValueOf(GetFeedbacks),
	"GetLatestNDisturbances":             reflect.ValueOf(GetLatestNDisturbances),
	"GetLine":                            reflect.ValueOf(GetLine),
	"GetLineCondition":                   reflect.ValueOf(GetLineCondition),
	"GetLineConditions":                  reflect.ValueOf(GetLineConditions),
	"GetLinePaths":                       reflect.ValueOf(GetLinePaths),
	"GetLineSchedules":                   reflect.ValueOf(GetLineSchedules),
	"GetLines":                           reflect.ValueOf(GetLines),
	"GetLobbies":                         reflect.ValueOf(GetLobbies),
	"GetLobbiesForStation":               reflect.ValueOf(GetLobbiesForStation),
	"GetLobby":                           reflect.ValueOf(GetLobby),
	"GetLobbySchedules":                  reflect.ValueOf(GetLobbySchedules),
	"GetNetwork":                         reflect.ValueOf(GetNetwork),
	"GetNetworkSchedules":                reflect.ValueOf(GetNetworkSchedules),
	"GetNetworks":                        reflect.ValueOf(GetNetworks),
	"GetOngoingDisturbances":             reflect.ValueOf(GetOngoingDisturbances),
	"GetPOI":                             reflect.ValueOf(GetPOI),
	"GetPOIs":                            reflect.ValueOf(GetPOIs),
	"GetPPAchievement":                   reflect.ValueOf(GetPPAchievement),
	"GetPPAchievements":                  reflect.ValueOf(GetPPAchievements),
	"GetPPNotificationSetting":           reflect.ValueOf(GetPPNotificationSetting),
	"GetPPPair":                          reflect.ValueOf(GetPPPair),
	"GetPPPairForKey":                    reflect.ValueOf(GetPPPairForKey),
	"GetPPPairs":                         reflect.ValueOf(GetPPPairs),
	"GetPPPlayer":                        reflect.ValueOf(GetPPPlayer),
	"GetPPPlayerAchievement":             reflect.ValueOf(GetPPPlayerAchievement),
	"GetPPPlayerAchievements":            reflect.ValueOf(GetPPPlayerAchievements),
	"GetPPPlayers":                       reflect.ValueOf(GetPPPlayers),
	"GetPPXPTransaction":                 reflect.ValueOf(GetPPXPTransaction),
	"GetPPXPTransactions":                reflect.ValueOf(GetPPXPTransactions),
	"GetPPXPTransactionsBetween":         reflect.ValueOf(GetPPXPTransactionsBetween),
	"GetPPXPTransactionsTotal":           reflect.ValueOf(GetPPXPTransactionsTotal),
	"GetPPXPTransactionsWithType":        reflect.ValueOf(GetPPXPTransactionsWithType),
	"GetPair":                            reflect.ValueOf(GetPair),
	"GetPairIfCorrect":                   reflect.ValueOf(GetPairIfCorrect),
	"GetScript":                          reflect.ValueOf(GetScript),
	"GetScripts":                         reflect.ValueOf(GetScripts),
	"GetScriptsWithType":                 reflect.ValueOf(GetScriptsWithType),
	"GetSource":                          reflect.ValueOf(GetSource),
	"GetSources":                         reflect.ValueOf(GetSources),
	"GetStation":                         reflect.ValueOf(GetStation),
	"GetStationTags":                     reflect.ValueOf(GetStationTags),
	"GetStationUses":                     reflect.ValueOf(GetStationUses),
	"GetStations":                        reflect.ValueOf(GetStations),
	"GetStatus":                          reflect.ValueOf(GetStatus),
	"GetStatuses":                        reflect.ValueOf(GetStatuses),
	"GetTransfer":                        reflect.ValueOf(GetTransfer),
	"GetTransfers":                       reflect.ValueOf(GetTransfers),
	"GetTrip":                            reflect.ValueOf(GetTrip),
	"GetTripIDs":                         reflect.ValueOf(GetTripIDs),
	"GetTripIDsBetween":                  reflect.ValueOf(GetTripIDsBetween),
	"GetTrips":                           reflect.ValueOf(GetTrips),
	"GetTripsForSubmitter":               reflect.ValueOf(GetTripsForSubmitter),
	"GetTripsForSubmitterBetween":        reflect.ValueOf(GetTripsForSubmitterBetween),
	"GetWiFiAP":                          reflect.ValueOf(GetWiFiAP),
	"GetWiFiAPs":                         reflect.ValueOf(GetWiFiAPs),
	"NewAndroidPairRequest":              reflect.ValueOf(NewAndroidPairRequest),
	"NewLineDisturbanceReport":           reflect.ValueOf(NewLineDisturbanceReport),
	"NewLineDisturbanceReportDebug":      reflect.ValueOf(NewLineDisturbanceReportDebug),
	"NewLineDisturbanceReportThroughAPI": reflect.ValueOf(NewLineDisturbanceReportThroughAPI),
	"NewPair":                            reflect.ValueOf(NewPair),
	"PPLeaderboardBetween":               reflect.ValueOf(PPLeaderboardBetween),
	"PosPlayLevelToXP":                   reflect.ValueOf(PosPlayLevelToXP),
	"PosPlayPlayerLevel":                 reflect.ValueOf(PosPlayPlayerLevel),
	"RegisterPPAchievementStrategy":      reflect.ValueOf(RegisterPPAchievementStrategy),
	"SetPPNotificationSetting":           reflect.ValueOf(SetPPNotificationSetting),
	"UnregisterPPAchievementStrategy":    reflect.ValueOf(UnregisterPPAchievementStrategy),
}

var Variables = map[string]reflect.Value{
	"ErrTimeParse":          reflect.ValueOf(&ErrTimeParse),
	"NewStatusNotification": reflect.ValueOf(&NewStatusNotification),
}

var Consts = map[string]reflect.Value{
	"CommunityReportedCategory": reflect.ValueOf(CommunityReportedCategory),
	"GoneThrough":               reflect.ValueOf(GoneThrough),
	"Interchange":               reflect.ValueOf(Interchange),
	"MLClosedMessage":           reflect.ValueOf(MLClosedMessage),
	"MLCompositeMessage":        reflect.ValueOf(MLCompositeMessage),
	"MLGenericMessage":          reflect.ValueOf(MLGenericMessage),
	"MLSolvedMessage":           reflect.ValueOf(MLSolvedMessage),
	"MLSpecialServiceMessage":   reflect.ValueOf(MLSpecialServiceMessage),
	"NetworkEntry":              reflect.ValueOf(NetworkEntry),
	"NetworkExit":               reflect.ValueOf(NetworkExit),
	"PassengerIncidentCategory": reflect.ValueOf(PassengerIncidentCategory),
	"PowerOutageCategory":       reflect.ValueOf(PowerOutageCategory),
	"RawMessage":                reflect.ValueOf(RawMessage),
	"ReportBeginMessage":        reflect.ValueOf(ReportBeginMessage),
	"ReportConfirmMessage":      reflect.ValueOf(ReportConfirmMessage),
	"ReportReconfirmMessage":    reflect.ValueOf(ReportReconfirmMessage),
	"ReportSolvedMessage":       reflect.ValueOf(ReportSolvedMessage),
	"S2LSincorrectDetection":    reflect.ValueOf(S2LSincorrectDetection),
	"SignalFailureCategory":     reflect.ValueOf(SignalFailureCategory),
	"StationAnomalyCategory":    reflect.ValueOf(StationAnomalyCategory),
	"ThirdPartyFaultCategory":   reflect.ValueOf(ThirdPartyFaultCategory),
	"TrainFailureCategory":      reflect.ValueOf(TrainFailureCategory),
	"Visit":                     reflect.ValueOf(Visit),
}
