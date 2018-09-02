package dataobjects

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	sq "github.com/gbl08ma/squirrel"
	"github.com/gbl08ma/sqalx"
)

// APIPair contains API auth credentials
type APIPair struct {
	Key string
	// Secret contains the plaintext secret. Only used when the pair is newly created
	Secret string
	// SecretHash contains the SHA256-HMAC hash of the secret
	SecretHash string
	Type       string
	Activation time.Time
}

// GetPair returns the API pair with the given ID
func GetPair(node sqalx.Node, key string) (*APIPair, error) {
	var pair APIPair
	tx, err := node.Beginx()
	if err != nil {
		return &pair, err
	}
	defer tx.Commit() // read-only tx

	err = sdb.Select("key", "secret", "type", "activation").
		From("api_pair").
		Where(sq.Eq{"key": key}).
		RunWith(tx).QueryRow().Scan(&pair.Key, &pair.SecretHash, &pair.Type, &pair.Activation)
	if err != nil {
		return &pair, errors.New("GetPair: " + err.Error())
	}
	return &pair, nil
}

// NewPair creates a new API access pair, stores it in the DB and returns it
func NewPair(node sqalx.Node, pairtype string, activation time.Time, hashKey []byte) (pair *APIPair, err error) {
	tx, err := node.Beginx()
	if err != nil {
		return &APIPair{}, err
	}
	defer tx.Rollback()

	pair = &APIPair{
		Type:       pairtype,
		Activation: activation,
	}

	pair.Key, err = GenerateAPIKey()
	if err != nil {
		return &APIPair{}, errors.New("NewAPIPair: " + err.Error())
	}
	pair.Secret, err = GenerateAPISecret()
	if err != nil {
		return &APIPair{}, errors.New("NewAPIPair: " + err.Error())
	}
	pair.SecretHash = ComputeAPISecretHash(pair.Secret, hashKey)

	_, err = sdb.Insert("api_pair").
		Columns("key", "secret", "type", "activation").
		Values(pair.Key, pair.SecretHash, pair.Type, pair.Activation).
		RunWith(tx).Exec()

	if err != nil {
		return &APIPair{}, errors.New("NewAPIPair: " + err.Error())
	}
	err = tx.Commit()
	if err != nil {
		return &APIPair{}, errors.New("NewAPIPair: " + err.Error())
	}

	return pair, nil
}

// GetPairIfCorrect returns the pair and no errors if the given secret is correct for this
// API key, and the pair is ready to be used. Otherwise, a nil pointer and a error is returned.
func GetPairIfCorrect(node sqalx.Node, key string, givenSecret string, hashKey []byte) (*APIPair, error) {
	pair, err := GetPair(node, key)
	if err != nil {
		return nil, err
	}
	if !pair.Activated() {
		return nil, errors.New("Pair is not activated")
	}
	if err = pair.CheckSecret(givenSecret, hashKey); err != nil {
		return nil, err
	}
	return pair, nil
}

// CheckSecret returns no errors if the given secret is correct for this API pair
func (pair *APIPair) CheckSecret(givenSecret string, hashKey []byte) (err error) {
	if pair.SecretHash != ComputeAPISecretHash(givenSecret, hashKey) {
		return errors.New("CheckSecret: the given secret does not match with the pair secret")
	}
	return nil
}

// Activated returns whether this pair is activated
func (pair *APIPair) Activated() bool {
	return time.Now().UTC().After(pair.Activation)
}

// Delete deletes the pair
func (pair *APIPair) Delete(node sqalx.Node) error {
	tx, err := node.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = sdb.Delete("api_pair").
		Where(sq.Eq{"key": pair.Key}).RunWith(tx).Exec()
	if err != nil {
		return fmt.Errorf("RemoveAPIPair: %s", err)
	}
	return tx.Commit()
}

// ComputeAPISecretHash calculates the hash for the specified secret
// with SHA256 HMAC using the specified key
func ComputeAPISecretHash(secret string, key []byte) string {
	h := hmac.New(sha256.New, key)
	h.Write([]byte(secret))
	return hex.EncodeToString(h.Sum(nil))
}
