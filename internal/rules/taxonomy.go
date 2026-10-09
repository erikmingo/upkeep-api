package rules

// Taxonomy is the v1 fact list from docs/spec-home-facts.md. A rule may only match on keys
// listed here; enum keys list their values, other keys list the Go kind a value must have.
var Taxonomy = map[string][]any{
	"home.type": {"house", "condo", "townhouse"}, "home.year_built": {kindInt}, "home.stories": {kindInt},
	"home.sqft": {kindInt}, "home.beds": {kindInt}, "home.baths": {kindInt},
	"furnace.type": {"gas", "electric", "heat_pump", "boiler", "none"}, "furnace.filter_size": {kindString},
	"ac.type": {"central", "window", "mini_split", "none"}, "humidifier.present": {kindBool},
	"water_heater.type": {"tank", "tankless", "none"}, "water_heater.installed": {kindInt},
	"softener.present": {kindBool}, "water.source": {"city", "well"}, "sewer.type": {"sewer", "septic"}, "sump_pump.present": {kindBool},
	"roof.material": {"asphalt", "metal", "tile", "flat"}, "roof.installed": {kindInt},
	"gutters.present": {kindBool}, "gutters.guards": {kindBool}, "deck.material": {"wood", "composite", "none"},
	"lawn.present": {kindBool}, "sprinkler.present": {kindBool}, "trees.near_house": {kindBool}, "pool.type": {"none", "pool", "hot_tub"},
	"detectors.smoke_count": {kindInt}, "detectors.co_count": {kindInt}, "extinguisher.present": {kindBool},
	"dryer.vent_length": {"short", "long"}, "radon.last_test": {kindInt},
	"fridge.water_filter": {kindBool}, "dishwasher.present": {kindBool}, "disposal.present": {kindBool}, "range_hood.present": {kindBool},
	"garage.present": {kindBool}, "garage.door_opener": {kindBool},
}

type kind int

const (
	kindInt kind = iota
	kindBool
	kindString
)
