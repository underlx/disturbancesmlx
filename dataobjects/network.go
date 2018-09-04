package dataobjects

import (
	"errors"
	"fmt"
	"time"

	"sort"

	"github.com/gbl08ma/sqalx"
	sq "github.com/gbl08ma/squirrel"
	"github.com/lib/pq"
)

// Network is a transportation network
type Network struct {
	ID           string
	Name         string
	MainLocale   string
	Names        map[string]string
	TypicalCars  int
	Holidays     []int64
	OpenTime     Time
	OpenDuration Duration
	Timezone     string
	NewsURL      string
}

// GetNetworks returns a slice with all registered networks
func GetNetworks(node sqalx.Node) ([]*Network, error) {
	return getNetworksWithSelect(node, sdb.Select())
}

// GetNetwork returns the Line with the given ID
func GetNetwork(node sqalx.Node, id string) (*Network, error) {
	if value, present := node.Load(getCacheKey("network", id)); present {
		return value.(*Network), nil
	}
	s := sdb.Select().
		Where(sq.Eq{"network.id": id})
	networks, err := getNetworksWithSelect(node, s)
	if err != nil {
		return nil, err
	}
	if len(networks) == 0 {
		return nil, errors.New("Network not found")
	}
	node.Store(getCacheKey("network", id), networks[0])
	return networks[0], nil
}

func getNetworksWithSelect(node sqalx.Node, sbuilder sq.SelectBuilder) ([]*Network, error) {
	networks := []*Network{}
	networkMap := make(map[string]*Network)

	tx, err := node.Beginx()
	if err != nil {
		return networks, err
	}
	defer tx.Commit() // read-only tx

	rows, err := sbuilder.Columns("network.id", "network.name", "network.typ_cars", "network.holidays", "network.open_time", "network.open_duration", "network.timezone", "network.news_url").
		From("network").RunWith(tx).Query()
	if err != nil {
		return networks, fmt.Errorf("getNetworksWithSelect: %s", err)
	}
	defer rows.Close()

	for rows.Next() {
		var network Network
		var holidays pq.Int64Array
		err := rows.Scan(
			&network.ID,
			&network.Name,
			&network.TypicalCars,
			&holidays,
			&network.OpenTime,
			&network.OpenDuration,
			&network.Timezone,
			&network.NewsURL)
		if err != nil {
			return networks, fmt.Errorf("getNetworksWithSelect: %s", err)
		}
		network.Holidays = holidays
		networks = append(networks, &network)
		networkMap[network.ID] = &network
		networkMap[network.ID].Names = make(map[string]string)
	}
	if err := rows.Err(); err != nil {
		return networks, fmt.Errorf("getNetworksWithSelect: %s", err)
	}

	// get MainLocale for each network
	rows2, err := sbuilder.Columns("network.id", "network_name.lang").
		From("network").
		Join("network_name ON network.id = network_name.id AND network_name.main = true").
		RunWith(tx).Query()

	if err != nil {
		return networks, fmt.Errorf("getNetworksWithSelect: %s", err)
	}
	defer rows2.Close()

	for rows2.Next() {
		var id string
		var lang string
		err := rows2.Scan(&id, &lang)
		if err != nil {
			return networks, fmt.Errorf("getNetworksWithSelect: %s", err)
		}
		networkMap[id].MainLocale = lang
	}
	if err := rows2.Err(); err != nil {
		return networks, fmt.Errorf("getNetworksWithSelect: %s", err)
	}

	// get localized name map for each network
	rows3, err := sbuilder.Columns("network.id", "network_name.lang", "network_name.name").
		From("network").
		Join("network_name ON network.id = network_name.id").
		RunWith(tx).Query()
	if err != nil {
		return networks, fmt.Errorf("getNetworksWithSelect: %s", err)
	}
	defer rows3.Close()

	for rows3.Next() {
		var id string
		var lang string
		var name string
		err := rows3.Scan(&id, &lang, &name)
		if err != nil {
			return networks, fmt.Errorf("getNetworksWithSelect: %s", err)
		}
		networkMap[id].Names[lang] = name
	}
	if err := rows3.Err(); err != nil {
		return networks, fmt.Errorf("getNetworksWithSelect: %s", err)
	}

	return networks, nil
}

// Lines returns the lines in this network
func (network *Network) Lines(node sqalx.Node) ([]*Line, error) {
	s := sdb.Select().
		Where(sq.Eq{"network": network.ID}).
		OrderBy("\"order\" ASC")
	return getLinesWithSelect(node, s)
}

// Stations returns the stations in this network
func (network *Network) Stations(node sqalx.Node) ([]*Station, error) {
	s := sdb.Select().
		Where(sq.Eq{"network": network.ID})
	return getStationsWithSelect(node, s)
}

// Schedules returns the schedules of this network
func (network *Network) Schedules(node sqalx.Node) ([]*NetworkSchedule, error) {
	s := sdb.Select().
		Where(sq.Eq{"network_id": network.ID})
	return getNetworkSchedulesWithSelect(node, s)
}

// LastDisturbance returns the latest disturbance affecting this network
func (network *Network) LastDisturbance(node sqalx.Node, officialOnly bool) (*Disturbance, error) {
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
		d, err := line.LastDisturbance(tx, officialOnly)
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

		iStartTime := i.UStartTime
		iEndTime := i.UEndTime
		iEnded := i.UEnded
		if officialOnly {
			iStartTime = i.OStartTime
			iEndTime = i.OEndTime
			iEnded = i.OEnded
		}

		jStartTime := j.UStartTime
		jEndTime := j.UEndTime
		jEnded := j.UEnded
		if officialOnly {
			jStartTime = j.OStartTime
			jEndTime = j.OEndTime
			jEnded = j.OEnded
		}

		// i < j ?
		if iEnded && jEnded {
			return iEndTime.Before(jEndTime)
		}
		if iEnded && !jEnded {
			return true
		}
		if !iEnded && jEnded {
			return false
		}
		return iStartTime.Before(jStartTime)
	})
	return lastDisturbances[len(lastDisturbances)-1], nil
}

// CountDisturbancesByHour counts disturbances by hour between the specified dates
func (network *Network) CountDisturbancesByHour(node sqalx.Node, start time.Time, end time.Time) ([]int, error) {
	tx, err := node.Beginx()
	if err != nil {
		return []int{}, err
	}
	defer tx.Commit() // read-only tx

	rows, err := tx.Query("SELECT curd, COUNT(id) "+
		"FROM generate_series(date_trunc('hour', $2 at time zone $1), date_trunc('hour', $3 at time zone $1), '1 hour') AS curd "+
		"LEFT OUTER JOIN line_disturbance ON "+
		"(curd BETWEEN date_trunc('hour', time_start at time zone $1) AND date_trunc('hour', coalesce(time_end, now()) at time zone $1)) "+
		"GROUP BY curd ORDER BY curd;",
		start.Location().String(), start, end)
	if err != nil {
		return []int{}, fmt.Errorf("CountDisturbancesByHour: %s", err)
	}
	defer rows.Close()

	var counts []int
	for rows.Next() {
		var date time.Time
		var count int
		err := rows.Scan(&date, &count)
		if err != nil {
			return counts, fmt.Errorf("CountDisturbancesByHour: %s", err)
		}
		if err != nil {
			return counts, fmt.Errorf("CountDisturbancesByHour: %s", err)
		}
		counts = append(counts, count)
	}
	if err := rows.Err(); err != nil {
		return counts, fmt.Errorf("CountDisturbancesByHour: %s", err)
	}
	return counts, nil
}

// Update adds or updates the network
func (network *Network) Update(node sqalx.Node) error {
	tx, err := node.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = sdb.Insert("network").
		Columns("id", "name", "typ_cars", "holidays", "open_time", "open_duration", "timezone", "news_url").
		Values(network.ID, network.Name, network.TypicalCars, pq.Int64Array(network.Holidays), network.OpenTime, network.OpenDuration, network.Timezone, network.NewsURL).
		Suffix("ON CONFLICT (id) DO UPDATE SET name = ?, typ_cars = ?, holidays = ?, open_time = ?, open_duration = ?, timezone = ?, news_url = ?",
			network.Name, network.TypicalCars, pq.Int64Array(network.Holidays), network.OpenTime, network.OpenDuration, network.Timezone, network.NewsURL).
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
	tx.Delete(getCacheKey("network", network.ID))
	return tx.Commit()
}
