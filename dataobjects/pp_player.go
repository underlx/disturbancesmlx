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
	DiscordID  uint64
	Joined     time.Time
	LBPrivacy  string
	NameType   string
	InGuild    bool
	CachedName string
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
		"pp_player.lb_privacy", "pp_player.name_type", "pp_player.in_guild",
		"pp_player.cached_name").
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
			&player.InGuild,
			&player.CachedName)
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

// CountPPPlayers returns the total number of players
func CountPPPlayers(node sqalx.Node) (int, error) {
	tx, err := node.Beginx()
	if err != nil {
		return 0, err
	}
	defer tx.Commit() // read-only tx

	var count int
	err = sdb.Select("COUNT(*)").
		From("pp_player").
		RunWith(tx).
		Scan(&count)
	return count, err
}

// XPTransactions returns a slice with all registered transactions for this player
func (player *PPPlayer) XPTransactions(node sqalx.Node) ([]*PPXPTransaction, error) {
	s := sdb.Select().
		Where(sq.Eq{"discord_id": player.DiscordID}).
		OrderBy("timestamp DESC")
	return getPPXPTransactionsWithSelect(node, s)
}

// XPTransactionsWithType returns a slice with the transactions for this player of the specified type
func (player *PPPlayer) XPTransactionsWithType(node sqalx.Node, txtype string) ([]*PPXPTransaction, error) {
	s := sdb.Select().
		Where(sq.Eq{"discord_id": player.DiscordID}).
		Where(sq.Eq{"type": txtype}).
		OrderBy("timestamp DESC")
	return getPPXPTransactionsWithSelect(node, s)
}

// XPTransactionsCustomFilter returns a slice with the transactions for this player matching a custom filter
func (player *PPPlayer) XPTransactionsCustomFilter(node sqalx.Node, preds ...interface{}) ([]*PPXPTransaction, error) {
	s := sdb.Select().
		Where(sq.Eq{"discord_id": player.DiscordID})

	for _, pred := range preds {
		s = s.Where(pred)
	}
	s = s.OrderBy("timestamp DESC")
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
	level, progress := PosPlayPlayerLevel(xp)
	return xp, level, progress, nil
}

// PosPlayPlayerLevel computes the PosPlay level and the % of progression to the next level given the XP total
func PosPlayPlayerLevel(totalXP int) (int, float64) {
	// progression = (xp/c)^(1/b)
	// c = 22.8376671827315
	// b = 1.62265355291952, 1/b = 0.6162744956869
	progression := math.Pow(float64(totalXP)/22.8376671827315, 0.6162744956869)
	level, part := math.Modf(progression)
	return int(level), part * 100
}

// PosPlayLevelToXP computes the PosPlay XP necessary to reach the given level
func PosPlayLevelToXP(level int) int {
	return int(22.8376671827315 * math.Pow(float64(level), 1.62265355291952))
}

// RankBetween returns the global XP rank for this player within the specified time interval
func (player *PPPlayer) RankBetween(node sqalx.Node, start, end time.Time) (int, error) {
	tx, err := node.Beginx()
	if err != nil {
		return 0, err
	}
	defer tx.Commit() // read-only tx

	rows, err := tx.Query("SELECT position "+
		"FROM ( "+
		"  SELECT discord_id, SUM(value) AS s, rank() OVER (ORDER BY sum(value) DESC) AS position "+
		"  FROM pp_xp_tx "+
		"  WHERE timestamp BETWEEN $1 AND $2 "+
		"  GROUP BY discord_id "+
		") AS leaderboard "+
		"WHERE leaderboard.discord_id = $3;",
		start, end, player.DiscordID)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	var rank int
	for rows.Next() {
		err := rows.Scan(&rank)
		if err != nil {
			return 0, err
		}
	}
	return rank, rows.Err()
}

// Achievements returns the PPPlayerAchievements for this player
// (achieved and non-achieved)
func (player *PPPlayer) Achievements(node sqalx.Node) ([]*PPPlayerAchievement, error) {
	s := sdb.Select().
		Where(sq.Eq{"discord_id": player.DiscordID}).
		OrderBy("achieved ASC")
	achievements, err := getPPPlayerAchievementsWithSelect(node, s)
	if err != nil {
		return nil, err
	}
	return achievements, nil
}

// Achievement returns the PPPlayerAchievement for this player corresponding
// to the given achievement ID
func (player *PPPlayer) Achievement(node sqalx.Node, achievementID string) (*PPPlayerAchievement, error) {
	s := sdb.Select().
		Where(sq.Eq{"discord_id": player.DiscordID}).
		Where(sq.Eq{"achievement_id": achievementID})
	achievements, err := getPPPlayerAchievementsWithSelect(node, s)
	if err != nil {
		return nil, err
	}
	if len(achievements) == 0 {
		return nil, errors.New("PPPlayerAchievement not found")
	}
	return achievements[0], nil
}

var anonymousPlayerNames = []string{
	"Guaxinim anónimo",
	"Ouriço-cacheiro anónimo",
	"Cotovia anónima",
	"Dragão-de-Komodo anónimo",
	"Estrela-do-mar anónima",
	"Garça-real anónima",
	"Gazela anónima",
	"Porquinho-da-índia anónimo",
	"Papa-formigas anónimo",
	"Ganso-patola anónimo",
	"Cegonha-preta anónima",
	"Cegonha-branca anónima",
	"Peneireiro-das-torres anónimo",
	"Gaivota-polar anónima",
	"Pombo-da-madeira anónimo",
	"Rouxinol-bravo anónimo",
	"Bengali-vermelho anónimo",
	"Pintassilgo anónimo",
	"Pintarroxo-de-bico-amarelo anónimo",
	"Cavalo-marinho anónimo",
	"Peixe-espada anónimo",
	"Peixe-palhaço anónimo",
	"Peixe-aranha anónimo",
	"Bacalhau anónimo",
	"Salmão anónimo",
	"Sardinha anónima",
	"Peixe-dourado anónimo",
	"Carapau anónimo",
	"Tartaruga-de-couro anónima",
	"Baleia-azul anónima",
	"Baleia-cinzenta anónima",
	"Crocodilo-do-nilo anónimo",
	"Crocodilo-marinho anónimo",
	"Salamandra-de-fogo anónima",
	"Coelho-bravo anónimo",
	"Lince-ibérico anónimo",
	"Lince-do-canadá anónimo",
	"Lince-pardo anónimo",
	"Lince-do-deserto anónimo",
	"Veado anónimo",
	"Morcego-negro anónimo",
	"Morcego-de-peluche anónimo",
	"Baleia-anã anónima",
	"Jacaré anónimo",
	"Coala anónimo",
	"Chinchila anónima",
	"Alce anónimo",
	"Lebre anónima",
	"Coelho anónimo",
	"Galinha anónima",
	"Burro-de-miranda anónimo",
	"Lemingue anónimo",
	"Coiote anónimo",
	"Lobo-guará anónimo",
	"Lobo-das-malvinas anónimo",
	"Cachorro-do-mato anónimo",
	"Raposa-do-ártico anónima",
	"Raposa-cinzenta anónima",
	"Raposa-das-ilhas anónima",
	"Raposa-de-Cozumel anónima",
	"Chacal anónimo",
	"Cão-selvagem-asiático anónimo",
	"Chita anónima",
	"Gazela-de-thomson anónima",
	"Gazela-de-grant anónima",
	"Hipopótamo-anão-do-chipre anónimo",
	"Hipopótamo-pigmeu anónimo",
	"Golfinho-chileno anónimo",
	"Golfinho-listrado anónimo",
	"Golfinho-de-bico-branco anónimo",
	"Coruja anónima",
	"Zebra-da-planície anónima",
	"Zebra-de-grevy anónima",
	"Zebra-da-montanha anónima",
	"Pavão-indiano anónimo",
	"Pavão-verde anónimo",
	"Pavão-do-Congo anónimo",
	"Pato-real anónimo",
	"Cavalo-de-przewalski anónimo",
	"Pónei anónimo",
	"Javali-europeu anónimo",
	"Javali-do-cárpatos anónimo",
	"Javali-norte-africano anónimo",
	"Javali-japonês anónimo",
	"Javali-malaio anónimo",
	"Javali-do-mediterrâneo anónimo",
	"Tigre-siberiano anónimo",
	"Tigre-de-sumatra anónimo",
	"Tigre-malaio anónimo",
	"Tigre-de-bali anónimo",
	"Tigre-de-bengala anónimo",
	"Tigre-do-cáspio anónimo",
	"Leão-asiático anónimo",
	"Leão-da-barbária anónimo",
	"Leão-do-senegal anónimo",
	"Leão-do-cabo anónimo",
	"Leão-do-katanga anónimo",
	"Jaguar anónimo",
	"Leopardo-nebuloso anónimo",
	"Gato-maracajá anónimo",
	"Gato-vermelho-de-bornéu anónimo",
	"Gato-andino anónimo",
	"Gato-do-deserto anónimo",
	"Gato-de-cabeça-chata anónimo",
	"Ligre anónimo",
	"Sapo anónimo",
	"Sapo-de-unha-negra anónimo",
	"Sapo-corredor anónimo",
	"Papagaio-de-bico-preto anónimo",
	"Papagaio-de-testa-branca anónimo",
	"Papagaio-de-nuca-amarela anónimo",
	"Papagaio-cubano anónimo",
	"Papagaio-de-ombro-amarelo anónimo",
	"Cisne-negro anónimo",
	"Cisne-bravo anónimo",
	"Cisne-pequeno anónimo",
	"Cisne-branco anónimo",
	"Cascavel anónima",
	"Jiboia anónima",
	"Píton-real anónima",
	"Píton-reticulada anónima",
	"Cobra-papagaio anónima",
	"Cobra-do-milho anónima",
	"Cobra-de-escada anónima",
	"Cobra-d'água anónima",
	"Camaleão anónimo",
	"Lagartixa anónima",
	"Iguana-verde anónima",
	"Iguana-do-caribe anónima",
	"Serpente anónima",
	"Cágado anónimo",
	"Tartaruga anónima",
	"Tartaruga-de-pente anónima",
	"Tartaruga-das-galápagos anónima",
	"Tartaruga-de-couro anónima",
	"Tartaruga-marinha anónima",
}

// AnonymousName returns an anonymous fixed name for this player
func (player *PPPlayer) AnonymousName() string {
	return anonymousPlayerNames[player.Seed()%uint64(len(anonymousPlayerNames))]
}

// Seed returns an unique ID for this user that is not his Discord ID
func (player *PPPlayer) Seed() uint64 {
	h := player.DiscordID * uint64(player.Joined.UnixNano())
	h ^= h >> 33
	h *= 0xff51afd7ed558ccd
	h ^= h >> 33
	h *= 0xc4ceb9fe1a85ec53
	h ^= h >> 33
	return h
}

// Update adds or updates the PPPlayer
func (player *PPPlayer) Update(node sqalx.Node) error {
	tx, err := node.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = sdb.Insert("pp_player").
		Columns("discord_id", "joined", "lb_privacy", "name_type", "in_guild", "cached_name").
		Values(player.DiscordID, player.Joined, player.LBPrivacy, player.NameType, player.InGuild, player.CachedName).
		Suffix("ON CONFLICT (discord_id) DO UPDATE SET joined = ?, lb_privacy = ?, name_type = ?, in_guild = ?, cached_name = ?",
			player.Joined, player.LBPrivacy, player.NameType, player.InGuild, player.CachedName).
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
