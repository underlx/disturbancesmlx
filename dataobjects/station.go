package dataobjects

import (
	"errors"
	"fmt"

	sq "github.com/gbl08ma/squirrel"
	"github.com/heetch/sqalx"
	"github.com/lib/pq"
)

// Station is a Network station
type Station struct {
	ID       string
	Name     string
	AltNames []string
	Tags     []string
	LowTags  []string
	Network  *Network
}

// GetStations returns a slice with all registered stations
func GetStations(node sqalx.Node) ([]*Station, error) {
	return getStationsWithSelect(node, sdb.Select())
}

func getStationsWithSelect(node sqalx.Node, sbuilder sq.SelectBuilder) ([]*Station, error) {
	stations := []*Station{}
	stationMap := make(map[string]*Station)

	tx, err := node.Beginx()
	if err != nil {
		return stations, err
	}
	defer tx.Commit() // read-only tx

	rows, err := sbuilder.Columns("id", "name", "alt_names", "network").
		From("station").
		RunWith(tx).Query()
	if err != nil {
		return stations, fmt.Errorf("getStationsWithSelect: %s", err)
	}
	defer rows.Close()

	var stationIDs []string
	for rows.Next() {
		var station Station
		var altNames pq.StringArray
		var networkID string
		err := rows.Scan(
			&station.ID,
			&station.Name,
			&altNames,
			&networkID)
		if err != nil {
			return stations, fmt.Errorf("getStationsWithSelect: %s", err)
		}
		station.AltNames = altNames
		stations = append(stations, &station)
		stationMap[station.ID] = &station
		stationIDs = append(stationIDs, networkID)
	}
	if err := rows.Err(); err != nil {
		return stations, fmt.Errorf("getStationsWithSelect: %s", err)
	}
	for i := range stationIDs {
		stations[i].Network, err = GetNetwork(tx, stationIDs[i])
		if err != nil {
			return stations, fmt.Errorf("getStationsWithSelect: %s", err)
		}
		stationTags, err := getStationTagsForStation(tx, stations[i].ID)
		if err != nil {
			return stations, fmt.Errorf("getStationsWithSelect: %s", err)
		}
		stations[i].Tags = stationTags.Tags
		stations[i].LowTags = stationTags.LowTags
	}

	return stations, nil
}

// GetStation returns the Station with the given ID
func GetStation(node sqalx.Node, id string) (*Station, error) {
	s := sdb.Select().
		Where(sq.Eq{"id": id})
	stations, err := getStationsWithSelect(node, s)
	if err != nil {
		return nil, err
	}
	if len(stations) == 0 {
		return nil, errors.New("Station not found")
	}
	return stations[0], nil
}

// Lines returns the lines that serve this station
func (station *Station) Lines(node sqalx.Node) ([]*Line, error) {
	s := sdb.Select().
		Join("line_has_station ON station_id = ? AND line_id = id", station.ID)
	return getLinesWithSelect(node, s)
}

// WiFiAPs returns the WiFi APs that are available in this station
func (station *Station) WiFiAPs(node sqalx.Node) ([]*WiFiAP, error) {
	s := sdb.Select().
		Where(sq.Eq{"station_id": station.ID})
	return getWiFiAPsWithSelect(node, s)
}

// Lobbies returns the lobbies of this station
func (station *Station) Lobbies(node sqalx.Node) ([]*Lobby, error) {
	s := sdb.Select().
		Where(sq.Eq{"station_id": station.ID})
	return getLobbiesWithSelect(node, s)
}

// POIs returns the POIs associated with this station
func (station *Station) POIs(node sqalx.Node) ([]*POI, error) {
	s := sdb.Select().
		Join("station_has_poi ON station_has_poi.station_id = ? AND station_has_poi.poi_id = poi.id", station.ID)
	return getPOIsWithSelect(node, s)
}

// Directions returns the directions (stations at an end of a line) that can be reached directly from this station
// (i.e. without additional line changes)
func (station *Station) Directions(node sqalx.Node) ([]*Station, error) {
	tx, err := node.Beginx()
	if err != nil {
		return nil, err
	}
	defer tx.Commit() // read-only tx

	lines, err := station.Lines(tx)
	if err != nil {
		return nil, err
	}

	ends := []*Station{}
	for _, line := range lines {
		stations, err := line.Stations(tx)
		if err != nil {
			return nil, err
		}
		if len(stations) > 0 {
			ends = append(ends, stations[0])
			if len(stations) > 1 {
				ends = append(ends, stations[len(stations)-1])
			}
		}
	}
	return ends, nil
}

// Closed returns whether this station is closed
func (station *Station) Closed(node sqalx.Node) (bool, error) {
	tx, err := node.Beginx()
	if err != nil {
		return false, err
	}
	defer tx.Commit() // read-only tx

	lobbies, err := station.Lobbies(tx)
	if err != nil {
		return false, err
	}
	for _, lobby := range lobbies {
		closed, err := lobby.Closed(tx)
		if err != nil {
			return false, err
		}
		if !closed {
			return false, nil
		}
	}
	return true, nil
}

// HasTag returns true if this station was assigned the provided tag
func (station *Station) HasTag(needle string) bool {
	for _, tag := range station.Tags {
		if tag == needle {
			return true
		}
	}
	for _, tag := range station.LowTags {
		if tag == needle {
			return true
		}
	}
	return false
}

// Update adds or updates the station
func (station *Station) Update(node sqalx.Node) error {
	tx, err := node.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	err = station.Network.Update(tx)
	if err != nil {
		return errors.New("AddStation: " + err.Error())
	}

	_, err = sdb.Insert("station").
		Columns("id", "name", "network", "alt_names").
		Values(station.ID, station.Name, station.Network.ID, station.AltNames).
		Suffix("ON CONFLICT (id) DO UPDATE SET name = ?, network = ?, alt_names = ?",
			station.Name, station.Network.ID, station.AltNames).
		RunWith(tx).Exec()

	if err != nil {
		return errors.New("AddStation: " + err.Error())
	}
	return tx.Commit()
}

// Delete deletes the station
func (station *Station) Delete(node sqalx.Node) error {
	tx, err := node.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = sdb.Delete("station").
		Where(sq.Eq{"id": station.ID}).RunWith(tx).Exec()
	if err != nil {
		return fmt.Errorf("RemoveStation: %s", err)
	}
	return tx.Commit()
}
