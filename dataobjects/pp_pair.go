package dataobjects

import (
	"errors"
	"fmt"
	"time"

	"github.com/gbl08ma/sqalx"
	sq "github.com/gbl08ma/squirrel"
)

// PPPair is a PosPlay pair
type PPPair struct {
	DiscordID  uint64
	Pair       *APIPair
	Paired     time.Time
	DeviceName string
}

// GetPPPairs returns a slice with all registered pairs
func GetPPPairs(node sqalx.Node) ([]*PPPair, error) {
	return getPPPairsWithSelect(node, sdb.Select())
}

func getPPPairsWithSelect(node sqalx.Node, sbuilder sq.SelectBuilder) ([]*PPPair, error) {
	pairs := []*PPPair{}

	tx, err := node.Beginx()
	if err != nil {
		return pairs, err
	}
	defer tx.Commit() // read-only tx

	rows, err := sbuilder.Columns("pp_pair.discord_id", "pp_pair.api_key",
		"pp_pair.paired", "pp_pair.device_name").
		From("pp_pair").
		RunWith(tx).Query()
	if err != nil {
		return pairs, fmt.Errorf("getPPPairsWithSelect: %s", err)
	}

	apiPairs := []string{}
	for rows.Next() {
		var pair PPPair
		var apiPair string
		err := rows.Scan(
			&pair.DiscordID,
			&apiPair,
			&pair.Paired,
			&pair.DeviceName)
		if err != nil {
			rows.Close()
			return pairs, fmt.Errorf("getPPPairsWithSelect: %s", err)
		}
		if err != nil {
			rows.Close()
			return pairs, fmt.Errorf("getPPPairsWithSelect: %s", err)
		}
		apiPairs = append(apiPairs, apiPair)
		pairs = append(pairs, &pair)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return pairs, fmt.Errorf("getPPPairsWithSelect: %s", err)
	}
	rows.Close()
	for i := range pairs {
		pairs[i].Pair, err = GetPair(tx, apiPairs[i])
		if err != nil {
			return pairs, fmt.Errorf("getPPPairsWithSelect: %s", err)
		}
	}
	return pairs, nil
}

// GetPPPair returns the pair with the given Discord ID
func GetPPPair(node sqalx.Node, discordID uint64) (*PPPair, error) {
	if value, present := node.Load(getCacheKey("pp_pair", fmt.Sprintf("%d", discordID))); present {
		return value.(*PPPair), nil
	}
	s := sdb.Select().
		Where(sq.Eq{"discord_id": discordID})
	pairs, err := getPPPairsWithSelect(node, s)
	if err != nil {
		return nil, err
	}
	if len(pairs) == 0 {
		return nil, errors.New("PPPair not found")
	}
	node.Store(getCacheKey("pp_pair", fmt.Sprintf("%d", discordID)), pairs[0])
	return pairs[0], nil
}

// GetPPPairForKey returns the pair with the given Discord ID
func GetPPPairForKey(node sqalx.Node, apiKey string) (*PPPair, error) {
	if value, present := node.Load(getCacheKey("pp_pair_key", apiKey)); present {
		return value.(*PPPair), nil
	}
	s := sdb.Select().
		Where(sq.Eq{"api_key": apiKey})
	pairs, err := getPPPairsWithSelect(node, s)
	if err != nil {
		return nil, err
	}
	if len(pairs) == 0 {
		return nil, errors.New("PPPair not found")
	}
	node.Store(getCacheKey("pp_pair_key", apiKey), pairs[0])
	return pairs[0], nil
}

// Update adds or updates the PPPair
func (pair *PPPair) Update(node sqalx.Node) error {
	tx, err := node.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = sdb.Insert("pp_pair").
		Columns("discord_id", "api_key", "paired", "device_name").
		Values(pair.DiscordID, pair.Pair.Key, pair.Paired, pair.DeviceName).
		Suffix("ON CONFLICT (discord_id) DO UPDATE SET api_key = ?, paired = ?, device_name = ?",
			pair.Pair.Key, pair.Paired, pair.DeviceName).
		RunWith(tx).Exec()

	if err != nil {
		return errors.New("AddPPair: " + err.Error())
	}
	return tx.Commit()
}

// Delete deletes the PPPair
func (pair *PPPair) Delete(node sqalx.Node) error {
	tx, err := node.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = sdb.Delete("pp_pair").
		Where(sq.Eq{"discord_id": pair.DiscordID}).RunWith(tx).Exec()
	if err != nil {
		return fmt.Errorf("RemovePPPair: %s", err)
	}
	tx.Delete(getCacheKey("pp_pair", fmt.Sprintf("%d", pair.DiscordID)))
	tx.Delete(getCacheKey("pp_pair_key", pair.Pair.Key))
	return tx.Commit()
}
