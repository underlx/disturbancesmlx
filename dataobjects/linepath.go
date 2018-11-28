package dataobjects

import (
	"errors"
	"fmt"

	"github.com/gbl08ma/sqalx"
	sq "github.com/gbl08ma/squirrel"
	"github.com/jackc/pgx/pgtype"
)

// LinePath is a Line path
type LinePath struct {
	Line *Line
	ID   string
	Path pgtype.Path
}

// GetLinePaths returns a slice with all registered paths
func GetLinePaths(node sqalx.Node) ([]*LinePath, error) {
	return getLinePathsWithSelect(node, sdb.Select())
}

func getLinePathsWithSelect(node sqalx.Node, sbuilder sq.SelectBuilder) ([]*LinePath, error) {
	paths := []*LinePath{}

	tx, err := node.Beginx()
	if err != nil {
		return paths, err
	}
	defer tx.Commit() // read-only tx

	rows, err := sbuilder.Columns("line_id", "id", "path").
		From("line_path").
		RunWith(tx).Query()
	if err != nil {
		return paths, fmt.Errorf("getLinePathsWithSelect: %s", err)
	}
	defer rows.Close()

	var lineIDs []string
	for rows.Next() {
		var path LinePath
		var lineID string
		err := rows.Scan(
			&lineID,
			&path.ID,
			&path.Path)
		if err != nil {
			return paths, fmt.Errorf("getLinePathsWithSelect: %s", err)
		}
		paths = append(paths, &path)
		lineIDs = append(lineIDs, lineID)
	}
	if err := rows.Err(); err != nil {
		return paths, fmt.Errorf("getLinePathsWithSelect: %s", err)
	}
	for i := range lineIDs {
		paths[i].Line, err = GetLine(tx, lineIDs[i])
		if err != nil {
			return paths, fmt.Errorf("getLinePathsWithSelect: %s", err)
		}
	}
	return paths, nil
}

// Update adds or updates the path
func (path *LinePath) Update(node sqalx.Node) error {
	tx, err := node.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	err = path.Line.Update(tx)
	if err != nil {
		return errors.New("AddPath: " + err.Error())
	}

	_, err = sdb.Insert("line_path").
		Columns("line_id", "id", "path").
		Values(path.Line.ID, path.ID, path.Path).
		Suffix("ON CONFLICT (line_id, id) DO UPDATE SET path = ?", path.Path).
		RunWith(tx).Exec()

	if err != nil {
		return errors.New("AddLinePath: " + err.Error())
	}
	return tx.Commit()
}

// Delete deletes the path
func (path *LinePath) Delete(node sqalx.Node) error {
	tx, err := node.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = sdb.Delete("line_path").
		Where(sq.Eq{"line_id": path.Line.ID},
			sq.Eq{"id": path.ID}).
		RunWith(tx).Exec()
	if err != nil {
		return fmt.Errorf("RemoveLinePath: %s", err)
	}
	return tx.Commit()
}
