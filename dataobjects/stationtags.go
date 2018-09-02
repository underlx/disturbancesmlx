package dataobjects

import (
	"fmt"

	sq "github.com/gbl08ma/squirrel"
	"github.com/gbl08ma/sqalx"
)

// StationTags contains station tags
type StationTags struct {
	StationID string
	Tags      []string
	LowTags   []string
}

// GetStationTags returns a slice with all registered tags
func GetStationTags(node sqalx.Node) ([]*StationTags, error) {
	return getStationTagsWithSelect(node, sdb.Select())
}

func getStationTagsWithSelect(node sqalx.Node, sbuilder sq.SelectBuilder) ([]*StationTags, error) {
	tags := []*StationTags{}
	tagsMap := make(map[string]*StationTags)

	tx, err := node.Beginx()
	if err != nil {
		return tags, err
	}
	defer tx.Commit() // read-only tx

	rows, err := sbuilder.Columns("station_id", "tag", "priority").
		From("station_tag").
		OrderBy("station_id, priority ASC").
		RunWith(tx).Query()
	if err != nil {
		return tags, fmt.Errorf("getStationTagsWithSelect: %s", err)
	}
	defer rows.Close()

	for rows.Next() {
		var id string
		var tag string
		var priority int
		err := rows.Scan(&id, &tag, &priority)
		if err != nil {
			return tags, fmt.Errorf("getStationTagsWithSelect: %s", err)
		}
		if _, present := tagsMap[id]; !present {
			tagsMap[id] = &StationTags{
				StationID: id,
				Tags:      []string{},
				LowTags:   []string{},
			}
			tags = append(tags, tagsMap[id])
		}
		if priority >= 1000 {
			tagsMap[id].LowTags = append(tagsMap[id].LowTags, tag)
		} else {
			tagsMap[id].Tags = append(tagsMap[id].Tags, tag)
		}
	}
	if err := rows.Err(); err != nil {
		return tags, fmt.Errorf("getStationTagsWithSelect: %s", err)
	}
	return tags, nil
}

// getStationTagsForStation returns the StationTags for the station with the given ID
func getStationTagsForStation(node sqalx.Node, id string) (*StationTags, error) {
	s := sdb.Select().
		Where(sq.Eq{"station_id": id})
	tags, err := getStationTagsWithSelect(node, s)
	if err != nil {
		return nil, err
	}
	if len(tags) == 0 {
		// station does not have tags
		return &StationTags{
			StationID: id,
			Tags:      []string{},
			LowTags:   []string{},
		}, nil
	}
	return tags[0], nil
}
