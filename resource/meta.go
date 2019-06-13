package resource

import (
	"github.com/gbl08ma/sqalx"
	"github.com/yarf-framework/yarf"
)

// Meta composites resource
type Meta struct {
	resource
}

// apiMeta contains information about this API endpoint
type apiMeta struct {
	// Whether this API is still supported
	Supported bool `msgpack:"supported" json:"supported"`

	// Whether this endpoint is up (it would be "down" for example in the event of server maintenance)
	Up bool `msgpack:"up" json:"up"`

	// The minimum build number of the Android client that is guaranteed to still be compatible with this endpoint
	MinAndroidClient int `msgpack:"minAndroidClient" json:"minAndroidClient"`

	// A "message of the day" to display to the user, with more or less prominency (lower priority = more prominency)
	MOTD apiMOTD `msgpack:"motd" json:"motd"`
}

// MOTD is the "message of the day" that is served to API clients
var MOTD apiMOTD

func init() {
	MOTD.HTML = make(map[string]string)
}

// apiMOTD contains a "message of the day"
type apiMOTD struct {
	HTML       map[string]string `msgpack:"html" json:"html"`
	MainLocale string            `msgpack:"mainLocale" json:"mainLocale"`
	Priority   int               `msgpack:"priority" json:"priority"`
}

// WithNode associates a sqalx Node with this resource
func (r *Meta) WithNode(node sqalx.Node) *Meta {
	r.node = node
	return r
}

// Get serves HTTP GET requests on this resource
func (r *Meta) Get(c *yarf.Context) error {
	RenderData(c, apiMeta{
		Supported:        true,
		Up:               true,
		MinAndroidClient: 1,
		MOTD:             MOTD,
	}, "s-maxage=10")
	return nil
}
