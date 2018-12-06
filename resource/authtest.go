package resource

import (
	"github.com/gbl08ma/sqalx"
	"github.com/yarf-framework/yarf"
)

// AuthTest composites resource
type AuthTest struct {
	resource
}

// WithNode associates a sqalx Node with this resource
func (r *AuthTest) WithNode(node sqalx.Node) *AuthTest {
	r.node = node
	return r
}

// WithHashKey associates a HMAC key with this resource so it can participate in authentication processes
func (r *AuthTest) WithHashKey(key []byte) *AuthTest {
	r.hashKey = key
	return r
}

// Get serves HTTP GET requests on this resource
func (r *AuthTest) Get(c *yarf.Context) error {
	pair, err := r.AuthenticateClient(c)
	if err != nil {
		RenderUnauthorized(c)
		return nil
	}

	RenderData(c, struct {
		Result string `msgpack:"result" json:"result"`
		Key    string `msgpack:"key" json:"key"`
	}{
		Result: "ok",
		Key:    pair.Key,
	}, "no-cache, no-store, must-revalidate")
	return nil
}
