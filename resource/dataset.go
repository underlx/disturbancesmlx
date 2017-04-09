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
	Version   time.Time      `msgpack:"version" json:"version"`
	Authors   pq.StringArray `msgpack:"authors" json:"authors"`
}

func (r *Dataset) WithNode(node sqalx.Node) *Dataset {
	r.node = node
	return r
}

func (r *Dataset) WithSquirrel(sdb *sq.StatementBuilderType) *Dataset {
	r.sdb = sdb
	return r
}

func (n *Dataset) Get(c *yarf.Context) error {
	tx, err := n.Beginx()
	if err != nil {
		return err
	}
	defer tx.Commit() // read-only tx

	if c.Param("id") != "" {
		var dataset apiDataset
		err := n.sdb.Select("network_id", "version", "authors").
			From("dataset_info").RunWith(tx).QueryRow().Scan(
			&dataset.NetworkID,
			&dataset.Version,
			&dataset.Authors)
		if err != nil {
			return err
		}

		RenderData(c, dataset)
	} else {
		datasets := []*apiDataset{}
		rows, err := n.sdb.Select("network_id", "version", "authors").
			From("dataset_info").RunWith(tx).Query()
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var dataset apiDataset
			err := rows.Scan(
				&dataset.NetworkID,
				&dataset.Version,
				&dataset.Authors)
			if err != nil {
				return err
			}
			datasets = append(datasets, &dataset)
		}
		if err := rows.Err(); err != nil {
			return err
		}

		RenderData(c, datasets)
	}
	return nil
}
