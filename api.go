package main

import (
	"encoding/hex"
	"encoding/pem"
	"math/rand"
	"net/http"
	"time"

	"crypto/x509"
	"io/ioutil"

	"crypto/ecdsa"

	"github.com/underlx/disturbancesmlx/posplay"
	"github.com/underlx/disturbancesmlx/resource"
	"github.com/yarf-framework/yarf"
)

// Static defines a simple resource
type Static struct {
	yarf.Resource
	path  string // Directory to serve static files from.
	strip string // Part of path to strip from http file server handler
}

// WithPath associates the path and part of the path to strip with this resource
func (r *Static) WithPath(path string, strip string) *Static {
	r.path = path
	r.strip = strip
	return r
}

// Get implements the static files handler
func (r *Static) Get(c *yarf.Context) error {
	http.StripPrefix(r.strip, http.FileServer(http.Dir(r.path))).ServeHTTP(c.Response, c.Request)

	return nil
}

// APIserver sets up and starts the API server
func APIserver(trustedClientCertPath string) {
	resource.RegisterPairConnectionHandler(posplay.TheConnectionHandler)

	y := yarf.New()

	v1 := yarf.RouteGroup("/v1")

	v1.Add("/meta", new(resource.Meta).WithNode(rootSqalxNode))

	v1.Add("/meta/backers", new(resource.Backers))

	gateway := new(resource.Gateway)
	if mqttGateway != nil {
		gateway = gateway.RegisterMQTTGateway(mqttGateway)
	}
	v1.Add("/gateways", gateway)

	v1.Add("/maps", new(resource.Map).WithNode(rootSqalxNode))

	v1.Add("/networks", new(resource.Network).WithNode(rootSqalxNode))
	v1.Add("/networks/:id", new(resource.Network).WithNode(rootSqalxNode))

	v1.Add("/lines", new(resource.Line).WithNode(rootSqalxNode))
	v1.Add("/lines/:id", new(resource.Line).WithNode(rootSqalxNode)) // contains logic for when :id is "conditions" to handle /lines/conditions
	v1.Add("/lines/conditions/:id", new(resource.LineCondition).WithNode(rootSqalxNode))
	v1.Add("/lines/:lineid/conditions", new(resource.LineCondition).WithNode(rootSqalxNode))

	v1.Add("/stations", new(resource.Station).WithNode(rootSqalxNode))
	v1.Add("/stations/:id", new(resource.Station).WithNode(rootSqalxNode))
	v1.Add("/stations/:sid/lobbies", new(resource.Lobby).WithNode(rootSqalxNode))

	v1.Add("/lobbies", new(resource.Lobby).WithNode(rootSqalxNode))
	v1.Add("/lobbies/:id", new(resource.Lobby).WithNode(rootSqalxNode))

	v1.Add("/pois", new(resource.POI).WithNode(rootSqalxNode))
	v1.Add("/pois/:id", new(resource.POI).WithNode(rootSqalxNode))

	v1.Add("/connections", new(resource.Connection).WithNode(rootSqalxNode))
	v1.Add("/connections/:from/:to", new(resource.Connection).WithNode(rootSqalxNode))

	v1.Add("/transfers", new(resource.Transfer).WithNode(rootSqalxNode))
	v1.Add("/transfers/:station/:from/:to", new(resource.Transfer).WithNode(rootSqalxNode))

	v1.Add("/disturbances", new(resource.Disturbance).WithNode(rootSqalxNode))

	v1.Add("/disturbances/reports", new(resource.DisturbanceReport).
		WithNode(rootSqalxNode).
		WithHashKey(getHashKey()).
		WithReportHandler(reportHandler))

	v1.Add("/disturbances/:id", new(resource.Disturbance).WithNode(rootSqalxNode))

	v1.Add("/datasets", new(resource.Dataset).WithNode(rootSqalxNode).WithSquirrel(&sdb))
	v1.Add("/datasets/:id", new(resource.Dataset).WithNode(rootSqalxNode).WithSquirrel(&sdb))

	v1.Add("/stats", new(resource.Stats).WithNode(rootSqalxNode).WithStats(statsHandler))
	v1.Add("/stats/:id", new(resource.Stats).WithNode(rootSqalxNode).WithStats(statsHandler))

	v1.Add("/announcements", new(resource.Announcement).WithAnnouncementStore(&annStore))
	v1.Add("/announcements/:source", new(resource.Announcement).WithAnnouncementStore(&annStore))

	v1.Add("/stationkb/*", new(Static).WithPath("stationkb/", "/v1/stationkb/"))
	v1.Add("/mapassets/*", new(Static).WithPath("mapassets/", "/v1/mapassets/"))

	v1.Add("/trips", new(resource.Trip).WithNode(rootSqalxNode).WithHashKey(getHashKey()))
	v1.Add("/trips/:id", new(resource.Trip).WithNode(rootSqalxNode).WithHashKey(getHashKey()))

	v1.Add("/rt", new(resource.Realtime).
		WithNode(rootSqalxNode).
		WithHashKey(getHashKey()).
		WithStatsHandler(statsHandler).
		WithVehicleHandler(vehicleHandler))

	v1.Add("/feedback", new(resource.Feedback).WithNode(rootSqalxNode).WithHashKey(getHashKey()))

	pubkey := getTrustedClientPublicKey(trustedClientCertPath)

	v1.Add("/pair", new(resource.Pair).
		WithNode(rootSqalxNode).
		WithPublicKey(pubkey).
		WithHashKey(getHashKey()))

	v1.Add("/pair/connections", new(resource.PairConnection).
		WithNode(rootSqalxNode).
		WithHashKey(getHashKey()))

	v1.Add("/authtest", new(resource.AuthTest).
		WithNode(rootSqalxNode).
		WithHashKey(getHashKey()))

	y.AddGroup(v1)
	if DEBUG {
		y.Insert(new(DelayMiddleware))
	}

	y.Insert(new(TelemetryMiddleware))

	y.Logger = webLog
	y.Start(":12000")
}

// DelayMiddleware injects a semi-random delay before each request is processed, for debugging
type DelayMiddleware struct {
	yarf.Middleware
}

// PreDispatch runs before the request is dispatched
func (m *DelayMiddleware) PreDispatch(c *yarf.Context) error {
	time.Sleep(500*time.Millisecond + time.Duration(rand.Intn(500))*time.Millisecond)
	return nil
}

// TelemetryMiddleware collects statistics about API usage
type TelemetryMiddleware struct {
	yarf.Middleware
}

// PostDispatch runs after the request is dispatched
func (m *TelemetryMiddleware) PostDispatch(c *yarf.Context) error {
	apiTotalRequests++
	// non-blocking send
	select {
	case APIrequestTelemetry <- true:
	default:
	}

	return nil
}

func getTrustedClientPublicKey(trustedClientCertPath string) *ecdsa.PublicKey {
	certBytes, err := ioutil.ReadFile(trustedClientCertPath)
	if err != nil {
		panic("Error reading trusted client certificate")
	}
	block, _ := pem.Decode([]byte(certBytes))
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		panic("Error parsing client certificate: " + err.Error())
	}

	return cert.PublicKey.(*ecdsa.PublicKey)
}

func getHashKey() []byte {
	hexkey, present := secrets.Get("secretHMACkey")
	if !present {
		mainLog.Fatal("API secret HMAC key not present in keybox")
	}
	key, err := hex.DecodeString(hexkey)
	if err != nil {
		mainLog.Fatal("Invalid API secret HMAC key specified")
	}
	return key
}
