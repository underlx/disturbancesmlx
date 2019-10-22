package posplay

import (
	"time"

	"github.com/underlx/disturbancesmlx/types"
)

// PairConnection implements types.PairConnection
type PairConnection struct {
	pair    *types.APIPair
	created time.Time
	extra   PairConnectionExtra
}

// PairConnectionExtra is the extra info bundle for PairConnections. This is serialized and sent to clients
type PairConnectionExtra struct {
	DiscordID     uint64  `msgpack:"discordID" json:"discordID"`
	Username      string  `msgpack:"username" json:"username"`
	AvatarURL     string  `msgpack:"avatarURL" json:"avatarURL"`
	Level         int     `msgpack:"level" json:"level"`
	LevelProgress float64 `msgpack:"levelProgress" json:"levelProgress"`
	XP            int     `msgpack:"xp" json:"xp"`
	XPthisWeek    int     `msgpack:"xpThisWeek" json:"xpThisWeek"`
	Rank          int     `msgpack:"rank" json:"rank"`
	RankThisWeek  int     `msgpack:"rankThisWeek" json:"rankThisWeek"`
}

// Pair returns the pair of this connection
func (c *PairConnection) Pair() *types.APIPair {
	return c.pair
}

// Created returns the creation time of this connection
func (c *PairConnection) Created() time.Time {
	return c.created
}

// Extra returns the extra bundle for this connection
func (c *PairConnection) Extra() interface{} {
	return c.extra
}
