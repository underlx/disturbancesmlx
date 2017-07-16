package resource

import (
	"github.com/heetch/sqalx"
	"github.com/yarf-framework/yarf"
)

// AuthTest composites resource
type AuthTest struct {
	resource
}

func (r *AuthTest) WithNode(node sqalx.Node) *AuthTest {
	r.node = node
	return r
}

func (r *AuthTest) WithHashKey(key []byte) *AuthTest {
	r.hashKey = key
	return r
}

func (n *AuthTest) Get(c *yarf.Context) error {
	key, err := n.AuthenticateClient(c)
	if err != nil {
		RenderUnauthorized(c)
		return nil
	}

	RenderData(c, struct {
		Result string `msgpack:"result" json:"result"`
		Key    string `msgpack:"key" json:"key"`
	}{
		Result: "ok",
		Key:    key,
	})
	return nil
}
