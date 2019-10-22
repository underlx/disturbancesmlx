package types

import (
	"github.com/dchest/uniuri"
)

// GenerateAPIKey returns a securely randomly generated API key
func GenerateAPIKey() string {
	return uniuri.NewLenChars(16, []byte("0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"))
}

// GenerateAPISecret returns a securely randomly generated API secret
func GenerateAPISecret() string {
	return uniuri.NewLenChars(24, []byte("0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz!#$%&()*+-./<=>?@{|}~"))
}
