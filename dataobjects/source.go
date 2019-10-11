package dataobjects

import (
	"errors"
	"fmt"

	"github.com/gbl08ma/sqalx"
	sq "github.com/Masterminds/squirrel"
)

// Source represents a Status source
type Source struct {
	ID        string
	Name      string
	Automatic bool
	Official  bool
}

// GetSources returns a slice with all registered sources
func GetSources(node sqalx.Node) ([]*Source, error) {
	return getSourcesWithSelect(node, sdb.Select())
}

// GetSource returns the Source with the given ID
func GetSource(node sqalx.Node, id string) (*Source, error) {
	if value, present := node.Load(getCacheKey("source", id)); present {
		return value.(*Source), nil
	}
	s := sdb.Select().
		Where(sq.Eq{"id": id})
	sources, err := getSourcesWithSelect(node, s)
	if err != nil {
		return nil, err
	}
	if len(sources) == 0 {
		return nil, errors.New("Source not found")
	}
	node.Store(getCacheKey("source", id), sources[0])
	return sources[0], nil
}

func getSourcesWithSelect(node sqalx.Node, sbuilder sq.SelectBuilder) ([]*Source, error) {
	sources := []*Source{}

	tx, err := node.Beginx()
	if err != nil {
		return sources, err
	}
	defer tx.Commit() // read-only tx

	rows, err := sbuilder.Columns("id", "name", "automatic", "official").
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
			&source.Automatic,
			&source.Official)
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

// Update adds or updates the source
func (source *Source) Update(node sqalx.Node) error {
	tx, err := node.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = sdb.Insert("source").
		Columns("id", "name", "automatic", "official").
		Values(source.ID, source.Name, source.Automatic, source.Official).
		Suffix("ON CONFLICT (id) DO UPDATE SET name = ?, automatic = ?, official = ?",
			source.Name, source.Automatic, source.Official).
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
	tx.Delete(getCacheKey("source", source.ID))
	return tx.Commit()
}
