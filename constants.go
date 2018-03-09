// +build !release

package main

const (
	DEBUG                   = true
	MLnetworkID             = "pt-ml"
	SecretsPath             = "secrets-debug.json"
	DefaultClientCertPath   = "trusted_client_cert.pem"
	MaxDBconnectionPoolSize = 30
)
