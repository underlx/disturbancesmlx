package interfaces

import (
	"errors"
	"fmt"
	"time"

	sq "github.com/gbl08ma/squirrel"
	"github.com/heetch/sqalx"
)

// Line is a Network line
type Line struct {
	ID      string
	Name    string
	Network *Network
}

// GetLines returns a slice with all registered lines
func GetLines(node sqalx.Node) ([]*Line, error) {
	return getLinesWithSelect(node, sdb.Select())
}

func getLinesWithSelect(node sqalx.Node, sbuilder sq.SelectBuilder) ([]*Line, error) {
	lines := []*Line{}

	tx, err := node.Beginx()
	if err != nil {
		return lines, err
	}
	defer tx.Commit() // read-only tx

	rows, err := sbuilder.Columns("id", "name", "network").
		From("mline").
		RunWith(tx).Query()
	defer rows.Close()

	var networkIDs []string
	for rows.Next() {
		var line Line
		var networkID string
		err := rows.Scan(
			&line.ID,
			&line.Name,
			&networkID)
		if err != nil {
			return lines, fmt.Errorf("getLinesWithSelect: %s", err)
		}
		if err != nil {
			return lines, fmt.Errorf("getLinesWithSelect: %s", err)
		}
		lines = append(lines, &line)
		networkIDs = append(networkIDs, networkID)
	}
	if err := rows.Err(); err != nil {
		return lines, fmt.Errorf("getLinesWithSelect: %s", err)
	}
	for i := range networkIDs {
		lines[i].Network, err = GetNetwork(tx, networkIDs[i])
		if err != nil {
			return lines, fmt.Errorf("getLinesWithSelect: %s", err)
		}
	}
	return lines, nil
}

// GetLine returns the Line with the given ID
func GetLine(node sqalx.Node, id string) (*Line, error) {
	var line Line
	tx, err := node.Beginx()
	if err != nil {
		return &line, err
	}
	defer tx.Commit() // read-only tx

	var networkID string
	err = sdb.Select("id", "name", "network").
		From("mline").
		Where(sq.Eq{"id": id}).
		RunWith(tx).QueryRow().Scan(&line.ID, &line.Name, &networkID)
	if err != nil {
		return &line, errors.New("GetLine: " + err.Error())
	}
	line.Network, err = GetNetwork(tx, networkID)
	if err != nil {
		return &line, errors.New("GetLine: " + err.Error())
	}
	return &line, nil
}

// OngoingDisturbances returns a slice with all ongoing disturbances on this line
func (line *Line) OngoingDisturbances(node sqalx.Node) ([]*Disturbance, error) {
	s := sdb.Select().
		Where(sq.Eq{"mline": line.ID}).
		Where("time_end IS NULL").
		OrderBy("time_start ASC")
	return getDisturbancesWithSelect(node, s)
}

// DisturbancesBetween returns a slice with all disturbances that start or end between the specified times
func (line *Line) DisturbancesBetween(node sqalx.Node, startTime time.Time, endTime time.Time) ([]*Disturbance, error) {
	s := sdb.Select().
		Where(sq.Eq{"mline": line.ID}).
		Where(sq.Or{
			sq.Expr("time_start BETWEEN ? AND ?", startTime, endTime),
			sq.And{
				sq.Expr("time_end IS NOT NULL"),
				sq.Expr("time_end BETWEEN ? AND ?", startTime, endTime),
			},
		}).OrderBy("time_start ASC")
	return getDisturbancesWithSelect(node, s)
}

// LastDisturbance returns the latest disturbance affecting this line
func (line *Line) LastDisturbance(node sqalx.Node) (*Disturbance, error) {
	tx, err := node.Beginx()
	if err != nil {
		return nil, err
	}
	defer tx.Commit() // read-only tx

	// are there any ongoing disturbances? get the one with most recent START time
	s := sdb.Select().
		Where(sq.Eq{"mline": line.ID}).
		Where(sq.Expr("time_start = (SELECT MAX(time_start) FROM line_disturbance WHERE mline = ? AND time_end IS NULL)", line.ID)).
		OrderBy("time_start DESC")
	disturbances, err := getDisturbancesWithSelect(tx, s)
	if err != nil {
		return nil, err
	}
	if len(disturbances) <= 0 {
		// no ongoing disturbances. look at past ones and get the one with the most recent END time
		s := sdb.Select().
			Where(sq.Eq{"mline": line.ID}).
			Where(sq.Expr("time_end = (SELECT MAX(time_end) FROM line_disturbance WHERE mline = ?)", line.ID)).
			OrderBy("time_end DESC")
		disturbances, err = getDisturbancesWithSelect(tx, s)
		if err != nil {
			return nil, errors.New("LastDisturbance: " + err.Error())
		}
		if len(disturbances) == 0 {
			return nil, errors.New("No disturbances for this line")
		}
	}

	return disturbances[0], err
}

// Update adds or updates the line
func (line *Line) Update(node sqalx.Node) error {
	tx, err := node.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	err = line.Network.Update(tx)
	if err != nil {
		return errors.New("AddLine: " + err.Error())
	}

	_, err = sdb.Insert("mline").
		Columns("id", "name", "network").
		Values(line.ID, line.Name, line.Network.ID).
		Suffix("ON CONFLICT (id) DO UPDATE SET name = ?, network = ?",
			line.Name, line.Network.ID).
		RunWith(tx).Exec()

	if err != nil {
		return errors.New("AddLine: " + err.Error())
	}
	return tx.Commit()
}

// Delete deletes the line
func (line *Line) Delete(node sqalx.Node, lineID string) error {
	tx, err := node.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = sdb.Delete("mline").
		Where(sq.Eq{"id": line.ID}).RunWith(tx).Exec()
	if err != nil {
		return fmt.Errorf("RemoveLine: %s", err)
	}
	return tx.Commit()
}
