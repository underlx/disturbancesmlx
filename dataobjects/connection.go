package dataobjects

import (
	"errors"
	"fmt"

	sq "github.com/gbl08ma/squirrel"
	"github.com/gbl08ma/sqalx"
)

// Connection connects two stations in a single direction
type Connection struct {
	From *Station
	To   *Station
	// TypicalWaitingSeconds: time in seconds it usually takes to catch a train at the From station when moving towards To
	TypicalWaitingSeconds int
	// TypicalStopSeconds: time in seconds the train usually stops at the From station when moving towards To
	TypicalStopSeconds int
	// TypicalSeconds: the time in seconds it usually takes for the train to move from From to To
	TypicalSeconds int
	// WorldLength: the physical length of this connection in meters
	WorldLength int
}

// GetConnections returns a slice with all registered connections
func GetConnections(node sqalx.Node) ([]*Connection, error) {
	return getConnectionsWithSelect(node, sdb.Select())
}

func getConnectionsWithSelect(node sqalx.Node, sbuilder sq.SelectBuilder) ([]*Connection, error) {
	connections := []*Connection{}

	tx, err := node.Beginx()
	if err != nil {
		return connections, err
	}
	defer tx.Commit() // read-only tx

	rows, err := sbuilder.Columns("from_station", "to_station", "typ_wait_time",
		"typ_stop_time", "typ_time", "world_length").
		From("connection").
		RunWith(tx).Query()
	if err != nil {
		return connections, fmt.Errorf("getConnectionsWithSelect: %s", err)
	}
	defer rows.Close()

	var fromIDs []string
	var toIDs []string
	for rows.Next() {
		var connection Connection
		var fromID string
		var toID string
		err := rows.Scan(
			&fromID,
			&toID,
			&connection.TypicalWaitingSeconds,
			&connection.TypicalStopSeconds,
			&connection.TypicalSeconds,
			&connection.WorldLength)
		if err != nil {
			return connections, fmt.Errorf("getConnectionsWithSelect: %s", err)
		}
		if err != nil {
			return connections, fmt.Errorf("getConnectionsWithSelect: %s", err)
		}
		connections = append(connections, &connection)
		fromIDs = append(fromIDs, fromID)
		toIDs = append(toIDs, toID)
	}
	if err := rows.Err(); err != nil {
		return connections, fmt.Errorf("getConnectionsWithSelect: %s", err)
	}
	for i := range connections {
		connections[i].From, err = GetStation(tx, fromIDs[i])
		if err != nil {
			return connections, fmt.Errorf("getConnectionsWithSelect: %s", err)
		}
		connections[i].To, err = GetStation(tx, toIDs[i])
		if err != nil {
			return connections, fmt.Errorf("getConnectionsWithSelect: %s", err)
		}
	}
	return connections, nil
}

// GetConnection returns the Connection with the given ID
func GetConnection(node sqalx.Node, from string, to string) (*Connection, error) {
	s := sdb.Select().
		Where(sq.Eq{"from_station": from}).
		Where(sq.Eq{"to_station": to})
	connections, err := getConnectionsWithSelect(node, s)
	if err != nil {
		return nil, err
	}
	if len(connections) == 0 {
		return nil, errors.New("Connection not found")
	}
	return connections[0], nil
}

// Update adds or updates the connection
func (connection *Connection) Update(node sqalx.Node) error {
	tx, err := node.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = sdb.Insert("connection").
		Columns("from_station", "to_station", "typ_wait_time",
			"typ_stop_time", "typ_time", "world_length").
		Values(connection.From.ID, connection.To.ID, connection.TypicalWaitingSeconds,
			connection.TypicalStopSeconds, connection.TypicalSeconds, connection.WorldLength).
		Suffix("ON CONFLICT (from_station, to_station) DO UPDATE SET typ_wait_time = ?, typ_stop_time = ?, typ_time = ?, world_length = ?",
			connection.TypicalWaitingSeconds, connection.TypicalStopSeconds, connection.TypicalSeconds, connection.WorldLength).
		RunWith(tx).Exec()

	if err != nil {
		return errors.New("AddConnection: " + err.Error())
	}
	return tx.Commit()
}

// Delete deletes the connection
func (connection *Connection) Delete(node sqalx.Node) error {
	tx, err := node.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = sdb.Delete("connection").
		Where(sq.Eq{"from_station": connection.From.ID}).
		Where(sq.Eq{"to_station": connection.To.ID}).
		RunWith(tx).Exec()
	if err != nil {
		return fmt.Errorf("RemoveConnection: %s", err)
	}
	return tx.Commit()
}
