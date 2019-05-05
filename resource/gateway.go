package resource

import (
	"github.com/yarf-framework/yarf"
)

// EnableMQTTGateway controls whether clients are informed about the MQTT gateway
var EnableMQTTGateway = true

// Gateway composites resource
// Gateways are the UnderLX's form of real-time communication between server and clients
// They are used to support communication paradigms that are poorly supported by conventional HTTP,
// such as publish-subscribe or unreliable connections (as in UDP)
type Gateway struct {
	resource
	mqttGateways []MQTTGatewayInfoProvider
}

// MQTTGatewayInfoProvider contains the methods the Gateway resource uses to provide information about a MQTTGateway
type MQTTGatewayInfoProvider interface {
	Hostname() string
	Port() uint16
	IsTLS() bool
	MQTTVersion() string
}

// apiGateway contains information about a gateway
type apiGateway struct {
	Protocol string `msgpack:"protocol" json:"protocol"` // only current valid value is "mqtt"
}

// apiGateway contains information about a MQTT gateway
type apiMQTTGateway struct {
	apiGateway  `msgpack:",inline"`
	Host        string `msgpack:"host" json:"host"`
	Port        uint16 `msgpack:"port" json:"port"`
	MqttVersion string `msgpack:"pVer" json:"pVer"`
	TLS         bool   `msgpack:"tls" json:"tls"`
}

// RegisterMQTTGateway associates another MQTTGateway with this resource
func (r *Gateway) RegisterMQTTGateway(mqtt MQTTGatewayInfoProvider) *Gateway {
	r.mqttGateways = append(r.mqttGateways, mqtt)
	return r
}

// Get serves HTTP GET requests on this resource
func (r *Gateway) Get(c *yarf.Context) error {
	data := []interface{}{}
	if EnableMQTTGateway {
		for _, g := range r.mqttGateways {
			data = append(data, apiMQTTGateway{
				apiGateway: apiGateway{
					Protocol: "mqtt",
				},
				Host:        g.Hostname(),
				Port:        g.Port(),
				MqttVersion: g.MQTTVersion(),
				TLS:         g.IsTLS(),
			})
		}
	}

	RenderData(c, data, "s-maxage=10")
	return nil
}
