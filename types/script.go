package types

import (
	"errors"
	"fmt"

	"github.com/gbl08ma/sqalx"
	sq "github.com/Masterminds/squirrel"
)

// Script contains dynamic behavior for the system to implement at run time
type Script struct {
	ID      string
	Type    string
	Autorun int
	Code    string
	Notes   string
}

// GetScripts returns a slice with all registered Scripts
func GetScripts(node sqalx.Node) ([]*Script, error) {
	return getScriptsWithSelect(node, sdb.Select())
}

// GetScriptsWithType returns a slice with all registered Scripts with the given type
func GetScriptsWithType(node sqalx.Node, scriptType string) ([]*Script, error) {
	return getScriptsWithSelect(node, sdb.Select().
		Where(sq.Eq{"script.type": scriptType}))
}

// GetAutorunScriptsWithType returns a slice with all registered Scripts with the given type and specified autorun level
func GetAutorunScriptsWithType(node sqalx.Node, scriptType string, autorunLevel int) ([]*Script, error) {
	return getScriptsWithSelect(node, sdb.Select().
		Where(sq.Eq{"script.type": scriptType}).
		Where(sq.Eq{"script.autorun": autorunLevel}))
}

func getScriptsWithSelect(node sqalx.Node, sbuilder sq.SelectBuilder) ([]*Script, error) {
	scripts := []*Script{}

	tx, err := node.Beginx()
	if err != nil {
		return scripts, err
	}
	defer tx.Commit() // read-only tx

	rows, err := sbuilder.Columns("script.id", "script.type", "script.autorun", "script.code", "script.notes").
		From("script").
		RunWith(tx).Query()
	if err != nil {
		return scripts, fmt.Errorf("getScriptsWithSelect: %s", err)
	}
	defer rows.Close()

	for rows.Next() {
		var script Script
		err := rows.Scan(
			&script.ID,
			&script.Type,
			&script.Autorun,
			&script.Code,
			&script.Notes)
		if err != nil {
			return scripts, fmt.Errorf("getScriptsWithSelect: %s", err)
		}
		scripts = append(scripts, &script)
	}
	if err := rows.Err(); err != nil {
		return scripts, fmt.Errorf("getScriptsWithSelect: %s", err)
	}
	return scripts, nil
}

// GetScript returns the Script with the given ID
func GetScript(node sqalx.Node, id string) (*Script, error) {
	s := sdb.Select().
		Where(sq.Eq{"script.id": id})
	scripts, err := getScriptsWithSelect(node, s)
	if err != nil {
		return nil, err
	}
	if len(scripts) == 0 {
		return nil, errors.New("Script not found")
	}
	return scripts[0], nil
}

// Update adds or updates the script
func (script *Script) Update(node sqalx.Node) error {
	tx, err := node.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = sdb.Insert("script").
		Columns("id", "type", "autorun", "code", "notes").
		Values(script.ID, script.Type, script.Autorun, script.Code, script.Notes).
		Suffix("ON CONFLICT (id) DO UPDATE SET type = ?, autorun = ?, code = ?, notes = ?",
			script.Type, script.Autorun, script.Code, script.Notes).
		RunWith(tx).Exec()

	if err != nil {
		return errors.New("AddScript: " + err.Error())
	}
	return tx.Commit()
}

// Delete deletes the script
func (script *Script) Delete(node sqalx.Node) error {
	tx, err := node.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = sdb.Delete("script").
		Where(sq.Eq{"id": script.ID}).RunWith(tx).Exec()
	if err != nil {
		return fmt.Errorf("RemoveScript: %s", err)
	}
	return tx.Commit()
}
