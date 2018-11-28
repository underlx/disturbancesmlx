package dataobjects

import (
	"time"

	"github.com/gbl08ma/sqalx"
)

// PPLeaderboardEntry represents a PosPlay leaderboard entry
type PPLeaderboardEntry struct {
	Position int
	Player   *PPPlayer
	Score    int
}

// PPLeaderboardBetween returns the PosPlay leaderboard for the specified period
func PPLeaderboardBetween(node sqalx.Node, start, end time.Time, size int, mustInclude *PPPlayer) ([]PPLeaderboardEntry, error) {
	tx, err := node.Beginx()
	if err != nil {
		return []PPLeaderboardEntry{}, err
	}
	defer tx.Commit() // read-only tx

	includedRequiredPlayer := false

	rows, err := tx.Query("SELECT discord_id, SUM(value) AS s, rank() OVER (ORDER BY sum(value) DESC) AS position "+
		"FROM pp_xp_tx "+
		"WHERE timestamp BETWEEN $1 AND $2 "+
		"GROUP BY discord_id LIMIT $3;",
		start, end, size)
	if err != nil {
		return []PPLeaderboardEntry{}, err
	}
	defer rows.Close()

	entries := []PPLeaderboardEntry{}
	var discordIDs []uint64
	for rows.Next() {
		var discordID uint64
		entry := PPLeaderboardEntry{}
		err := rows.Scan(&discordID, &entry.Score, &entry.Position)
		if err != nil {
			return entries, err
		}
		discordIDs = append(discordIDs, discordID)
		entries = append(entries, entry)
		if mustInclude != nil && discordID == mustInclude.DiscordID {
			includedRequiredPlayer = true
		}
	}
	for i := range discordIDs {
		entries[i].Player, err = GetPPPlayer(tx, discordIDs[i])
		if err != nil {
			return nil, err
		}
	}

	if !includedRequiredPlayer && mustInclude != nil {
		rank, err := mustInclude.RankBetween(tx, start, end)
		if err != nil {
			return nil, err
		}

		score, err := mustInclude.XPBalanceBetween(tx, start, end)
		if err != nil {
			return nil, err
		}

		entries = append(entries, PPLeaderboardEntry{
			Position: rank,
			Player:   mustInclude,
			Score:    score,
		})
	}
	return entries, rows.Err()
}
