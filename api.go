package main

import (
	"encoding/hex"
	"encoding/pem"
	"net/http"

	"crypto/x509"
	"io/ioutil"

	"crypto/ecdsa"

	"github.com/gbl08ma/disturbancesmlx/resource"
	"github.com/yarf-framework/yarf"
)

// Static defines a simple resource
type Static struct {
	yarf.Resource
	path  string // Directory to serve static files from.
	strip string // Part of path to strip from http file server handler
}

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

func APIserver(trustedClientCertPath string) {
	y := yarf.New()

	v1 := yarf.RouteGroup("/v1")

	v1.Add("/networks", new(resource.Network).WithNode(rootSqalxNode))
	v1.Add("/networks/:id", new(resource.Network).WithNode(rootSqalxNode))

	v1.Add("/lines", new(resource.Line).WithNode(rootSqalxNode))
	v1.Add("/lines/:id", new(resource.Line).WithNode(rootSqalxNode))

	v1.Add("/stations", new(resource.Station).WithNode(rootSqalxNode))
	v1.Add("/stations/:id", new(resource.Station).WithNode(rootSqalxNode))
	v1.Add("/stations/:sid/lobbies", new(resource.Lobby).WithNode(rootSqalxNode))

	v1.Add("/lobbies", new(resource.Lobby).WithNode(rootSqalxNode))
	v1.Add("/lobbies/:id", new(resource.Lobby).WithNode(rootSqalxNode))

	v1.Add("/connections", new(resource.Connection).WithNode(rootSqalxNode))
	v1.Add("/connections/:from/:to", new(resource.Connection).WithNode(rootSqalxNode))

	v1.Add("/transfers", new(resource.Transfer).WithNode(rootSqalxNode))
	v1.Add("/transfers/:station/:from/:to", new(resource.Transfer).WithNode(rootSqalxNode))

	v1.Add("/disturbances", new(resource.Disturbance).WithNode(rootSqalxNode))
	v1.Add("/disturbances/:id", new(resource.Disturbance).WithNode(rootSqalxNode))

	v1.Add("/datasets", new(resource.Dataset).WithNode(rootSqalxNode).WithSquirrel(&sdb))
	v1.Add("/datasets/:id", new(resource.Dataset).WithNode(rootSqalxNode).WithSquirrel(&sdb))

	v1.Add("/stationkb/*", new(Static).WithPath("stationkb/", "/v1/stationkb/"))

	pubkey := getTrustedClientPublicKey(trustedClientCertPath)

	v1.Add("/pair", new(resource.Pair).
		WithNode(rootSqalxNode).
		WithPublicKey(pubkey).
		WithHashKey(getHashKey()))

	if DEBUG {
		v1.Add("/authtest", new(resource.AuthTest).
			WithNode(rootSqalxNode).
			WithHashKey(getHashKey()))
	}

	y.AddGroup(v1)

	y.Logger = webLog
	y.Start(":12000")
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
