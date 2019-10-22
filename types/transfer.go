package types

import (
	"errors"
	"fmt"

	"github.com/gbl08ma/sqalx"
	sq "github.com/Masterminds/squirrel"
)

// Transfer represents a crossing between two lines at a station
type Transfer struct {
	Station        *Station
	From           *Line
	To             *Line
	TypicalSeconds int
}

// GetTransfers returns a slice with all registered transfers
func GetTransfers(node sqalx.Node) ([]*Transfer, error) {
	return getTransfersWithSelect(node, sdb.Select())
}

func getTransfersWithSelect(node sqalx.Node, sbuilder sq.SelectBuilder) ([]*Transfer, error) {
	transfers := []*Transfer{}

	tx, err := node.Beginx()
	if err != nil {
		return transfers, err
	}
	defer tx.Commit() // read-only tx

	rows, err := sbuilder.Columns("station_id", "from_line", "to_line", "typ_time").
		From("transfer").
		RunWith(tx).Query()
	if err != nil {
		return transfers, fmt.Errorf("getTransfersWithSelect: %s", err)
	}
	defer rows.Close()

	var stationIDs []string
	var fromIDs []string
	var toIDs []string
	for rows.Next() {
		var transfer Transfer
		var stationID string
		var fromID string
		var toID string
		err := rows.Scan(
			&stationID,
			&fromID,
			&toID,
			&transfer.TypicalSeconds)
		if err != nil {
			return transfers, fmt.Errorf("getTransfersWithSelect: %s", err)
		}
		if err != nil {
			return transfers, fmt.Errorf("getTransfersWithSelect: %s", err)
		}
		transfers = append(transfers, &transfer)
		stationIDs = append(stationIDs, stationID)
		fromIDs = append(fromIDs, fromID)
		toIDs = append(toIDs, toID)
	}
	if err := rows.Err(); err != nil {
		return transfers, fmt.Errorf("getTransfersWithSelect: %s", err)
	}
	for i := range transfers {
		transfers[i].Station, err = GetStation(tx, stationIDs[i])
		if err != nil {
			return transfers, fmt.Errorf("getTransfersWithSelect: %s", err)
		}
		transfers[i].From, err = GetLine(tx, fromIDs[i])
		if err != nil {
			return transfers, fmt.Errorf("getTransfersWithSelect: %s", err)
		}
		transfers[i].To, err = GetLine(tx, toIDs[i])
		if err != nil {
			return transfers, fmt.Errorf("getTransfersWithSelect: %s", err)
		}
	}
	return transfers, nil
}

// GetTransfer returns the Transfer with the given ID
func GetTransfer(node sqalx.Node, station string, from string, to string) (*Transfer, error) {
	if value, present := node.Load(getCacheKey("transfer", station, from, to)); present {
		return value.(*Transfer), nil
	}
	s := sdb.Select().
		Where(sq.Eq{"station_id": station}).
		Where(sq.Eq{"from_line": from}).
		Where(sq.Eq{"to_line": to})
	transfers, err := getTransfersWithSelect(node, s)
	if err != nil {
		return nil, err
	}
	if len(transfers) == 0 {
		return nil, errors.New("Transfer not found")
	}
	node.Store(getCacheKey("transfer", station, from, to), transfers[0])
	return transfers[0], nil
}

// Update adds or updates the transfer
func (transfer *Transfer) Update(node sqalx.Node) error {
	tx, err := node.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = sdb.Insert("transfer").
		Columns("station_id", "from_line", "to_line", "typ_time").
		Values(transfer.Station.ID, transfer.From.ID, transfer.To.ID, transfer.TypicalSeconds).
		Suffix("ON CONFLICT (station_id, from_line, to_line) DO UPDATE SET typ_time = ?",
			transfer.TypicalSeconds).
		RunWith(tx).Exec()

	if err != nil {
		return errors.New("AddTransfer: " + err.Error())
	}
	return tx.Commit()
}

// Delete deletes the transfer
func (transfer *Transfer) Delete(node sqalx.Node) error {
	tx, err := node.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = sdb.Delete("transfer").
		Where(sq.Eq{"station_id": transfer.Station.ID}).
		Where(sq.Eq{"from_line": transfer.From.ID}).
		Where(sq.Eq{"to_line": transfer.To.ID}).
		RunWith(tx).Exec()
	if err != nil {
		return fmt.Errorf("RemoveTransfer: %s", err)
	}
	tx.Delete(getCacheKey("transfer", transfer.Station.ID, transfer.From.ID, transfer.To.ID))
	return tx.Commit()
}
