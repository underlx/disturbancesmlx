package dataobjects

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/gbl08ma/sqalx"
	sq "github.com/gbl08ma/squirrel"
)

// PPXPTransaction is a PosPlay XP transaction
type PPXPTransaction struct {
	ID        string
	DiscordID uint64
	Time      time.Time
	Value     int
	Type      string
	Extra     string
}

// GetPPXPTransactions returns a slice with all registered transactions
func GetPPXPTransactions(node sqalx.Node) ([]*PPXPTransaction, error) {
	return getPPXPTransactionsWithSelect(node, sdb.Select())
}

// GetPPXPTransactionsBetween returns a slice with all transactions within the specified interval
func GetPPXPTransactionsBetween(node sqalx.Node, start time.Time, end time.Time) ([]*PPXPTransaction, error) {
	s := sdb.Select().
		Where(sq.Expr("timestamp BETWEEN ? AND ?",
			start, end)).
		OrderBy("timestamp ASC")
	return getPPXPTransactionsWithSelect(node, s)
}

// GetPPXPTransactionsWithType returns a slice with all transactions with the specified type
func GetPPXPTransactionsWithType(node sqalx.Node, txtype string) ([]*PPXPTransaction, error) {
	s := sdb.Select().
		Where(sq.Eq{"type": txtype}).
		OrderBy("timestamp ASC")
	return getPPXPTransactionsWithSelect(node, s)
}

func getPPXPTransactionsWithSelect(node sqalx.Node, sbuilder sq.SelectBuilder) ([]*PPXPTransaction, error) {
	transactions := []*PPXPTransaction{}

	tx, err := node.Beginx()
	if err != nil {
		return transactions, err
	}
	defer tx.Commit() // read-only tx

	rows, err := sbuilder.Columns("pp_xp_tx.id", "pp_xp_tx.discord_id",
		"pp_xp_tx.timestamp", "pp_xp_tx.value", "pp_xp_tx.type", "pp_xp_tx.extra").
		From("pp_xp_tx").
		RunWith(tx).Query()
	if err != nil {
		return transactions, fmt.Errorf("getPPXPTransactionsWithSelect: %s", err)
	}
	defer rows.Close()

	for rows.Next() {
		var transaction PPXPTransaction
		err := rows.Scan(
			&transaction.ID,
			&transaction.DiscordID,
			&transaction.Time,
			&transaction.Value,
			&transaction.Type,
			&transaction.Extra)
		if err != nil {
			return transactions, fmt.Errorf("getPPXPTransactionsWithSelect: %s", err)
		}
		if err != nil {
			return transactions, fmt.Errorf("getPPXPTransactionsWithSelect: %s", err)
		}
		transactions = append(transactions, &transaction)
	}
	if err := rows.Err(); err != nil {
		return transactions, fmt.Errorf("getPPXPTransactionsWithSelect: %s", err)
	}
	return transactions, nil
}

// GetPPXPTransaction returns the transaction with the given ID
func GetPPXPTransaction(node sqalx.Node, id string) (*PPXPTransaction, error) {
	if value, present := node.Load(getCacheKey("pp_xp_tx", id)); present {
		return value.(*PPXPTransaction), nil
	}
	s := sdb.Select().
		Where(sq.Eq{"id": id})
	transactions, err := getPPXPTransactionsWithSelect(node, s)
	if err != nil {
		return nil, err
	}
	if len(transactions) == 0 {
		return nil, errors.New("PPXPTransaction not found")
	}
	node.Store(getCacheKey("pp_xp_tx", id), transactions[0])
	return transactions[0], nil
}

// UnmarshalExtra decodes the Extra field for this transaction as JSON
func (transaction *PPXPTransaction) UnmarshalExtra() map[string]interface{} {
	var f interface{}
	err := json.Unmarshal([]byte(transaction.Extra), &f)
	if err != nil {
		return make(map[string]interface{})
	}
	return f.(map[string]interface{})
}

// MarshalExtra encodes the parameter as JSON and places the result in the Extra field
func (transaction *PPXPTransaction) MarshalExtra(f map[string]interface{}) {
	b, err := json.Marshal(f)
	if err == nil {
		transaction.Extra = string(b)
	}
}

// Update adds or updates the PPXPTransaction
func (transaction *PPXPTransaction) Update(node sqalx.Node) error {
	tx, err := node.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = sdb.Insert("pp_xp_tx").
		Columns("id", "discord_id", "timestamp", "value", "type", "extra").
		Values(transaction.ID, transaction.DiscordID, transaction.Time, transaction.Value, transaction.Type, transaction.Extra).
		Suffix("ON CONFLICT (id) DO UPDATE SET discord_id = ?, timestamp = ?, value = ?, type = ?, extra = ?",
			transaction.DiscordID, transaction.Time, transaction.Value, transaction.Type, transaction.Extra).
		RunWith(tx).Exec()

	if err != nil {
		return errors.New("AddPPXPTransaction: " + err.Error())
	}
	return tx.Commit()
}

// Delete deletes the PPXPTransaction
func (transaction *PPXPTransaction) Delete(node sqalx.Node) error {
	tx, err := node.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = sdb.Delete("pp_xp_tx").
		Where(sq.Eq{"id": transaction.ID}).RunWith(tx).Exec()
	if err != nil {
		return fmt.Errorf("RemovePPXPTransaction: %s", err)
	}
	tx.Delete(getCacheKey("pp_xp_tx", transaction.ID))
	return tx.Commit()
}
