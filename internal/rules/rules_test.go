package rules

import (
	"context"
	"encoding/json"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/erikmingo/upkeep-api/internal/db"
	"github.com/erikmingo/upkeep-api/internal/seed"
	"github.com/mhiro2/seedling/seedlingpgx"
)

func TestLibraryParses(t *testing.T) {
	rs, err := Parse()
	if err != nil {
		t.Fatal(err)
	}
	if len(rs) < 40 {
		t.Fatalf("only %d rules", len(rs))
	}
}

func TestParseRejectsBadRules(t *testing.T) {
	cases := map[string]string{
		"unknown key":   "- id: a\n  when: { nope.key: [x] }\n  task: { title: t, interval: 1 }\n",
		"bad enum":      "- id: a\n  when: { furnace.type: [nuclear] }\n  task: { title: t, interval: 1 }\n",
		"bool as str":   "- id: a\n  when: { gutters.present: [yes] }\n  task: { title: t, interval: 1 }\n",
		"duplicate id":  "- id: a\n  when: {}\n  task: { title: t, interval: 1 }\n- id: a\n  when: {}\n  task: { title: t, interval: 1 }\n",
		"zero interval": "- id: a\n  when: {}\n  task: { title: t, interval: 0 }\n",
	}
	for name, src := range cases {
		if _, err := parse([]byte(src)); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
}

func denverFacts(t *testing.T) Facts {
	t.Helper()
	var rows []db.Fact
	for k, v := range seed.DenverFacts {
		raw, _ := json.Marshal(v)
		rows = append(rows, db.Fact{Key: k, Value: raw})
	}
	f, err := FactsFrom(rows)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

// The spec's worked example, as the library actually computes it.
var denverRules = []string{
	"furnace.filter.change", "furnace.service", "ac.coil.clean",
	"water_heater.flush", "water_heater.anode",
	"roof.inspect", "gutters.clean.spring", "gutters.clean.fall", "deck.seal.wood",
	"sprinkler.blowout", "sprinkler.startup", "tree.trim", "lawn.aerate",
	"detectors.batteries", "detectors.co.replace", "detectors.smoke.replace", "dryer.vent.clean.long", "radon.test",
	"dishwasher.filter", "disposal.clean", "range_hood.filter",
	"garage.door.balance", "garage.door.lube",
}

func TestDenverMatchesExactly(t *testing.T) {
	rs, err := Parse()
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, r := range Matching(rs, denverFacts(t)) {
		got = append(got, r.ID)
	}
	want := slices.Clone(denverRules)
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Fatalf("\ngot  %s\nwant %s", strings.Join(got, " "), strings.Join(want, " "))
	}
}

func TestLoadUpserts(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	pool, err := db.Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close) // registered before WithTx so the rollback runs first
	if err := db.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	tx := seedlingpgx.WithTx(t, pool)
	q := db.New(tx)
	rs, err := Load(ctx, q)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Load(ctx, q); err != nil {
		t.Fatal("second load:", err)
	}
	rows, err := q.ListRules(ctx)
	if err != nil || len(rows) != len(rs) {
		t.Fatalf("rows=%d want %d err=%v", len(rows), len(rs), err)
	}
}
