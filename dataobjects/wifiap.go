package dataobjects

import (
	"errors"
	"fmt"

	sq "github.com/gbl08ma/squirrel"
	"github.com/gbl08ma/sqalx"
)

// WiFiAP is a WiFi access point
type WiFiAP struct {
	BSSID string
	SSID  string
	Line  string
}

// GetWiFiAPs returns a slice with all registered wiFiAPs
func GetWiFiAPs(node sqalx.Node) ([]*WiFiAP, error) {
	return getWiFiAPsWithSelect(node, sdb.Select())
}

func getWiFiAPsWithSelect(node sqalx.Node, sbuilder sq.SelectBuilder) ([]*WiFiAP, error) {
	wiFiAPs := []*WiFiAP{}

	tx, err := node.Beginx()
	if err != nil {
		return wiFiAPs, err
	}
	defer tx.Commit() // read-only tx

	rows, err := sbuilder.Columns("wifiap.bssid", "wifiap.ssid", "line_id").
		From("wifiap").
		Join("station_has_wifiap ON wifiap.bssid = station_has_wifiap.bssid").
		RunWith(tx).Query()
	if err != nil {
		return wiFiAPs, fmt.Errorf("getWiFiAPsWithSelect: %s", err)
	}
	defer rows.Close()

	for rows.Next() {
		var wiFiAP WiFiAP
		err := rows.Scan(
			&wiFiAP.BSSID,
			&wiFiAP.SSID,
			&wiFiAP.Line)
		if err != nil {
			return wiFiAPs, fmt.Errorf("getWiFiAPsWithSelect: %s", err)
		}
		wiFiAPs = append(wiFiAPs, &wiFiAP)
	}
	if err := rows.Err(); err != nil {
		return wiFiAPs, fmt.Errorf("getWiFiAPsWithSelect: %s", err)
	}
	return wiFiAPs, nil
}

// GetWiFiAP returns the WiFiAP with the given BSSID
func GetWiFiAP(node sqalx.Node, bssid string) (*WiFiAP, error) {
	s := sdb.Select().
		Where(sq.Eq{"bssid": bssid})
	wiFiAPs, err := getWiFiAPsWithSelect(node, s)
	if err != nil {
		return nil, err
	}
	if len(wiFiAPs) == 0 {
		return nil, errors.New("WiFiAP not found")
	}
	return wiFiAPs[0], nil
}

// Stations returns the stations this wiFiAP belongs to
func (wiFiAP *WiFiAP) Stations(node sqalx.Node) ([]*Station, error) {
	s := sdb.Select().
		Join("station_has_wifiap ON bssid = ? AND station_id = id", wiFiAP.BSSID)
	return getStationsWithSelect(node, s)
}

// Update adds or updates the wiFiAP
func (wiFiAP *WiFiAP) Update(node sqalx.Node) error {
	tx, err := node.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = sdb.Insert("wifiap").
		Columns("bssid", "name").
		Values(wiFiAP.BSSID, wiFiAP.SSID).
		Suffix("ON CONFLICT (bssid) DO UPDATE SET ssid = ?",
			wiFiAP.SSID).
		RunWith(tx).Exec()

	if err != nil {
		return errors.New("AddWiFiAP: " + err.Error())
	}
	return tx.Commit()
}

// Delete deletes the wiFiAP
func (wiFiAP *WiFiAP) Delete(node sqalx.Node) error {
	tx, err := node.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = sdb.Delete("wifiap").
		Where(sq.Eq{"bssid": wiFiAP.BSSID}).RunWith(tx).Exec()
	if err != nil {
		return fmt.Errorf("RemoveWiFiAP: %s", err)
	}
	return tx.Commit()
}
