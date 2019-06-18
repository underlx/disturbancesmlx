package resource

import (
	"github.com/gbl08ma/sqalx"
	"github.com/yarf-framework/yarf"
)

// Map composites resource
type Map struct {
	resource
}

// apiMap contains information about a network diagram/map
type apiMap struct {
	Type string `msgpack:"type" json:"type"`
}

type apiHTMLMap struct {
	apiMap       `msgpack:",inline"`
	URL          string `msgpack:"url" json:"url"`
	Cache        bool   `msgpack:"cache" json:"cache"`
	WideViewport bool   `msgpack:"wideViewport" json:"wideViewport"`
}

// WithNode associates a sqalx Node with this resource
func (r *Map) WithNode(node sqalx.Node) *Map {
	r.node = node
	return r
}

// Get serves HTTP GET requests on this resource
func (r *Map) Get(c *yarf.Context) error {
	RenderData(c, []interface{}{
		apiMap{
			Type: "world-map",
		},
		apiHTMLMap{
			apiMap: apiMap{
				Type: "html",
			},
			URL:          "mapassets/map-pt-ml.html",
			Cache:        true,
			WideViewport: true,
		},
		apiHTMLMap{
			apiMap: apiMap{
				Type: "html",
			},
			URL:   "mapassets/map-pt-ml-portrait.html",
			Cache: true,
		},
	}, "s-maxage=10")
	return nil
}
