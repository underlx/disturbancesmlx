package resource

import (
	"github.com/heetch/sqalx"
	"github.com/underlx/disturbancesmlx/dataobjects"
	"github.com/yarf-framework/yarf"
)

// ReportHandler handles user reports such as service disturbances
type ReportHandler interface {
	HandleLineDisturbanceReport(report *dataobjects.LineDisturbanceReport) error
}

// DisturbanceReport composites resource
type DisturbanceReport struct {
	resource
	reportHandler ReportHandler
}

type apiDisturbanceReport struct {
	LineID   string `msgpack:"line" json:"line"`
	Category string `msgpack:"category" json:"category"`
}

// WithNode associates a sqalx Node with this resource
func (r *DisturbanceReport) WithNode(node sqalx.Node) *DisturbanceReport {
	r.node = node
	return r
}

// WithHashKey associates a HMAC key with this resource so it can participate in authentication processes
func (r *DisturbanceReport) WithHashKey(key []byte) *DisturbanceReport {
	r.hashKey = key
	return r
}

// WithReportHandler associates a ReportsHandler with this resource
func (r *DisturbanceReport) WithReportHandler(handler ReportHandler) *DisturbanceReport {
	r.reportHandler = handler
	return r
}

// Post serves HTTP POST requests on this resource
func (r *DisturbanceReport) Post(c *yarf.Context) error {
	pair, err := r.AuthenticateClient(c)
	if err != nil {
		RenderUnauthorized(c)
		return nil
	}

	var request apiDisturbanceReport
	err = r.DecodeRequest(c, &request)
	if err != nil {
		return err
	}

	tx, err := r.Beginx()
	if err != nil {
		return err
	}
	defer tx.Commit() // read-only tx

	line, err := dataobjects.GetLine(tx, request.LineID)
	if err != nil {
		return err
	}

	// TODO validate categories once we use categories for anything

	report := dataobjects.NewLineDisturbanceReportThroughAPI(pair, line, request.Category)
	err = r.reportHandler.HandleLineDisturbanceReport(report)

	return err
}