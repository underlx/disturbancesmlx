package interfaces

import (
	"errors"
	"fmt"

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
