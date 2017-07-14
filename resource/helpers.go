package resource

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	msgpack "gopkg.in/vmihailenco/msgpack.v2"

	"log"

	"github.com/heetch/sqalx"
	"github.com/yarf-framework/yarf"
)

type resource struct {
	yarf.Resource
	node sqalx.Node
}

// Beginx is shorthand for resource.node.Beginx()
func (r *resource) Beginx() (sqalx.Node, error) {
	return r.node.Beginx()
}

func (r *resource) DecodeRequest(c *yarf.Context, v interface{}) error {
	contentType := c.Request.Header.Get("Content-Type")
	var err error
	switch {
	case strings.Contains(contentType, "msgpack"):
		err = msgpack.NewDecoder(c.Request.Body).Decode(v)
	case strings.Contains(contentType, "json"):
	default:
		err = json.NewDecoder(c.Request.Body).Decode(v)
	}

	if err != nil {
		return &yarf.CustomError{
			HTTPCode:  http.StatusBadRequest,
			ErrorMsg:  "Failed to decode request",
			ErrorBody: err.Error(),
		}
	}
	return nil

}

// RenderData takes a interface{} object and writes the encoded representation of it.
// Encoding used will be idented JSON, non-idented JSON, Msgpack or XML
func RenderData(c *yarf.Context, data interface{}) {
	accept := c.Request.Header.Get("Accept")
	switch {
	case strings.Contains(accept, "json"):
		c.Response.Header().Set("Content-Type", "application/json; charset=utf-8")
		c.RenderJSON(data)
	case strings.Contains(accept, "xml") && !strings.Contains(accept, "xhtml"):
		c.Response.Header().Set("Content-Type", "application/xml; charset=utf-8")
		c.RenderXML(data)
	case strings.Contains(accept, "msgpack"):
		RenderMsgpack(c, data)
	default:
		c.Response.Header().Set("Content-Type", "application/json; charset=utf-8")
		c.RenderJSONIndent(data)
	}
}

// RenderMsgpack takes a interface{} object and writes the Msgpack encoded string of it.
func RenderMsgpack(c *yarf.Context, data interface{}) {
	c.Response.Header().Set("Content-Type", "application/msgpack")
	// Set content
	encoded, err := msgpack.Marshal(data)
	if err != nil {
		log.Println(err)
		c.Response.Write([]byte(err.Error()))
	} else {
		c.Response.Write(encoded)
	}
}

func maxDuration(d1 time.Duration, d2 time.Duration) time.Duration {
	if d1 > d2 {
		return d1
	}
	return d2
}
