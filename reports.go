package main

import (
	"errors"
	"math"
	"time"

	"github.com/heetch/sqalx"
	cache "github.com/patrickmn/go-cache"
	"github.com/underlx/disturbancesmlx/dataobjects"
)

var reportHandler = NewReportHandler(statsHandler, rootSqalxNode)

// ReportHandler implements resource.ReportHandler
type ReportHandler struct {
	reports      *cache.Cache
	statsHandler *StatsHandler
	node         sqalx.Node
}

// NewReportHandler initializes a new ReportHandler and returns it
func NewReportHandler(statsHandler *StatsHandler, node sqalx.Node) *ReportHandler {
	h := &ReportHandler{
		reports:      cache.New(cache.NoExpiration, 30*time.Second),
		statsHandler: statsHandler,
	}
	h.reports.OnEvicted(func(string, interface{}) {
		h.evaluateSituation()
	})
	return h
}

type reportData struct {
	Report dataobjects.Report
	Weight int
}

// HandleLineDisturbanceReport handles line disturbance reports
func (r *ReportHandler) HandleLineDisturbanceReport(report *dataobjects.LineDisturbanceReport) error {
	if closed, err := report.Line().CurrentlyClosed(r.node); err == nil && closed {
		return errors.New("HandleLineDisturbanceReport: the line of this report is currently closed")
	}

	weight, err := r.getVoteWeightForReport(report)
	if err != nil {
		return err
	}

	data := &reportData{report, weight}

	err = r.reports.Add(report.RateLimiterKey(), data, 15*time.Minute)
	if err != nil {
		return errors.New("HandleLineDisturbanceReport: report rate-limited")
	}

	go r.evaluateSituation()
	return nil
}

func (r *ReportHandler) getVoteWeightForReport(report *dataobjects.LineDisturbanceReport) (int, error) {
	if !report.ReplayProtected() || report.Submitter() == nil {
		return 1, nil
	}

	// app user that is currently in the reported line
	if r.statsHandler.UserInLine(report.Line(), report.Submitter()) {
		return 30, nil
	}

	// app user that is currently in the reported network
	if r.statsHandler.UserInNetwork(report.Line().Network, report.Submitter()) {
		return 20, nil
	}

	// app user that submitted a trip in the last 20 minutes
	recentTrips, err := dataobjects.GetTripsForSubmitterBetween(r.node, report.Submitter(), time.Now().Add(-20*time.Minute), time.Now())
	if err != nil {
		return 0, err
	}
	if len(recentTrips) > 0 {
		return 10, nil
	}

	// app user that is not in the network/has location turned off
	return 5, nil
}

func (r *ReportHandler) countVotesForLine(line *dataobjects.Line) int {
	count := 0
	for _, item := range r.reports.Items() {
		data := item.Object.(*reportData)
		if ldr, ok := data.Report.(*dataobjects.LineDisturbanceReport); ok {
			if ldr.Line().ID == line.ID {
				count += data.Weight
			}
		}
	}
	return count
}

func (r *ReportHandler) clearVotesForLine(line *dataobjects.Line) {
	for key, item := range r.reports.Items() {
		data := item.Object.(*reportData)
		if ldr, ok := data.Report.(*dataobjects.LineDisturbanceReport); ok {
			if ldr.Line().ID == line.ID {
				r.reports.Delete(key)
			}
		}
	}
}

func (r *ReportHandler) getThresholdForLine(line *dataobjects.Line) int {
	numUsers := r.statsHandler.OITInLine(line, 0)
	if numUsers <= 1 {
		return 15
	}
	return int(math.Round(56.8206*math.Log(float64(numUsers)) - 18.9))
}

func (r *ReportHandler) lineHasEnoughVotesToStartDisturbance(line *dataobjects.Line) bool {
	return r.countVotesForLine(line) >= r.getThresholdForLine(line)
}

func (r *ReportHandler) lineHasEnoughVotesToKeepDisturbance(line *dataobjects.Line) bool {
	return r.countVotesForLine(line) >= r.getThresholdForLine(line)/2
}

func (r *ReportHandler) evaluateSituation() {
	tx, err := r.node.Beginx()
	if err != nil {
		mainLog.Println("ReportHandler: " + err.Error())
		return
	}
	defer tx.Rollback()

	lines, err := dataobjects.GetLines(tx)
	if err != nil {
		mainLog.Println("ReportHandler: " + err.Error())
		return
	}

	for _, line := range lines {
		disturbances, err := line.OngoingDisturbances(tx, false)
		if err != nil {
			mainLog.Println("ReportHandler: " + err.Error())
			return
		}

		if len(disturbances) == 0 && r.lineHasEnoughVotesToStartDisturbance(line) {
			// TODO start new unofficial disturbance
			// idea: even though we are only creating the disturbance now, the start time might be the time of the earliest report in memory
			// we would then add two line states: one for the date of the earliest report ("users began reporting...")
			// and another for the current time ("reports have been confirmed by multiple users...")
		} else {
			for _, disturbance := range disturbances {
				if disturbance.Official {
					// this avoids a new disturbance reopening immediately after it officially ends
					// do we really want this? TODO
					r.clearVotesForLine(line)
				} else if !r.lineHasEnoughVotesToKeepDisturbance(line) {
					// TODO end this unofficial disturbance
				}
			}
		}
	}

	tx.Commit()
}
