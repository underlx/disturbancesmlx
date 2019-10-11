package dataobjects

import (
	"errors"
	"fmt"
	"time"

	"github.com/gbl08ma/sqalx"
	sq "github.com/Masterminds/squirrel"
	"github.com/lib/pq"
)

// Dataset is a dataset
type Dataset struct {
	Version string
	Authors pq.StringArray
	Network *Network
}

// GetDatasets returns a slice with all registered Datasets
func GetDatasets(node sqalx.Node) ([]*Dataset, error) {
	return getDatasetsWithSelect(node, sdb.Select())
}

func getDatasetsWithSelect(node sqalx.Node, sbuilder sq.SelectBuilder) ([]*Dataset, error) {
	datasets := []*Dataset{}

	tx, err := node.Beginx()
	if err != nil {
		return datasets, err
	}
	defer tx.Commit() // read-only tx

	rows, err := sbuilder.Columns("network_id", "version", "authors").
		From("dataset_info").
		RunWith(tx).Query()
	if err != nil {
		return datasets, fmt.Errorf("getDatasetsWithSelect: %s", err)
	}
	defer rows.Close()

	var networkIDs []string
	for rows.Next() {
		var dataset Dataset
		var version time.Time
		var networkID string
		err := rows.Scan(
			&networkID,
			&version,
			&dataset.Authors)
		if err != nil {
			return datasets, fmt.Errorf("getDatasetsWithSelect: %s", err)
		}
		networkIDs = append(networkIDs, networkID)
		dataset.Version = version.Format(time.RFC3339)
		datasets = append(datasets, &dataset)
	}
	if err := rows.Err(); err != nil {
		return datasets, fmt.Errorf("getDatasetsWithSelect: %s", err)
	}

	for i := range networkIDs {
		datasets[i].Network, err = GetNetwork(tx, networkIDs[i])
		if err != nil {
			return datasets, fmt.Errorf("getDatasetsWithSelect: %s", err)
		}
	}

	return datasets, nil
}

// GetDataset returns the Dataset with the given network ID
func GetDataset(node sqalx.Node, networkID string) (*Dataset, error) {
	s := sdb.Select().
		Where(sq.Eq{"network_id": networkID})
	datasets, err := getDatasetsWithSelect(node, s)
	if err != nil {
		return nil, err
	}
	if len(datasets) == 0 {
		return nil, errors.New("Dataset not found")
	}
	return datasets[0], nil
}
