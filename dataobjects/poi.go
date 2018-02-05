package dataobjects

import (
	"errors"
	"fmt"

	sq "github.com/gbl08ma/squirrel"
	"github.com/heetch/sqalx"
)

// POI is a Point of Interest
type POI struct {
	ID         string
	Type       string
	WorldCoord [2]float64
	URL        string
	MainLocale string
	Names      map[string]string
}

// GetPOIs returns a slice with all registered POIs
func GetPOIs(node sqalx.Node) ([]*POI, error) {
	return getPOIsWithSelect(node, sdb.Select())
}

func getPOIsWithSelect(node sqalx.Node, sbuilder sq.SelectBuilder) ([]*POI, error) {
	pois := []*POI{}
	poiMap := make(map[string]*POI)

	tx, err := node.Beginx()
	if err != nil {
		return pois, err
	}
	defer tx.Commit() // read-only tx

	rows, err := sbuilder.Columns("poi.id", "poi.type", "poi.world_coord", "poi.web_url").
		From("poi").
		Join("poi_name ON poi.id = poi_name.id AND poi_name.main = true").
		OrderBy("poi_name.name ASC").
		RunWith(tx).Query()
	if err != nil {
		return pois, fmt.Errorf("getPOIsWithSelect: %s", err)
	}
	defer rows.Close()

	for rows.Next() {
		var poi POI
		var worldCoord Point
		err := rows.Scan(
			&poi.ID,
			&poi.Type,
			&worldCoord,
			&poi.URL)
		if err != nil {
			return pois, fmt.Errorf("getPOIsWithSelect: %s", err)
		}
		poi.WorldCoord[0] = worldCoord[0]
		poi.WorldCoord[1] = worldCoord[1]
		pois = append(pois, &poi)
		poiMap[poi.ID] = &poi
		poiMap[poi.ID].Names = make(map[string]string)
	}
	if err := rows.Err(); err != nil {
		return pois, fmt.Errorf("getPOIsWithSelect: %s", err)
	}

	// get MainLocale for each POI
	rows2, err := sbuilder.Columns("poi.id", "poi_name.lang").
		From("poi").
		Join("poi_name ON poi.id = poi_name.id AND poi_name.main = true").
		RunWith(tx).Query()
	if err != nil {
		return pois, fmt.Errorf("getPOIsWithSelect: %s", err)
	}
	defer rows2.Close()

	for rows2.Next() {
		var id string
		var lang string
		err := rows2.Scan(&id, &lang)
		if err != nil {
			return pois, fmt.Errorf("getPOIsWithSelect: %s", err)
		}
		poiMap[id].MainLocale = lang
	}
	if err := rows2.Err(); err != nil {
		return pois, fmt.Errorf("getPOIsWithSelect: %s", err)
	}

	// get localized name map for each POI
	rows3, err := sbuilder.Columns("poi.id", "poi_name.lang", "poi_name.name").
		From("poi").
		Join("poi_name ON poi.id = poi_name.id").
		RunWith(tx).Query()
	if err != nil {
		return pois, fmt.Errorf("getPOIsWithSelect: %s", err)
	}
	defer rows3.Close()

	for rows3.Next() {
		var id string
		var lang string
		var name string
		err := rows3.Scan(&id, &lang, &name)
		if err != nil {
			return pois, fmt.Errorf("getPOIsWithSelect: %s", err)
		}
		poiMap[id].Names[lang] = name
	}
	if err := rows3.Err(); err != nil {
		return pois, fmt.Errorf("getPOIsWithSelect: %s", err)
	}
	return pois, nil
}

// GetPOI returns the POI with the given ID
func GetPOI(node sqalx.Node, id string) (*POI, error) {
	s := sdb.Select().
		Where(sq.Eq{"poi.id": id})
	pois, err := getPOIsWithSelect(node, s)
	if err != nil {
		return nil, err
	}
	if len(pois) == 0 {
		return nil, errors.New("POI not found")
	}
	return pois[0], nil
}

// Stations returns the stations this POI is associated with
func (poi *POI) Stations(node sqalx.Node) ([]*Station, error) {
	s := sdb.Select().
		Join("station_has_poi ON poi_id = ? AND station_id = id", poi.ID)
	return getStationsWithSelect(node, s)
}
