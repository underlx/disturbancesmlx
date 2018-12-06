package resource

import (
	"github.com/gbl08ma/sqalx"
	"github.com/underlx/disturbancesmlx/dataobjects"
	"github.com/yarf-framework/yarf"

	sq "github.com/gbl08ma/squirrel"
	"github.com/lib/pq"
)

// Dataset composites resource
type Dataset struct {
	resource
	sdb *sq.StatementBuilderType
}

type apiDatasetWrapper struct {
	Version string               `msgpack:"version" json:"version"`
	Authors pq.StringArray       `msgpack:"authors" json:"authors"`
	Network *dataobjects.Network `msgpack:"-" json:"-"`
}

type apiDataset struct {
	NetworkID         string `msgpack:"network" json:"network"`
	apiDatasetWrapper `msgpack:",inline"`
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
		dataset, err := dataobjects.GetDataset(tx, c.Param("id"))
		if err != nil {
			return err
		}

		apidataset := apiDataset{
			apiDatasetWrapper: apiDatasetWrapper(*dataset),
			NetworkID:         dataset.Network.ID,
		}

		RenderData(c, apidataset, "s-maxage=10")
	} else {
		datasets, err := dataobjects.GetDatasets(tx)
		if err != nil {
			return err
		}
		apidatasets := make([]apiDataset, len(datasets))
		for i := range datasets {
			apidatasets[i] = apiDataset{
				apiDatasetWrapper: apiDatasetWrapper(*datasets[i]),
				NetworkID:         datasets[i].Network.ID,
			}
		}

		RenderData(c, apidatasets, "s-maxage=10")
	}
	return nil
}
