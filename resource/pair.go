package resource

import (
	"crypto/sha256"
	"encoding/asn1"
	"encoding/base64"
	"math/big"
	"net/http"
	"time"

	"crypto/ecdsa"

	"net"

	"github.com/gbl08ma/sqalx"
	"github.com/underlx/disturbancesmlx/types"
	"github.com/yarf-framework/yarf"
)

// Pair composites resource
type Pair struct {
	resource
	trustedClientPublicKey *ecdsa.PublicKey
	hashKey                []byte
	telemetryChannel       chan bool
}

type apiPairRequest struct {
	// Nonce must be 36 characters long
	// A v4 UUID can be used, but a random string is fine as well
	Nonce string `msgpack:"nonce" json:"nonce"`
	// Timestamp is a string, because it is used to compute the signature
	// Thus, it's important that it doesn't change depending on the envelope (JSON or msgpack)
	// The timestamp should be encoded in RFC3339 format
	Timestamp string `msgpack:"timestamp" json:"timestamp"`
	AndroidID string `msgpack:"androidID" json:"androidID"`
	Signature string `msgpack:"signature" json:"signature"`
}

// apiPair contains the response to the pair creation request
type apiPair struct {
	Key        string    `msgpack:"key" json:"key"`
	Secret     string    `msgpack:"secret" json:"secret"`
	Type       string    `msgpack:"type" json:"type"`
	Activation time.Time `msgpack:"activation" json:"activation"`
}

type ecdsaSignature struct {
	R, S *big.Int
}

const maxTimestampSkew = 30 * time.Minute

// WithNode associates a sqalx Node with this resource
func (r *Pair) WithNode(node sqalx.Node) *Pair {
	r.node = node
	return r
}

// WithPublicKey associates a trusted client public key with this resource
func (r *Pair) WithPublicKey(key *ecdsa.PublicKey) *Pair {
	r.trustedClientPublicKey = key
	return r
}

// WithHashKey associates a HMAC key with this resource so it can participate in authentication processes
func (r *Pair) WithHashKey(key []byte) *Pair {
	r.hashKey = key
	return r
}

// WithTelemetryChannel associates a telemetry channel with this resource
func (r *Pair) WithTelemetryChannel(c chan bool) *Pair {
	r.telemetryChannel = c
	return r
}

func (r *Pair) sendTelemetry(success bool) {
	// non-blocking send
	select {
	case r.telemetryChannel <- success:
	default:
	}
}

// Post serves HTTP POST requests on this resource
func (r *Pair) Post(c *yarf.Context) (err error) {
	defer func() {
		r.sendTelemetry(err == nil)
	}()
	tx, err := r.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var pairRequest apiPairRequest
	err = r.DecodeRequest(c, &pairRequest)
	if err != nil {
		return err
	}

	if len(pairRequest.Nonce) != 36 {
		return &yarf.CustomError{
			HTTPCode:  http.StatusBadRequest,
			ErrorMsg:  "Nonce does not meet the length requirements",
			ErrorBody: "Nonce does not meet the length requirements",
		}
	}

	timestamp, err := time.Parse(time.RFC3339, pairRequest.Timestamp)
	if err != nil {
		return &yarf.CustomError{
			HTTPCode:  http.StatusBadRequest,
			ErrorMsg:  "Failed to parse timestamp",
			ErrorBody: err.Error(),
		}
	}
	diff := time.Now().UTC().Sub(timestamp)
	diff = maxDuration(diff, -diff)
	if diff > maxTimestampSkew {
		return &yarf.CustomError{
			HTTPCode:  http.StatusBadRequest,
			ErrorMsg:  "Timestamp too far from current time",
			ErrorBody: "Timestamp too far from current time",
		}
	}

	if len(pairRequest.AndroidID) > 16 {
		return &yarf.CustomError{
			HTTPCode:  http.StatusBadRequest,
			ErrorMsg:  "Android ID does not meet the length requirements",
			ErrorBody: "Android ID does not meet the length requirements",
		}
	}

	// the "fun" part: verify the signature
	// start by decoding the signature into something the crypto package can work with
	signDec, err := base64.StdEncoding.DecodeString(pairRequest.Signature)
	if err != nil {
		return &yarf.CustomError{
			HTTPCode:  http.StatusBadRequest,
			ErrorMsg:  "Bad signature encoding",
			ErrorBody: err.Error(),
		}
	}
	var signature ecdsaSignature
	_, err = asn1.Unmarshal(signDec, &signature)
	if err != nil {
		return &yarf.CustomError{
			HTTPCode:  http.StatusBadRequest,
			ErrorMsg:  "Bad signature",
			ErrorBody: err.Error(),
		}
	}

	hashedContent := pairRequest.Nonce + pairRequest.Timestamp + pairRequest.AndroidID
	hash := sha256.Sum256([]byte(hashedContent))

	if !ecdsa.Verify(r.trustedClientPublicKey, hash[:], signature.R, signature.S) {
		return &yarf.CustomError{
			HTTPCode:  http.StatusBadRequest,
			ErrorMsg:  "Bad signature",
			ErrorBody: err.Error(),
		}
	}

	// signature ok

	ipAddr := net.ParseIP(c.GetClientIP())
	pReq := types.NewAndroidPairRequest(pairRequest.Nonce, pairRequest.AndroidID, ipAddr)

	activation, err := pReq.CalculateActivationTime(tx, maxTimestampSkew)

	if err != nil {
		return &yarf.CustomError{
			HTTPCode:  http.StatusBadRequest,
			ErrorMsg:  "Activation failed",
			ErrorBody: err.Error(),
		}
	}

	if activation.IsZero() {
		return &yarf.CustomError{
			HTTPCode:  http.StatusForbidden,
			ErrorMsg:  "Activation failed",
			ErrorBody: err.Error(),
		}
	}

	err = pReq.Store(tx)
	if err != nil {
		return err
	}

	pair, err := types.NewPair(tx, "android", activation, r.hashKey)
	if err != nil {
		return err
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	RenderData(c, apiPair{
		Key:        pair.Key,
		Secret:     pair.Secret,
		Type:       pair.Type,
		Activation: pair.Activation,
	}, "no-cache, no-store, must-revalidate")
	return nil
}
