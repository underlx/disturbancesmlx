package dataobjects

import (
	"errors"
	"fmt"

	sq "github.com/gbl08ma/squirrel"
	"github.com/heetch/sqalx"
)

// Source represents a Status source
type Source struct {
	ID          string
	Name        string
	IsAutomatic bool
}

// GetSources returns a slice with all registered sources
func GetSources(node sqalx.Node) ([]*Source, error) {
	sources := []*Source{}

	tx, err := node.Beginx()
	if err != nil {
		return sources, err
	}
	defer tx.Commit() // read-only tx

	rows, err := sdb.Select("id", "name", "automatic").
		From("source").RunWith(tx).Query()
	if err != nil {
		return sources, fmt.Errorf("GetSources: %s", err)
	}
	defer rows.Close()

	for rows.Next() {
		var source Source
		err := rows.Scan(
			&source.ID,
			&source.Name,
			&source.IsAutomatic)
		if err != nil {
			return sources, fmt.Errorf("GetSources: %s", err)
		}
		sources = append(sources, &source)
	}
	if err := rows.Err(); err != nil {
		return sources, fmt.Errorf("GetSources: %s", err)
	}
	return sources, nil
}

// GetSource returns the Source with the given ID
func GetSource(node sqalx.Node, id string) (*Source, error) {
	var source Source
	tx, err := node.Beginx()
	if err != nil {
		return &source, err
	}
	defer tx.Commit() // read-only tx

	err = sdb.Select("id", "name").
		From("source").
		Where(sq.Eq{"id": id}).
		RunWith(tx).QueryRow().Scan(&source.ID, &source.Name)
	if err != nil {
		return &source, errors.New("GetSource: " + err.Error())
	}
	return &source, nil
}

// Update adds or updates the source
func (source *Source) Update(node sqalx.Node) error {
	tx, err := node.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = sdb.Insert("source").
		Columns("id", "name", "automatic").
		Values(source.ID, source.Name, source.IsAutomatic).
		Suffix("ON CONFLICT (id) DO UPDATE SET name = ?",
			source.Name).
		RunWith(tx).Exec()

	if err != nil {
		return errors.New("AddSource: " + err.Error())
	}
	return tx.Commit()
}

// Delete deletes the source
func (source *Source) Delete(node sqalx.Node) error {
	tx, err := node.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = sdb.Delete("source").
		Where(sq.Eq{"id": source.ID}).RunWith(tx).Exec()
	if err != nil {
		return fmt.Errorf("RemoveSource: %s", err)
	}
	return tx.Commit()
}
