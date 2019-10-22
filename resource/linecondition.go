package resource

import (
	"time"

	"github.com/gbl08ma/sqalx"
	"github.com/underlx/disturbancesmlx/types"
	"github.com/yarf-framework/yarf"
)

// LineCondition composites resource
type LineCondition struct {
	resource
}

type apiLineCondition struct {
	ID             string               `msgpack:"id" json:"id"`
	Time           time.Time            `msgpack:"time" json:"time"`
	Line           *types.Line    `msgpack:"-" json:"-"`
	TrainCars      int                  `msgpack:"trainCars" json:"trainCars"`
	TrainFrequency types.Duration `msgpack:"trainFrequency" json:"trainFrequency"`
	Source         *types.Source  `msgpack:"-" json:"-"`
}

type apiLineConditionWrapper struct {
	apiLineCondition `msgpack:",inline"`
	LineID           string `msgpack:"line" json:"line"`
	SourceID         string `msgpack:"source" json:"source"`
}

// WithNode associates a sqalx Node with this resource
func (r *LineCondition) WithNode(node sqalx.Node) *LineCondition {
	r.node = node
	return r
}

// Get serves HTTP GET requests on this resource
func (r *LineCondition) Get(c *yarf.Context) error {
	tx, err := r.Beginx()
	if err != nil {
		return err
	}
	defer tx.Commit() // read-only tx

	var lineconditions []*types.LineCondition
	cacheControl := "s-maxage=10"
	latestOnly := c.Param("id") == "" && c.Request.URL.Query().Get("filter") == "latest"
	if latestOnly {
		cacheControl = "no-cache, no-store, must-revalidate"
	}
	if c.Param("lineid") != "" {
		line, err := types.GetLine(tx, c.Param("lineid"))
		if err != nil {
			return err
		}
		if latestOnly {
			var condition *types.LineCondition
			condition, err = line.LastCondition(tx)
			lineconditions = []*types.LineCondition{condition}
		} else {
			lineconditions, err = line.Conditions(tx)
		}
		if err != nil {
			return err
		}
	} else if c.Param("id") != "" {
		condition, err := types.GetLineCondition(tx, c.Param("id"))
		if err != nil {
			return err
		}
		lineconditions = []*types.LineCondition{condition}
	} else if latestOnly {
		// all latest conditions for each line
		lines, err := types.GetLines(tx)
		if err != nil {
			return err
		}
		lineconditions = []*types.LineCondition{}
		for _, line := range lines {
			condition, err := line.LastCondition(tx)
			if err != nil {
				continue
			}
			lineconditions = append(lineconditions, condition)
		}
	} else {
		lineconditions, err = types.GetLineConditions(tx)
		if err != nil {
			return err
		}
	}

	apilineconditions := make([]apiLineConditionWrapper, len(lineconditions))
	for i := range lineconditions {
		apilineconditions[i] = apiLineConditionWrapper{
			apiLineCondition: apiLineCondition(*lineconditions[i]),
			LineID:           lineconditions[i].Line.ID,
			SourceID:         lineconditions[i].Source.ID,
		}
	}

	if c.Param("lineid") != "" {
		RenderData(c, apilineconditions, cacheControl)
	} else if c.Param("id") != "" {
		RenderData(c, apilineconditions[0], cacheControl)
	} else {
		RenderData(c, apilineconditions, cacheControl)
	}
	return nil
}
