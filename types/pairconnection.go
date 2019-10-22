package types

import "time"

// PairConnection represents a APIPair connection with an external service
type PairConnection interface {
	Pair() *APIPair
	Created() time.Time
	Extra() interface{}
}
