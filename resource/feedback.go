package resource

import (
	"net/http"
	"time"

	"github.com/gbl08ma/sqalx"
	"github.com/underlx/disturbancesmlx/dataobjects"
	"github.com/yarf-framework/yarf"
)

// Feedback composites resource
type Feedback struct {
	resource
}

type apiFeedback struct {
	ID       string                   `msgpack:"id" json:"id"`
	Time     time.Time                `msgpack:"timestamp" json:"timestamp"`
	Type     dataobjects.FeedbackType `msgpack:"type" json:"type"`
	Contents string                   `msgpack:"contents" json:"contents"`
}

// WithNode associates a sqalx Node with this resource
func (r *Feedback) WithNode(node sqalx.Node) *Feedback {
	r.node = node
	return r
}

// WithHashKey associates a HMAC key with this resource so it can participate in authentication processes
func (r *Feedback) WithHashKey(key []byte) *Feedback {
	r.hashKey = key
	return r
}

// Post serves HTTP POST requests on this resource
func (r *Feedback) Post(c *yarf.Context) error {
	pair, err := r.AuthenticateClient(c)
	if err != nil {
		RenderUnauthorized(c)
		return nil
	}

	var request apiFeedback
	err = r.DecodeRequest(c, &request)
	if err != nil {
		return err
	}

	tx, err := r.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Feedback UUIDs are client-generated, so we can't really trust their (lack of) uniqueness...
	if _, err := dataobjects.GetFeedback(tx, request.ID); err == nil {
		return &yarf.CustomError{
			HTTPCode:  http.StatusBadRequest,
			ErrorMsg:  "A feedback with the specified ID already exists. Better luck generating a UUID next time.",
			ErrorBody: "A feedback with the specified ID already exists. Better luck generating a UUID next time.",
		}
	}

	feedback := dataobjects.Feedback{
		ID:        request.ID,
		Submitter: pair,
		Time:      request.Time,
		Type:      request.Type,
		Contents:  request.Contents,
	}

	err = feedback.Update(tx)
	if err != nil {
		return err
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	c.Response.WriteHeader(http.StatusCreated)
	r.render(c, &feedback)
	return nil
}

func (r *Feedback) render(c *yarf.Context, feedback *dataobjects.Feedback) {
	data := apiFeedback{
		ID:       feedback.ID,
		Time:     feedback.Time,
		Type:     feedback.Type,
		Contents: feedback.Contents,
	}

	RenderData(c, data, "no-cache, no-store, must-revalidate")
}
