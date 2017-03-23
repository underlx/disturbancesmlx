package interfaces

import (
	"errors"
	"fmt"

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
	lines := []*Line{}

	tx, err := node.Beginx()
	if err != nil {
		return lines, err
	}
	defer tx.Commit() // read-only tx

	rows, err := sdb.Select("id", "name", "network").
		From("mline").RunWith(tx).Query()
	if err != nil {
		return lines, fmt.Errorf("GetLines: %s", err)
	}
	defer rows.Close()

	for rows.Next() {
		var line Line
		var networkID string
		err := rows.Scan(
			&line.ID,
			&line.Name,
			&networkID)
		if err != nil {
			return lines, fmt.Errorf("GetLines: %s", err)
		}
		line.Network, err = GetNetwork(tx, networkID)
		if err != nil {
			return lines, fmt.Errorf("GetLines: %s", err)
		}
		lines = append(lines, &line)
	}
	if err := rows.Err(); err != nil {
		return lines, fmt.Errorf("GetLines: %s", err)
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
