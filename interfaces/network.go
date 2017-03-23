package interfaces

import (
	"errors"
	"fmt"

	"sort"

	sq "github.com/gbl08ma/squirrel"
	"github.com/heetch/sqalx"
)

// Network is a transportation network
type Network struct {
	ID   string
	Name string
}

// GetNetworks returns a slice with all registered networks
func GetNetworks(node sqalx.Node) ([]*Network, error) {
	networks := []*Network{}

	tx, err := node.Beginx()
	if err != nil {
		return networks, err
	}
	defer tx.Commit() // read-only tx

	rows, err := sdb.Select("id", "name").
		From("network").RunWith(tx).Query()
	if err != nil {
		return networks, fmt.Errorf("GetNetworks: %s", err)
	}
	defer rows.Close()

	for rows.Next() {
		var network Network
		err := rows.Scan(
			&network.ID,
			&network.Name)
		if err != nil {
			return networks, fmt.Errorf("GetNetworks: %s", err)
		}
		networks = append(networks, &network)
	}
	if err := rows.Err(); err != nil {
		return networks, fmt.Errorf("GetNetworks: %s", err)
	}
	return networks, nil
}

// GetNetwork returns the Network with the given ID
func GetNetwork(node sqalx.Node, id string) (*Network, error) {
	var network Network
	tx, err := node.Beginx()
	if err != nil {
		return &network, err
	}
	defer tx.Commit() // read-only tx

	err = sdb.Select("id", "name").
		From("network").
		Where(sq.Eq{"id": id}).
		RunWith(tx).QueryRow().Scan(&network.ID, &network.Name)
	if err != nil {
		return &network, errors.New("GetNetwork: " + err.Error())
	}
	return &network, nil
}

// Lines returns the lines in this network
func (network *Network) Lines(node sqalx.Node) ([]*Line, error) {
	s := sdb.Select().
		Where(sq.Eq{"network": network.ID})
	return getLinesWithSelect(node, s)
}

// LastDisturbance returns the latest disturbance affecting this line
func (network *Network) LastDisturbance(node sqalx.Node) (*Disturbance, error) {
	tx, err := node.Beginx()
	if err != nil {
		return nil, err
	}
	defer tx.Commit() // read-only tx
	lines, err := network.Lines(tx)
	if err != nil {
		return nil, errors.New("LastDisturbance: " + err.Error())
	}
	lastDisturbances := []*Disturbance{}
	for _, line := range lines {
		d, err := line.LastDisturbance(tx)
		if err != nil {
			continue
		}
		lastDisturbances = append(lastDisturbances, d)
	}
	if len(lastDisturbances) == 0 {
		return nil, errors.New("No disturbances for this network")
	}
	sort.Slice(lastDisturbances, func(iidx, jidx int) bool {
		i := lastDisturbances[iidx]
		j := lastDisturbances[jidx]
		// i < j ?
		if i.Ended && j.Ended {
			return i.EndTime.Before(j.EndTime)
		}
		if i.Ended && !j.Ended {
			return true
		}
		if !i.Ended && j.Ended {
			return false
		}
		return i.StartTime.Before(j.StartTime)
	})
	return lastDisturbances[len(lastDisturbances)-1], nil
}

// Update adds or updates the network
func (network *Network) Update(node sqalx.Node) error {
	tx, err := node.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = sdb.Insert("network").
		Columns("id", "name").
		Values(network.ID, network.Name).
		Suffix("ON CONFLICT (id) DO UPDATE SET name = ?",
			network.Name).
		RunWith(tx).Exec()

	if err != nil {
		return errors.New("AddNetwork: " + err.Error())
	}
	return tx.Commit()
}

// Delete deletes the network
func (network *Network) Delete(node sqalx.Node) error {
	tx, err := node.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = sdb.Delete("network").
		Where(sq.Eq{"id": network.ID}).RunWith(tx).Exec()
	if err != nil {
		return fmt.Errorf("RemoveNetwork: %s", err)
	}
	return tx.Commit()
}
