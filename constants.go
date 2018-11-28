// +build !release

package main

const (
	// DEBUG is whether this is a debug build
	DEBUG = true

	// MLnetworkID is the ID of the main network
	MLnetworkID = "pt-ml"

	// SecretsPath is the default path to the file containing secrets
	SecretsPath = "secrets-debug.json"

	// DefaultClientCertPath is the default path to the certificate containing the public key of trusted API clients
	DefaultClientCertPath = "trusted_client_cert.pem"

	// MaxDBconnectionPoolSize is the maximum number of simultaneous database connections in the connection pool
	MaxDBconnectionPoolSize = 30
)
