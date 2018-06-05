package main

import (
	"errors"
	"math"
	"time"

	"github.com/heetch/sqalx"
	cache "github.com/patrickmn/go-cache"
	uuid "github.com/satori/go.uuid"
	"github.com/underlx/disturbancesmlx/dataobjects"
)

var reportHandler *ReportHandler

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
		node:         node,
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

	return r.addReport(report, weight)
}

func (r *ReportHandler) addReport(report *dataobjects.LineDisturbanceReport, weight int) error {
	data := &reportData{report, weight}
	err := r.reports.Add(report.RateLimiterKey(), data, 15*time.Minute)
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

func (r *ReportHandler) getEarliestVoteForLine(line *dataobjects.Line) *reportData {
	earliestTime := time.Time{}
	var earliestValue *reportData
	for _, item := range r.reports.Items() {
		data := item.Object.(*reportData)
		if ldr, ok := data.Report.(*dataobjects.LineDisturbanceReport); ok {
			if earliestTime.IsZero() || ldr.Time().Before(earliestTime) {
				earliestTime = ldr.Time()
				earliestValue = data
			}
		}
	}
	return earliestValue
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
	defer tx.Commit() // read-only tx (any new statuses are handled in different transactions)

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
			err := r.startDisturbance(tx, line)
			if err != nil {
				mainLog.Println("ReportHandler: " + err.Error())
			}
			continue
		}

		// the system works in such a way that there can only be one ongoing disturbance per line at a time,
		// but we use a loop anyway
		for _, disturbance := range disturbances {
			if disturbance.Official {
				// this avoids a new disturbance reopening immediately after it officially ends
				r.clearVotesForLine(line)
			} else if !r.lineHasEnoughVotesToKeepDisturbance(line) {
				// end this unofficial disturbance
				err := r.endDisturbance(line)
				if err != nil {
					mainLog.Println("ReportHandler: " + err.Error())
				}
			}
		}
	}
}

func (r *ReportHandler) startDisturbance(node sqalx.Node, line *dataobjects.Line) error {
	earliestVote := r.getEarliestVoteForLine(line)
	if earliestVote == nil {
		return errors.New("earliest vote is nil")
	}

	latestDisturbance, err := line.LastDisturbance(node, false)
	if err != nil {
		return err
	}

	id, err := uuid.NewV4()
	if err != nil {
		return err
	}

	if earliestVote.Report.Time().After(latestDisturbance.UEndTime) {
		// even though we are only creating the disturbance now, the start time might be the time of the earliest report in memory
		// we would then add two line states: one for the date of the earliest report ("users began reporting...")
		// (we do not notify for this first state)
		// and another for the current time ("reports have been confirmed by multiple users...")

		if earliestVote.Report.Time().Before(time.Now().Add(-1 * time.Minute)) {
			// avoid issuing two states if their times would be too close to each other
			status := &dataobjects.Status{
				ID:         id.String(),
				Time:       earliestVote.Report.Time().UTC(),
				Line:       line,
				IsDowntime: true,
				Status:     "Os utilizadores comunicaram problemas na circulação",
				Source: &dataobjects.Source{
					ID:        "underlx-community",
					Name:      "UnderLX user community",
					Automatic: false,
					Official:  false,
				},
				MsgType: dataobjects.ReportBeginMessage,
			}

			handleNewStatus(status, false)

			id, err = uuid.NewV4()
			if err != nil {
				return err
			}
		}

		status := &dataobjects.Status{
			ID:         id.String(),
			Time:       time.Now().UTC(),
			Line:       line,
			IsDowntime: true,
			Status:     "Vários utilizadores confirmaram problemas na circulação",
			Source: &dataobjects.Source{
				ID:        "underlx-community",
				Name:      "UnderLX user community",
				Automatic: false,
				Official:  false,
			},
			MsgType: dataobjects.ReportConfirmMessage,
		}

		handleNewStatus(status, true)
	} else {
		// last disturbance ended after the earliest vote we have in memory
		// show a different message in this case

		status := &dataobjects.Status{
			ID:         id.String(),
			Time:       time.Now().UTC(),
			Line:       line,
			IsDowntime: true,
			Status:     "Vários utilizadores confirmaram mais problemas na circulação",
			Source: &dataobjects.Source{
				ID:        "underlx-community",
				Name:      "UnderLX user community",
				Automatic: false,
				Official:  false,
			},
			MsgType: dataobjects.ReportReconfirmMessage,
		}

		handleNewStatus(status, true)
	}
	return nil
}

func (r *ReportHandler) endDisturbance(line *dataobjects.Line) error {
	id, err := uuid.NewV4()
	if err != nil {
		return err
	}

	status := &dataobjects.Status{
		ID:         id.String(),
		Time:       time.Now().UTC(),
		Line:       line,
		IsDowntime: false,
		Status:     "Já não existem relatos de problemas na circulação",
		Source: &dataobjects.Source{
			ID:        "underlx-community",
			Name:      "UnderLX user community",
			Automatic: false,
			Official:  false,
		},
		MsgType: dataobjects.ReportSolvedMessage,
	}

	handleNewStatus(status, true)
	return nil
}
