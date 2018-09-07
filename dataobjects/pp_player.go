package dataobjects

import (
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/gbl08ma/sqalx"
	sq "github.com/gbl08ma/squirrel"
)

// PPPlayer is a PosPlay player
type PPPlayer struct {
	DiscordID uint64
	Joined    time.Time
	LBPrivacy string
	NameType  string
	InGuild   bool
}

// GetPPPlayers returns a slice with all registered players
func GetPPPlayers(node sqalx.Node) ([]*PPPlayer, error) {
	return getPPPlayersWithSelect(node, sdb.Select())
}

func getPPPlayersWithSelect(node sqalx.Node, sbuilder sq.SelectBuilder) ([]*PPPlayer, error) {
	players := []*PPPlayer{}

	tx, err := node.Beginx()
	if err != nil {
		return players, err
	}
	defer tx.Commit() // read-only tx

	rows, err := sbuilder.Columns("pp_player.discord_id", "pp_player.joined",
		"pp_player.lb_privacy", "pp_player.name_type", "pp_player.in_guild").
		From("pp_player").
		RunWith(tx).Query()
	if err != nil {
		return players, fmt.Errorf("getPPPlayersWithSelect: %s", err)
	}
	defer rows.Close()

	for rows.Next() {
		var player PPPlayer
		err := rows.Scan(
			&player.DiscordID,
			&player.Joined,
			&player.LBPrivacy,
			&player.NameType,
			&player.InGuild)
		if err != nil {
			return players, fmt.Errorf("getPPPlayersWithSelect: %s", err)
		}
		if err != nil {
			return players, fmt.Errorf("getPPPlayersWithSelect: %s", err)
		}
		players = append(players, &player)
	}
	if err := rows.Err(); err != nil {
		return players, fmt.Errorf("getPPPlayersWithSelect: %s", err)
	}
	return players, nil
}

// GetPPPlayer returns the player with the given Discord ID
func GetPPPlayer(node sqalx.Node, discordID uint64) (*PPPlayer, error) {
	if value, present := node.Load(getCacheKey("pp_player", fmt.Sprintf("%d", discordID))); present {
		return value.(*PPPlayer), nil
	}
	s := sdb.Select().
		Where(sq.Eq{"discord_id": discordID})
	players, err := getPPPlayersWithSelect(node, s)
	if err != nil {
		return nil, err
	}
	if len(players) == 0 {
		return nil, errors.New("PPPlayer not found")
	}
	node.Store(getCacheKey("pp_player", fmt.Sprintf("%d", discordID)), players[0])
	return players[0], nil
}

// XPTransactions returns a slice with all registered transactions for this player
func (player *PPPlayer) XPTransactions(node sqalx.Node) ([]*PPXPTransaction, error) {
	s := sdb.Select().
		Where(sq.Eq{"discord_id": player.DiscordID}).
		OrderBy("timestamp DESC")
	return getPPXPTransactionsWithSelect(node, s)
}

// XPTransactionsLimit returns a slice with `limit` most recent transactions for this player
func (player *PPPlayer) XPTransactionsLimit(node sqalx.Node, limit uint64) ([]*PPXPTransaction, error) {
	s := sdb.Select().
		Where(sq.Eq{"discord_id": player.DiscordID}).
		OrderBy("timestamp DESC").
		Limit(limit)
	return getPPXPTransactionsWithSelect(node, s)
}

// XPTransactionsBetween returns a slice with all registered transactions for this player within the specified interval
func (player *PPPlayer) XPTransactionsBetween(node sqalx.Node, start, end time.Time) ([]*PPXPTransaction, error) {
	s := sdb.Select().
		Where(sq.Eq{"discord_id": player.DiscordID}).
		Where(sq.Expr("timestamp BETWEEN ? AND ?",
			start, end)).
		OrderBy("timestamp DESC")
	return getPPXPTransactionsWithSelect(node, s)
}

// XPBalance returns the total XP for this player
func (player *PPPlayer) XPBalance(node sqalx.Node) (int, error) {
	tx, err := node.Beginx()
	if err != nil {
		return 0, err
	}
	defer tx.Commit() // read-only tx

	s := sdb.Select("SUM(value)").
		From("pp_xp_tx").
		Where(sq.Eq{"discord_id": player.DiscordID})

	var count int
	// this might error if sum returns null (no rows), no problem, just return 0
	s.RunWith(tx).Scan(&count)
	return count, nil
}

// XPBalanceBetween returns the total XP for this player within the specified time interval
func (player *PPPlayer) XPBalanceBetween(node sqalx.Node, start, end time.Time) (int, error) {
	tx, err := node.Beginx()
	if err != nil {
		return 0, err
	}
	defer tx.Commit() // read-only tx

	s := sdb.Select("SUM(value)").
		From("pp_xp_tx").
		Where(sq.Eq{"discord_id": player.DiscordID}).
		Where(sq.Expr("timestamp BETWEEN ? AND ?",
			start, end))

	var count int
	// this might error if sum returns null (no rows), no problem, just return 0
	s.RunWith(tx).Scan(&count)
	return count, nil
}

// Level returns the user's level, and a % indicating the progression to the next one
func (player *PPPlayer) Level(node sqalx.Node) (int, int, float64, error) {
	xp, err := player.XPBalance(node)
	if err != nil {
		return 0, 0, 0, err
	}
	// progression = (xp/c)^(1/b)
	// c = 17.2563531130954
	// b = 1.58138911016788, 1/b = 0.63235543584453
	progression := math.Pow(float64(xp)/17.2563531130954, 0.63235543584453)
	level, part := math.Modf(progression)
	return xp, int(level), part * 100, nil
}

// Update adds or updates the PPPlayer
func (player *PPPlayer) Update(node sqalx.Node) error {
	tx, err := node.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = sdb.Insert("pp_player").
		Columns("discord_id", "joined", "lb_privacy", "name_type", "in_guild").
		Values(player.DiscordID, player.Joined, player.LBPrivacy, player.NameType, player.InGuild).
		Suffix("ON CONFLICT (discord_id) DO UPDATE SET joined = ?, lb_privacy = ?, name_type = ?, in_guild = ?",
			player.Joined, player.LBPrivacy, player.NameType, player.InGuild).
		RunWith(tx).Exec()

	if err != nil {
		return errors.New("AddPPlayer: " + err.Error())
	}
	return tx.Commit()
}

// Delete deletes the PPPlayer
func (player *PPPlayer) Delete(node sqalx.Node) error {
	tx, err := node.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = sdb.Delete("pp_player").
		Where(sq.Eq{"discord_id": player.DiscordID}).RunWith(tx).Exec()
	if err != nil {
		return fmt.Errorf("RemovePPPlayer: %s", err)
	}
	tx.Delete(getCacheKey("pp_player", fmt.Sprintf("%d", player.DiscordID)))
	return tx.Commit()
}
