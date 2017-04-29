package dataobjects

import (
	"errors"
	"fmt"

	sq "github.com/gbl08ma/squirrel"
	"github.com/heetch/sqalx"
)

// Features contains station features
type Features struct {
	StationID string
	Lift      bool
	Bus       bool
	Boat      bool
	Train     bool
	Airport   bool
}

// GetFeatures returns a slice with all registered features
func GetFeatures(node sqalx.Node) ([]*Features, error) {
	return getFeaturesWithSelect(node, sdb.Select())
}

func getFeaturesWithSelect(node sqalx.Node, sbuilder sq.SelectBuilder) ([]*Features, error) {
	features := []*Features{}

	tx, err := node.Beginx()
	if err != nil {
		return features, err
	}
	defer tx.Commit() // read-only tx

	rows, err := sbuilder.Columns("station_id", "lift", "bus", "boat", "train", "airport").
		From("station_feature").
		RunWith(tx).Query()
	if err != nil {
		return features, fmt.Errorf("getFeaturesWithSelect: %s", err)
	}
	defer rows.Close()

	for rows.Next() {
		var f Features
		err := rows.Scan(
			&f.StationID,
			&f.Lift,
			&f.Bus,
			&f.Boat,
			&f.Train,
			&f.Airport)
		if err != nil {
			return features, fmt.Errorf("getFeaturesWithSelect: %s", err)
		}
		features = append(features, &f)
	}
	if err := rows.Err(); err != nil {
		return features, fmt.Errorf("getFeaturesWithSelect: %s", err)
	}
	return features, nil
}

// GetFeaturesForStation returns the Features for the station with the given ID
func GetFeaturesForStation(node sqalx.Node, id string) (*Features, error) {
	var features Features
	tx, err := node.Beginx()
	if err != nil {
		return &features, err
	}
	defer tx.Commit() // read-only tx

	err = sdb.Select("station_id", "lift", "bus", "boat", "train", "airport").
		From("station_feature").
		Where(sq.Eq{"station_id": id}).
		RunWith(tx).QueryRow().
		Scan(&features.StationID, &features.Lift, &features.Bus, &features.Boat,
			&features.Train, &features.Airport)
	if err != nil {
		return &features, errors.New("GetFeatures: " + err.Error())
	}
	return &features, nil
}

// Update adds or updates features
func (features *Features) Update(node sqalx.Node) error {
	tx, err := node.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = sdb.Insert("station_feature").
		Columns("station_id", "lift", "bus", "boat", "train", "airport").
		Values(features.StationID, features.Lift, features.Bus, features.Boat,
			features.Train, features.Airport).
		Suffix("ON CONFLICT (station_id) DO UPDATE SET lift = ?, bus = ?, boat = ?, train = ?, airport = ?",
			features.Lift, features.Bus, features.Boat, features.Train, features.Airport).
		RunWith(tx).Exec()

	if err != nil {
		return errors.New("AddFeatures: " + err.Error())
	}
	return tx.Commit()
}

// Delete deletes the features
func (features *Features) Delete(node sqalx.Node) error {
	tx, err := node.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = sdb.Delete("station_feature").
		Where(sq.Eq{"station_id": features.StationID}).RunWith(tx).Exec()
	if err != nil {
		return fmt.Errorf("RemoveFeatures: %s", err)
	}
	return tx.Commit()
}
