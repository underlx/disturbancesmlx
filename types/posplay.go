package types

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gbl08ma/sqalx"
	"github.com/thoas/go-funk"
)

// PPLeaderboardEntry represents a PosPlay leaderboard entry
type PPLeaderboardEntry struct {
	RowNum   int
	Position int
	Player   *PPPlayer
	Score    int
}

// PPLeaderboardBetween returns the PosPlay leaderboard for the specified period
func PPLeaderboardBetween(node sqalx.Node, start, end time.Time, size int, showNeighbors int, mustIncludes ...*PPPlayer) ([]PPLeaderboardEntry, error) {
	tx, err := node.Beginx()
	if err != nil {
		return []PPLeaderboardEntry{}, err
	}
	defer tx.Commit() // read-only tx

	query := `WITH lb AS (
		SELECT
			discord_id,
			SUM(value) AS s,
			rank() OVER (ORDER BY sum(value) DESC) AS position,
			row_number() OVER (ORDER BY sum(value) DESC) AS rownum
		FROM pp_xp_tx
		WHERE timestamp BETWEEN $1 AND $2
		GROUP BY discord_id
	)`

	if len(mustIncludes) > 0 {
		mustIDs := funk.Map(mustIncludes, func(p *PPPlayer) string {
			return strconv.FormatUint(p.DiscordID, 10)
		}).([]string)

		query += fmt.Sprintf(", mi AS (SELECT DISTINCT rownum AS mirn FROM lb WHERE discord_id IN (%s))",
			strings.Join(mustIDs, ", "))
	}

	query += `SELECT discord_id, s, position, rownum
	FROM lb
	WHERE rownum <= $3`

	if len(mustIncludes) > 0 {
		query += fmt.Sprintf(" OR EXISTS (SELECT 1 FROM mi WHERE ABS(rownum - mirn) <= %d)", showNeighbors)
	}

	rows, err := tx.Query(query, start, end, size)
	if err != nil {
		return []PPLeaderboardEntry{}, err
	}
	defer rows.Close()

	entries := []PPLeaderboardEntry{}
	var discordIDs []uint64
	for rows.Next() {
		var discordID uint64
		entry := PPLeaderboardEntry{}
		err := rows.Scan(&discordID, &entry.Score, &entry.Position, &entry.RowNum)
		if err != nil {
			return entries, err
		}
		discordIDs = append(discordIDs, discordID)
		entries = append(entries, entry)
	}
	for i := range discordIDs {
		entries[i].Player, err = GetPPPlayer(tx, discordIDs[i])
		if err != nil {
			return nil, err
		}
	}
	return entries, rows.Err()
}
