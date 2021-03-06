// Code generated by github.com/ungerik/pkgreflect DO NOT EDIT.

package compute

import "reflect"

var Types = map[string]reflect.Type{
	"PassengerReading": reflect.TypeOf((*PassengerReading)(nil)).Elem(),
	"ReportHandler":    reflect.TypeOf((*ReportHandler)(nil)).Elem(),
	"StatsHandler":     reflect.TypeOf((*StatsHandler)(nil)).Elem(),
	"TrainETA":         reflect.TypeOf((*TrainETA)(nil)).Elem(),
	"TripsScatterplotNumTripsVsAvgSpeedPoint": reflect.TypeOf((*TripsScatterplotNumTripsVsAvgSpeedPoint)(nil)).Elem(),
	"TypicalSecondsEntry":                     reflect.TypeOf((*TypicalSecondsEntry)(nil)).Elem(),
	"TypicalSecondsMinMax":                    reflect.TypeOf((*TypicalSecondsMinMax)(nil)).Elem(),
	"VehicleETAHandler":                       reflect.TypeOf((*VehicleETAHandler)(nil)).Elem(),
	"VehicleHandler":                          reflect.TypeOf((*VehicleHandler)(nil)).Elem(),
}

var Functions = map[string]reflect.Value{
	"AverageSpeed":                       reflect.ValueOf(AverageSpeed),
	"AverageSpeedCached":                 reflect.ValueOf(AverageSpeedCached),
	"AverageSpeedFilter":                 reflect.ValueOf(AverageSpeedFilter),
	"Initialize":                         reflect.ValueOf(Initialize),
	"NewReportHandler":                   reflect.ValueOf(NewReportHandler),
	"NewStatsHandler":                    reflect.ValueOf(NewStatsHandler),
	"NewVehicleETAHandler":               reflect.ValueOf(NewVehicleETAHandler),
	"NewVehicleHandler":                  reflect.ValueOf(NewVehicleHandler),
	"SimulateRealtime":                   reflect.ValueOf(SimulateRealtime),
	"TripsScatterplotNumTripsVsAvgSpeed": reflect.ValueOf(TripsScatterplotNumTripsVsAvgSpeed),
	"TypicalSecondsByDowAndHour":         reflect.ValueOf(TypicalSecondsByDowAndHour),
	"UpdateStatusMsgTypes":               reflect.ValueOf(UpdateStatusMsgTypes),
	"UpdateTypicalSeconds":               reflect.ValueOf(UpdateTypicalSeconds),
}

var Variables = map[string]reflect.Value{
	"ErrInfoNotReady": reflect.ValueOf(&ErrInfoNotReady),
}

var Consts = map[string]reflect.Value{}
