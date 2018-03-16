package resource

import (
	"time"

	"github.com/heetch/sqalx"
	"github.com/yarf-framework/yarf"

	sq "github.com/gbl08ma/squirrel"
	"github.com/lib/pq"
)

// Dataset composites resource
type Dataset struct {
	resource
	sdb *sq.StatementBuilderType
}

type apiDataset struct {
	NetworkID string         `msgpack:"network" json:"network"`
	Version   string         `msgpack:"version" json:"version"`
	Authors   pq.StringArray `msgpack:"authors" json:"authors"`
}

// WithNode associates a sqalx Node with this resource
func (r *Dataset) WithNode(node sqalx.Node) *Dataset {
	r.node = node
	return r
}

// WithSquirrel associates a statement builder with this resource
func (r *Dataset) WithSquirrel(sdb *sq.StatementBuilderType) *Dataset {
	r.sdb = sdb
	return r
}

// Get serves HTTP GET requests on this resource
func (r *Dataset) Get(c *yarf.Context) error {
	tx, err := r.Beginx()
	if err != nil {
		return err
	}
	defer tx.Commit() // read-only tx

	if c.Param("id") != "" {
		var dataset apiDataset
		var version time.Time
		err := r.sdb.Select("network_id", "version", "authors").
			From("dataset_info").RunWith(tx).QueryRow().Scan(
			&dataset.NetworkID,
			&version,
			&dataset.Authors)
		if err != nil {
			return err
		}

		dataset.Version = version.Format(time.RFC3339)

		RenderData(c, dataset)
	} else {
		datasets := []*apiDataset{}
		rows, err := r.sdb.Select("network_id", "version", "authors").
			From("dataset_info").RunWith(tx).Query()
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var dataset apiDataset
			var version time.Time
			err := rows.Scan(
				&dataset.NetworkID,
				&version,
				&dataset.Authors)
			if err != nil {
				return err
			}

			dataset.Version = version.Format(time.RFC3339)

			datasets = append(datasets, &dataset)
		}
		if err := rows.Err(); err != nil {
			return err
		}

		RenderData(c, datasets)
	}
	return nil
}
