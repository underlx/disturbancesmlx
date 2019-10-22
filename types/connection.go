package types

import (
	"errors"
	"fmt"

	"github.com/gbl08ma/sqalx"
	sq "github.com/Masterminds/squirrel"
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

	FromPlatform string
	ToPlatform   string

	isCompatFake     bool
	compatOrigFirst  *Connection
	compatOrigSecond *Connection
}

// GetConnections returns a slice with all registered connections
func GetConnections(node sqalx.Node, closedCompat bool) ([]*Connection, error) {
	if value, present := node.Load(getCacheKey("connections", closedCompat)); present {
		return value.([]*Connection), nil
	}
	connections, err := getConnectionsWithSelect(node, sdb.Select(), closedCompat)
	if err != nil {
		return connections, err
	}
	node.Store(getCacheKey("connections", closedCompat), connections)
	return connections, nil
}

func getConnectionsWithSelect(node sqalx.Node, sbuilder sq.SelectBuilder, closedCompat bool) ([]*Connection, error) {
	connections := []*Connection{}

	tx, err := node.Beginx()
	if err != nil {
		return connections, err
	}
	defer tx.Commit() // read-only tx

	rows, err := sbuilder.Columns("from_station", "to_station", "typ_wait_time",
		"typ_stop_time", "typ_time", "world_length",
		"from_platform", "to_platform").
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
			&connection.WorldLength,
			&connection.FromPlatform,
			&connection.ToPlatform)
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
	if closedCompat {
		connections, err = provideClosedCompat(tx, connections)
		if err != nil {
			return connections, err
		}
	}
	return connections, nil
}

// GetConnection returns the Connection with the given ID
func GetConnection(node sqalx.Node, from string, to string, closedCompat bool) (*Connection, error) {
	if value, present := node.Load(getCacheKey("connection", from, to, closedCompat)); present {
		return value.(*Connection), nil
	}
	connections := []*Connection{}
	if closedCompat {
		cs, err := GetConnections(node, true)
		if err != nil {
			return nil, err
		}
		for _, c := range cs {
			if c.From.ID == from && c.To.ID == to {
				connections = []*Connection{c}
				break
			}
		}
	} else {
		var err error
		s := sdb.Select().
			Where(sq.Eq{"from_station": from}).
			Where(sq.Eq{"to_station": to})
		connections, err = getConnectionsWithSelect(node, s, closedCompat)
		if err != nil {
			return nil, err
		}
	}
	if len(connections) == 0 {
		return nil, errors.New("Connection not found")
	}
	node.Store(getCacheKey("connection", from, to, closedCompat), connections[0])
	return connections[0], nil
}

// GetConnectionsTo returns the Connections that point to the given ID
func GetConnectionsTo(node sqalx.Node, to string, closedCompat bool) ([]*Connection, error) {
	if value, present := node.Load(getCacheKey("connections-to", to, closedCompat)); present {
		return value.([]*Connection), nil
	}
	connections := []*Connection{}
	if closedCompat {
		cs, err := GetConnections(node, true)
		if err != nil {
			return nil, err
		}
		for _, c := range cs {
			if c.To.ID == to {
				connections = append(connections, c)
			}
		}
	} else {
		var err error
		s := sdb.Select().
			Where(sq.Eq{"to_station": to})
		connections, err = getConnectionsWithSelect(node, s, closedCompat)
		if err != nil {
			return nil, err
		}
	}
	node.Store(getCacheKey("connections-to", to, closedCompat), connections)
	return connections, nil
}

// GetConnectionsFrom returns the Connections that point out of the given ID
func GetConnectionsFrom(node sqalx.Node, from string, closedCompat bool) ([]*Connection, error) {
	if value, present := node.Load(getCacheKey("connections-from", from, closedCompat)); present {
		return value.([]*Connection), nil
	}
	connections := []*Connection{}
	if closedCompat {
		cs, err := GetConnections(node, true)
		if err != nil {
			return nil, err
		}
		for _, c := range cs {
			if c.From.ID == from {
				connections = append(connections, c)
			}
		}
	} else {
		var err error
		s := sdb.Select().
			Where(sq.Eq{"from_station": from})
		connections, err = getConnectionsWithSelect(node, s, closedCompat)
		if err != nil {
			return nil, err
		}
	}
	node.Store(getCacheKey("connections-from", from, closedCompat), connections)
	return connections, nil
}

// this function provides a compatibility mode that emulates the old way of
// dealing with closed stations, where closed stations retain only their
// outbound edges and replacement edges are created to connect the two nearest
// stations in the same line)
func provideClosedCompat(node sqalx.Node, origConnections []*Connection) ([]*Connection, error) {
	tx, err := node.Beginx()
	if err != nil {
		return nil, err
	}
	defer tx.Commit() // read-only tx

	connections := []*Connection{}
	for _, connection := range origConnections {
		closed, err := connection.To.Closed(tx)
		if err != nil {
			return connections, err
		}
		if closed {
			connFromClosed, err := GetConnectionsFrom(tx, connection.To.ID, false)
			if err != nil {
				return connections, err
			}
			fromLines, err := connection.From.Lines(tx)
			if err != nil {
				return connections, err
			}
			candidates := []*Connection{}
			for _, c := range connFromClosed {
				if c.To.ID == connection.From.ID {
					continue
				}
				cToLines, err := c.To.Lines(tx)
				if err != nil {
					return connections, err
				}
				func() {
					for _, fromLine := range fromLines {
						for _, toLine := range cToLines {
							if fromLine.ID == toLine.ID {
								candidates = append(candidates, c)
								return
							}
						}
					}
				}()
			}
			for _, c := range candidates {
				connections = append(connections, &Connection{
					From:                  connection.From,
					To:                    c.To,
					TypicalWaitingSeconds: connection.TypicalWaitingSeconds,
					TypicalStopSeconds:    connection.TypicalStopSeconds,
					TypicalSeconds:        connection.TypicalSeconds + c.TypicalSeconds,
					WorldLength:           connection.WorldLength + c.WorldLength,
					FromPlatform:          connection.FromPlatform,
					ToPlatform:            connection.ToPlatform,
					isCompatFake:          true,
					compatOrigFirst:       connection,
					compatOrigSecond:      c,
				})
			}
		} else {
			connections = append(connections, connection)
		}
	}
	return connections, nil
}

// GetConnectionsToPlatform returns the Connections that end on the given platform ID
func GetConnectionsToPlatform(node sqalx.Node, toPlatform string) ([]*Connection, error) {
	if value, present := node.Load(getCacheKey("connections-toplatform", toPlatform)); present {
		return value.([]*Connection), nil
	}

	s := sdb.Select().
		Where(sq.Eq{"to_platform": toPlatform})
	connections, err := getConnectionsWithSelect(node, s, false)
	if err != nil {
		return nil, err
	}
	node.Store(getCacheKey("connections-toplatform", toPlatform), connections)
	return connections, nil
}

// GetConnectionsFromPlatform returns the Connections that start on the given platform ID
func GetConnectionsFromPlatform(node sqalx.Node, fromPlatform string) ([]*Connection, error) {
	if value, present := node.Load(getCacheKey("connections-fromplatform", fromPlatform)); present {
		return value.([]*Connection), nil
	}

	s := sdb.Select().
		Where(sq.Eq{"from_platform": fromPlatform})
	connections, err := getConnectionsWithSelect(node, s, false)
	if err != nil {
		return nil, err
	}
	node.Store(getCacheKey("connections-fromplatform", fromPlatform), connections)
	return connections, nil
}

// Update adds or updates the connection
func (connection *Connection) Update(node sqalx.Node) error {
	tx, err := node.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if connection.isCompatFake {
		connection.compatOrigFirst.TypicalSeconds = connection.TypicalSeconds / 2
		connection.compatOrigSecond.TypicalSeconds = connection.TypicalSeconds / 2
		for connection.compatOrigFirst.TypicalSeconds+connection.compatOrigSecond.TypicalSeconds < connection.TypicalSeconds {
			// make up lost in integer division
			connection.compatOrigSecond.TypicalSeconds++
		}

		// other ones only apply to the start of the edge, i.e. the original first
		connection.compatOrigFirst.TypicalStopSeconds = connection.TypicalStopSeconds
		connection.compatOrigFirst.TypicalWaitingSeconds = connection.TypicalWaitingSeconds

		err = connection.compatOrigFirst.Update(tx)
		if err != nil {
			return err
		}

		return connection.compatOrigSecond.Update(tx)
	}

	_, err = sdb.Insert("connection").
		Columns("from_station", "to_station", "typ_wait_time",
			"typ_stop_time", "typ_time", "world_length",
			"from_platform", "to_platform").
		Values(connection.From.ID, connection.To.ID, connection.TypicalWaitingSeconds,
			connection.TypicalStopSeconds, connection.TypicalSeconds, connection.WorldLength,
			connection.FromPlatform, connection.ToPlatform).
		Suffix("ON CONFLICT (from_station, to_station) DO UPDATE SET typ_wait_time = ?, typ_stop_time = ?, typ_time = ?, world_length = ?, from_platform = ?, to_platform = ?",
			connection.TypicalWaitingSeconds, connection.TypicalStopSeconds, connection.TypicalSeconds, connection.WorldLength, connection.FromPlatform, connection.ToPlatform).
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
	tx.Delete(getCacheKey("connection", connection.From.ID, connection.To.ID, true))
	tx.Delete(getCacheKey("connection", connection.From.ID, connection.To.ID, false))
	return tx.Commit()
}
