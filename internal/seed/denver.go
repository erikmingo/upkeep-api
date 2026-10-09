package seed

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/erikmingo/upkeep-api/internal/db"
)

// DenverFacts is the worked example from docs/spec-home-facts.md: a 1925 house in Denver.
var DenverFacts = map[string]any{
	"home.type": "house", "home.year_built": 1925, "home.stories": 1, "home.sqft": 1100, "home.beds": 2, "home.baths": 1,
	"furnace.type": "gas", "furnace.filter_size": "16x25x1", "ac.type": "central", "humidifier.present": false,
	"water_heater.type": "tank", "water_heater.installed": 2019, "softener.present": false, "water.source": "city", "sewer.type": "sewer", "sump_pump.present": false,
	"roof.material": "asphalt", "roof.installed": 2015, "gutters.present": true, "gutters.guards": false, "deck.material": "wood",
	"lawn.present": true, "sprinkler.present": true, "trees.near_house": true, "pool.type": "none",
	"detectors.smoke_count": 3, "detectors.co_count": 1, "extinguisher.present": false, "dryer.vent_length": "long",
	"fridge.water_filter": false, "dishwasher.present": true, "disposal.present": true, "range_hood.present": true,
	"garage.present": true, "garage.door_opener": true,
}

// Denver creates the example home with userID as its member and every DenverFact. Idempotent per fact.
func Denver(ctx context.Context, q *db.Queries, userID int64) (db.Home, error) {
	home, err := q.CreateHome(ctx, "1201 Newton St")
	if err != nil {
		return home, err
	}
	if _, err := q.CreateMember(ctx, db.CreateMemberParams{HomeID: home.ID, UserID: userID}); err != nil {
		return home, err
	}
	for key, v := range DenverFacts {
		raw, err := json.Marshal(v)
		if err != nil {
			return home, err
		}
		if _, err := q.UpsertFact(ctx, db.UpsertFactParams{HomeID: home.ID, Key: key, Value: raw, Source: "seed"}); err != nil {
			return home, fmt.Errorf("fact %s: %w", key, err)
		}
	}
	return home, nil
}
