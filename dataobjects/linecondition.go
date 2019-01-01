package dataobjects

import (
	"errors"
	"fmt"

	"github.com/gbl08ma/sqalx"
	sq "github.com/gbl08ma/squirrel"
)

// LineCondition represents the operational condition of a Line at a certain point in time
type LineCondition struct {
	ID             string
	Time           Time
	Line           *Line
	TrainCars      int
	TrainFrequency Duration
	Source         *Source
}

// GetLineConditions returns a slice with all registered line conditions
func GetLineConditions(node sqalx.Node) ([]*LineCondition, error) {
	return getLineConditionsWithSelect(node, sdb.Select())
}

// GetLineCondition returns the LineCondition with the given ID
func GetLineCondition(node sqalx.Node, id string) (*LineCondition, error) {
	s := sdb.Select().
		Where(sq.Eq{"id": id})
	conditions, err := getLineConditionsWithSelect(node, s)
	if err != nil {
		return nil, err
	}
	if len(conditions) == 0 {
		return nil, errors.New("LineCondition not found")
	}
	return conditions[0], nil
}

func getLineConditionsWithSelect(node sqalx.Node, sbuilder sq.SelectBuilder) ([]*LineCondition, error) {
	conditions := []*LineCondition{}

	tx, err := node.Beginx()
	if err != nil {
		return conditions, err
	}
	defer tx.Commit() // read-only tx

	rows, err := sbuilder.Columns("id", "timestamp", "mline", "train_cars", "train_frequency", "source").
		From("line_condition").
		OrderBy("timestamp ASC").
		RunWith(tx).Query()
	if err != nil {
		return conditions, fmt.Errorf("getLineConditionsWithSelect: %s", err)
	}
	defer rows.Close()

	lineIDs := []string{}
	sourceIDs := []string{}
	for rows.Next() {
		var condition LineCondition
		var lineID, sourceID string
		err := rows.Scan(
			&condition.ID,
			&condition.Time,
			&lineID,
			&condition.TrainCars,
			&condition.TrainFrequency,
			&sourceID)
		if err != nil {
			return conditions, fmt.Errorf("getLineConditionsWithSelect: %s", err)
		}
		conditions = append(conditions, &condition)
		lineIDs = append(lineIDs, lineID)
		sourceIDs = append(sourceIDs, sourceID)
	}
	if err := rows.Err(); err != nil {
		return conditions, fmt.Errorf("getLineConditionsWithSelect: %s", err)
	}
	for i := range lineIDs {
		conditions[i].Line, err = GetLine(tx, lineIDs[i])
		if err != nil {
			return conditions, fmt.Errorf("getLineConditionsWithSelect: %s", err)
		}
		conditions[i].Source, err = GetSource(tx, sourceIDs[i])
		if err != nil {
			return conditions, fmt.Errorf("getLineConditionsWithSelect: %s", err)
		}
	}
	return conditions, nil
}

// Update adds or updates the LineCondition
func (condition *LineCondition) Update(node sqalx.Node) error {
	tx, err := node.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	err = condition.Source.Update(tx)
	if err != nil {
		return errors.New("AddLineCondition: " + err.Error())
	}

	_, err = sdb.Insert("line_condition").
		Columns("id", "timestamp", "mline", "train_cars", "train_frequency", "source").
		Values(condition.ID, condition.Time, condition.Line.ID, condition.TrainCars, condition.TrainFrequency, condition.Source.ID).
		Suffix("ON CONFLICT (id) DO UPDATE SET timestamp = ?, mline = ?, train_cars = ?, train_frequency = ?, source = ?",
			condition.Time, condition.Line.ID, condition.TrainCars, condition.TrainFrequency, condition.Source.ID).
		RunWith(tx).Exec()

	if err != nil {
		return errors.New("AddLineCondition: " + err.Error())
	}
	return tx.Commit()
}

// Delete deletes the LineCondition
func (condition *LineCondition) Delete(node sqalx.Node) error {
	tx, err := node.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = sdb.Delete("line_condition").
		Where(sq.Eq{"id": condition.ID}).RunWith(tx).Exec()
	if err != nil {
		return fmt.Errorf("RemoveLineCondition: %s", err)
	}
	return tx.Commit()
}
