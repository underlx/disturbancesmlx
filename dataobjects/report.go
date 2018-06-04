package dataobjects

// Report is a user report
type Report interface {
	Submitter() *APIPair // might be nil
	RateLimiterKey() string
	ReplayProtected() bool // whether it is hard for the submitter to bypass the replay protections
}

// BaseReport is the base for user report structs
type BaseReport struct {
	submitter              *APIPair // might be nil
	submitterKey           string   // uniquely identifies the submitter (might be API key, IP address...)
	strongReplayProtection bool
}

// LineDisturbanceReport is a Report of a disturbance in a line
type LineDisturbanceReport struct {
	BaseReport
	category string
	line     *Line
}

// NewLineDisturbanceReportThroughAPI creates a new LineDisturbanceReport
func NewLineDisturbanceReportThroughAPI(pair *APIPair, line *Line, category string) *LineDisturbanceReport {
	return &LineDisturbanceReport{
		BaseReport: BaseReport{
			submitter:              pair,
			submitterKey:           pair.Key,
			strongReplayProtection: true,
		},
		category: category,
		line:     line,
	}
}

// NewLineDisturbanceReport creates a new LineDisturbanceReport
func NewLineDisturbanceReport(ipAddr string, line *Line, category string) *LineDisturbanceReport {
	return &LineDisturbanceReport{
		BaseReport: BaseReport{
			submitterKey:           ipAddr,
			strongReplayProtection: false,
		},
		category: category,
		line:     line,
	}
}

// Submitter returns the APIPair that submitted this report, if any
// Might be nil if the report was not submitted by an APIPair
func (r *LineDisturbanceReport) Submitter() *APIPair {
	return r.submitter
}

// RateLimiterKey returns a string that can be used to identify this report in a rate limiting/duplicate detection system
func (r *LineDisturbanceReport) RateLimiterKey() string {
	return r.line.ID + "#" + r.submitterKey
}

// ReplayProtected returns whether it is hard for the submitter to bypass the replay protections
func (r *LineDisturbanceReport) ReplayProtected() bool {
	return r.strongReplayProtection
}

// Category returns the category of this report
func (r *LineDisturbanceReport) Category() string {
	return r.category
}

// Line returns the line of this report
func (r *LineDisturbanceReport) Line() *Line {
	return r.line
}
