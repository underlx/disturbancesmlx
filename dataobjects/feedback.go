package dataobjects

import (
	"errors"
	"fmt"
	"time"

	"github.com/gbl08ma/sqalx"
	sq "github.com/Masterminds/squirrel"
)

// Feedback is a piece of user feedback about the service,
// like a bug report
type Feedback struct {
	ID        string
	Submitter *APIPair
	Time      time.Time
	Type      FeedbackType
	Contents  string
}

// FeedbackType corresponds to a type of feedback
type FeedbackType string

const (
	// S2LSincorrectDetection is a type of feedback reserved for incorrect detection of stations by the client
	S2LSincorrectDetection FeedbackType = "s2ls-incorrect-detection"
)

// GetFeedbacks returns a slice with all registered feedback
func GetFeedbacks(node sqalx.Node) ([]*Feedback, error) {
	return getFeedbacksWithSelect(node, sdb.Select())
}

func getFeedbacksWithSelect(node sqalx.Node, sbuilder sq.SelectBuilder) ([]*Feedback, error) {
	feedbacks := []*Feedback{}

	tx, err := node.Beginx()
	if err != nil {
		return feedbacks, err
	}
	defer tx.Commit() // read-only tx

	rows, err := sbuilder.Columns("id", "submitter", "timestamp", "type", "contents").
		From("feedback").
		RunWith(tx).Query()
	if err != nil {
		return feedbacks, fmt.Errorf("getFeedbacksWithSelect: %s", err)
	}
	defer rows.Close()

	submitters := []string{}
	for rows.Next() {
		var feedback Feedback
		var submitter string
		err := rows.Scan(
			&feedback.ID,
			&submitter,
			&feedback.Time,
			&feedback.Type,
			&feedback.Contents)
		if err != nil {
			return feedbacks, fmt.Errorf("getFeedbacksWithSelect: %s", err)
		}
		feedbacks = append(feedbacks, &feedback)
		submitters = append(submitters, submitter)
	}
	if err := rows.Err(); err != nil {
		return feedbacks, fmt.Errorf("getFeedbacksWithSelect: %s", err)
	}
	for i := range feedbacks {
		feedbacks[i].Submitter, err = GetPair(tx, submitters[i])
		if err != nil {
			return feedbacks, fmt.Errorf("getFeedbacksWithSelect: %s", err)
		}
	}
	return feedbacks, nil
}

// Update adds or updates the feedback
func (feedback *Feedback) Update(node sqalx.Node) error {
	tx, err := node.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = sdb.Insert("feedback").
		Columns("id", "submitter", "timestamp", "type", "contents").
		Values(feedback.ID, feedback.Submitter.Key, feedback.Time, feedback.Type, feedback.Contents).
		Suffix("ON CONFLICT (id) DO UPDATE SET submitter = ?, timestamp = ?, type = ?, contents = ?",
			feedback.Submitter.Key, feedback.Time, feedback.Type, feedback.Contents).
		RunWith(tx).Exec()

	if err != nil {
		return errors.New("AddFeedback: " + err.Error())
	}
	return tx.Commit()
}

// Delete deletes the feedback
func (feedback *Feedback) Delete(node sqalx.Node) error {
	tx, err := node.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = sdb.Delete("feedback").
		Where(sq.Eq{"id": feedback.ID}).
		RunWith(tx).Exec()
	if err != nil {
		return fmt.Errorf("RemoveFeedback: %s", err)
	}
	return tx.Commit()
}
