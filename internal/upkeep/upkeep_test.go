package upkeep

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/erikmingo/upkeep-api/internal/db"
	"github.com/erikmingo/upkeep-api/internal/rules"
	"github.com/erikmingo/upkeep-api/internal/seed"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/mhiro2/seedling/seedlingpgx"
)

func date(s string) time.Time { t, _ := time.Parse("2006-01-02", s); return t }

func TestClamp(t *testing.T) {
	s, e := "10-01", "11-15"
	cases := []struct{ in, want string }{
		{"2026-03-10", "2026-10-01"}, // before window → window start
		{"2026-10-20", "2026-10-20"}, // inside → unchanged
		{"2026-12-01", "2027-10-01"}, // after → next year's start
	}
	for _, c := range cases {
		if got := clamp(date(c.in), &s, &e); !got.Equal(date(c.want)) {
			t.Errorf("clamp(%s) = %s, want %s", c.in, got.Format("2006-01-02"), c.want)
		}
	}
	ws, we := "11-15", "01-15" // crosses new year
	if got := clamp(date("2026-12-20"), &ws, &we); !got.Equal(date("2026-12-20")) {
		t.Errorf("cross-year inside: got %s", got.Format("2006-01-02"))
	}
	if got := clamp(date("2026-02-01"), &ws, &we); !got.Equal(date("2026-11-15")) {
		t.Errorf("cross-year after: got %s", got.Format("2006-01-02"))
	}
}

func TestFill(t *testing.T) {
	facts := map[string]any{"furnace.filter_size": "16x25x1", "detectors.smoke_count": float64(3)}
	got := fill("Size {{furnace.filter_size}}, {{detectors.smoke_count}} smoke, {{nope}}.", facts)
	if got != "Size 16x25x1, 3 smoke, ?." {
		t.Fatalf("got %q", got)
	}
}

func testQueries(t *testing.T) (*db.Queries, int64) {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	pool, err := db.Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := db.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	tx := seedlingpgx.WithTx(t, pool)
	q := db.New(tx)
	if _, err := rules.Load(ctx, q); err != nil {
		t.Fatal(err)
	}
	u := seed.Users().InsertOne(t, tx, seed.Fake()).Root()
	home, err := seed.Denver(ctx, q, u.ID)
	if err != nil {
		t.Fatal(err)
	}
	return q, home.ID
}

func TestMaterializeDenverIsIdempotent(t *testing.T) {
	q, home := testQueries(t)
	ctx := context.Background()
	today := date("2026-10-09")

	created, archived, err := Materialize(ctx, q, home, today)
	if err != nil || created != 23 || archived != 0 {
		t.Fatalf("first: created=%d archived=%d err=%v", created, archived, err)
	}
	created, archived, err = Materialize(ctx, q, home, today)
	if err != nil || created != 0 || archived != 0 {
		t.Fatalf("second: created=%d archived=%d err=%v", created, archived, err)
	}
	tasks, _ := q.ListActiveTasks(ctx, home)
	if len(tasks) != 23 {
		t.Fatalf("active=%d", len(tasks))
	}
	for _, task := range tasks {
		if *task.RuleID == "furnace.filter.change" && task.Detail != "Size 16x25x1. Arrow points toward the furnace." {
			t.Fatalf("detail not filled: %q", task.Detail)
		}
	}
}

func TestFactChangeArchivesAndKeepsHistory(t *testing.T) {
	q, home := testQueries(t)
	ctx := context.Background()
	today := date("2026-10-09")
	if _, _, err := Materialize(ctx, q, home, today); err != nil {
		t.Fatal(err)
	}
	var flush db.Task
	for _, task := range mustActive(t, q, home) {
		if *task.RuleID == "water_heater.flush" {
			flush = task
		}
	}
	members, _ := q.ListMembers(ctx, home)
	if _, err := q.CreateCompletion(ctx, db.CreateCompletionParams{TaskID: flush.ID, MemberID: members[0].ID, CompletedAt: pgtype.Timestamptz{Time: today, Valid: true}}); err != nil {
		t.Fatal(err)
	}
	hand, _ := q.CreateTask(ctx, db.CreateTaskParams{HomeID: home, Title: "Fix the gate latch", IntervalDays: 365, StartDate: pgtype.Date{Time: today, Valid: true}})

	raw, _ := json.Marshal("tankless")
	if _, err := q.UpsertFact(ctx, db.UpsertFactParams{HomeID: home, Key: "water_heater.type", Value: raw, Source: "user"}); err != nil {
		t.Fatal(err)
	}
	created, archived, err := Materialize(ctx, q, home, today)
	if err != nil || created != 1 || archived != 2 { // +tankless.descale, -flush, -anode
		t.Fatalf("created=%d archived=%d err=%v", created, archived, err)
	}
	ids := map[string]bool{}
	handStillThere := false
	for _, task := range mustActive(t, q, home) {
		if task.RuleID != nil {
			ids[*task.RuleID] = true
		} else if task.ID == hand.ID {
			handStillThere = true
		}
	}
	if ids["water_heater.flush"] || ids["water_heater.anode"] || !ids["tankless.descale"] || !handStillThere {
		t.Fatalf("active rule ids=%v hand=%v", ids, handStillThere)
	}
	archivedTask, _ := q.GetTask(ctx, flush.ID)
	if !archivedTask.ArchivedAt.Valid {
		t.Fatal("flush not archived")
	}
	all, _ := q.LatestCompletions(ctx, home)
	if len(all) != 1 || all[0].TaskID != flush.ID {
		t.Fatalf("completion history lost: %+v", all)
	}
}

func TestListByNextDue(t *testing.T) {
	q, home := testQueries(t)
	ctx := context.Background()
	today := date("2026-10-09")
	if _, _, err := Materialize(ctx, q, home, today); err != nil {
		t.Fatal(err)
	}
	members, _ := q.ListMembers(ctx, home)
	byRule := map[string]db.Task{}
	for _, task := range mustActive(t, q, home) {
		byRule[*task.RuleID] = task
	}
	// completed 100 days ago with a 90-day interval → overdue by 10 days
	filter := byRule["furnace.filter.change"]
	if _, err := q.CreateCompletion(ctx, db.CreateCompletionParams{TaskID: filter.ID, MemberID: members[0].ID, CompletedAt: pgtype.Timestamptz{Time: today.AddDate(0, 0, -100), Valid: true}}); err != nil {
		t.Fatal(err)
	}

	due, err := ListByNextDue(ctx, q, home, today)
	if err != nil || len(due) != 23 {
		t.Fatalf("n=%d err=%v", len(due), err)
	}
	if due[0].Task.ID != filter.ID || !due[0].Overdue || !due[0].NextDue.Equal(date("2026-09-29")) {
		t.Fatalf("first=%s overdue=%v next=%s", *due[0].Task.RuleID, due[0].Overdue, due[0].NextDue.Format("2006-01-02"))
	}
	for _, d := range due {
		switch *d.Task.RuleID {
		case "roof.inspect": // no completion, no season → due on start date, not overdue
			if !d.NextDue.Equal(today) || d.Overdue {
				t.Errorf("roof: next=%s overdue=%v", d.NextDue.Format("2006-01-02"), d.Overdue)
			}
		case "sprinkler.startup": // start date Oct 9 is outside Apr 15–May 31 → next year's window
			if !d.NextDue.Equal(date("2027-04-15")) {
				t.Errorf("sprinkler startup: next=%s", d.NextDue.Format("2006-01-02"))
			}
		case "sprinkler.blowout": // Oct 9 is inside Oct 1–Nov 15 → unchanged
			if !d.NextDue.Equal(today) {
				t.Errorf("blowout: next=%s", d.NextDue.Format("2006-01-02"))
			}
		}
	}
	for i := 1; i < len(due); i++ {
		if due[i].NextDue.Before(due[i-1].NextDue) {
			t.Fatal("not sorted")
		}
	}
}

func mustActive(t *testing.T, q *db.Queries, home int64) []db.Task {
	t.Helper()
	tasks, err := q.ListActiveTasks(context.Background(), home)
	if err != nil {
		t.Fatal(err)
	}
	return tasks
}
