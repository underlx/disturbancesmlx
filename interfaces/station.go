package interfaces

import (
	"errors"
	"fmt"

	sq "github.com/gbl08ma/squirrel"
	"github.com/heetch/sqalx"
)

// Station is a Network station
type Station struct {
	ID      string
	Name    string
	Network *Network
}

// GetStations returns a slice with all registered stations
func GetStations(node sqalx.Node) ([]*Station, error) {
	return getStationsWithSelect(node, sdb.Select())
}

func getStationsWithSelect(node sqalx.Node, sbuilder sq.SelectBuilder) ([]*Station, error) {
	stations := []*Station{}

	tx, err := node.Beginx()
	if err != nil {
		return stations, err
	}
	defer tx.Commit() // read-only tx

	rows, err := sbuilder.Columns("id", "name", "network").
		From("station").
		RunWith(tx).Query()
	if err != nil {
		return stations, fmt.Errorf("getStationsWithSelect: %s", err)
	}
	defer rows.Close()

	var networkIDs []string
	for rows.Next() {
		var station Station
		var networkID string
		err := rows.Scan(
			&station.ID,
			&station.Name,
			&networkID)
		if err != nil {
			return stations, fmt.Errorf("getStationsWithSelect: %s", err)
		}
		if err != nil {
			return stations, fmt.Errorf("getStationsWithSelect: %s", err)
		}
		stations = append(stations, &station)
		networkIDs = append(networkIDs, networkID)
	}
	if err := rows.Err(); err != nil {
		return stations, fmt.Errorf("getStationsWithSelect: %s", err)
	}
	for i := range networkIDs {
		stations[i].Network, err = GetNetwork(tx, networkIDs[i])
		if err != nil {
			return stations, fmt.Errorf("getStationsWithSelect: %s", err)
		}
	}
	return stations, nil
}

// GetStation returns the Station with the given ID
func GetStation(node sqalx.Node, id string) (*Station, error) {
	var station Station
	tx, err := node.Beginx()
	if err != nil {
		return &station, err
	}
	defer tx.Commit() // read-only tx

	var networkID string
	err = sdb.Select("id", "name", "network").
		From("station").
		Where(sq.Eq{"id": id}).
		RunWith(tx).QueryRow().
		Scan(&station.ID, &station.Name, &networkID)
	if err != nil {
		return &station, errors.New("GetStation: " + err.Error())
	}
	station.Network, err = GetNetwork(tx, networkID)
	if err != nil {
		return &station, errors.New("GetStation: " + err.Error())
	}
	return &station, nil
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
		Join("station_has_wifiap ON station_id = ? AND wifiap.bssid = station_has_wifiap.bssid", station.ID)
	return getWiFiAPsWithSelect(node, s)
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
		Columns("id", "name", "network").
		Values(station.ID, station.Name, station.Network.ID).
		Suffix("ON CONFLICT (id) DO UPDATE SET name = ?, network = ?",
			station.Name, station.Network.ID).
		RunWith(tx).Exec()

	if err != nil {
		return errors.New("AddStation: " + err.Error())
	}
	return tx.Commit()
}

// Delete deletes the station
func (station *Station) Delete(node sqalx.Node, stationID string) error {
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
