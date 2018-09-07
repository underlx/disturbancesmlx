package posplay

import (
	"math"
	"time"

	uuid "github.com/satori/go.uuid"
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

	txid, err := uuid.NewV4()
	if err != nil {
		return err
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

	xptx := &dataobjects.PPXPTransaction{
		ID:        txid.String(),
		DiscordID: pair.DiscordID,
		Time:      time.Now(),
		Type:      "TRIP_SUBMIT_REWARD",
		Value:     reward,
	}
	xptx.MarshalExtra(map[string]interface{}{
		"trip_id":           trip.ID,
		"station_count":     nstations,
		"interchange_count": ninterchanges,
		"offpeak":           offpeak,
	})

	err = xptx.Update(tx)
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

	txid, err := uuid.NewV4()
	if err != nil {
		return err
	}

	xptx := &dataobjects.PPXPTransaction{
		ID:        txid.String(),
		DiscordID: pair.DiscordID,
		Time:      time.Now(),
		Type:      "TRIP_CONFIRM_REWARD",
		Value:     5,
	}
	xptx.MarshalExtra(map[string]interface{}{
		"trip_id": trip.ID,
	})

	err = xptx.Update(tx)
	if err != nil {
		return err
	}

	return tx.Commit()
}
