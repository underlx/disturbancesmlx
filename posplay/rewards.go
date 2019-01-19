package posplay

import (
	"math"
	"time"

	"github.com/underlx/disturbancesmlx/dataobjects"
)

func processTripForReward(id string) error {
	tx, err := config.Node.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	trip, err := dataobjects.GetTrip(tx, id)
	if err != nil {
		return err
	}

	// is this submitter even linked with a PosPlay account?
	pair, err := dataobjects.GetPPPairForKey(tx, trip.Submitter.Key)
	if err != nil {
		// the answer is no, move on
		return nil
	}

	reward, nstations, ninterchanges, offpeak := computeTripXPReward(trip)
	if reward == 0 {
		// no reward, no transaction
		return nil
	}

	player, err := dataobjects.GetPPPlayer(tx, pair.DiscordID)
	if err != nil {
		return err
	}
	// apply bonus for being in the project guild
	if player.InGuild {
		reward = int(math.Round(float64(reward) * 1.1))
	}

	err = DoXPTransaction(tx, player, trip.SubmitTime, reward, "TRIP_SUBMIT_REWARD",
		map[string]interface{}{
			"trip_id":           trip.ID,
			"station_count":     nstations,
			"interchange_count": ninterchanges,
			"offpeak":           offpeak,
		}, false)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func computeTripXPReward(trip *dataobjects.Trip) (int, int, int, bool) {
	visitedStations := make(map[string]bool)
	interchanges := make(map[string]bool)
	var network *dataobjects.Network
	for _, use := range trip.StationUses {
		visitedStations[use.Station.ID] = true
		if use.Type == dataobjects.Interchange {
			interchanges[use.Station.ID] = true
		}
		network = use.Station.Network
	}
	if len(visitedStations) < 2 {
		return 0, len(visitedStations), len(interchanges), false
	}

	loc, _ := time.LoadLocation(network.Timezone)

	xp := 7*len(visitedStations) + 7*len(interchanges)

	// check whether this trip was made during off-peak hours
	hour := trip.StartTime.In(loc).Hour()
	onpeak := (hour >= 7 && hour < 9) || (hour >= 16 && hour < 19)

	if !onpeak {
		// apply off-peak bonus
		xp = int(math.Round(float64(xp) * 1.2))
	}

	return xp, len(visitedStations), len(interchanges), !onpeak
}

func processTripEditForReward(id string) error {
	tx, err := config.Node.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	trip, err := dataobjects.GetTrip(tx, id)
	if err != nil {
		return err
	}

	// is this submitter even linked with a PosPlay account?
	pair, err := dataobjects.GetPPPairForKey(tx, trip.Submitter.Key)
	if err != nil {
		// the answer is no, move on
		return nil
	}

	player, err := dataobjects.GetPPPlayer(tx, pair.DiscordID)
	if err != nil {
		return err
	}

	err = DoXPTransaction(tx, player, trip.EditTime, 5, "TRIP_CONFIRM_REWARD", map[string]interface{}{
		"trip_id": trip.ID,
	}, false)
	if err != nil {
		return err
	}

	return tx.Commit()
}
