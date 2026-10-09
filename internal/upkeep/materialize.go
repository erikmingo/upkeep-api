// Package upkeep turns a home's facts into its task list and computes when each task is due.
package upkeep

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/erikmingo/upkeep-api/internal/db"
	"github.com/erikmingo/upkeep-api/internal/rules"
	"github.com/jackc/pgx/v5/pgtype"
)

// Materialize ensures one active task per rule the home's facts satisfy, archives active
// rule-backed tasks whose rule no longer matches, and leaves hand-added tasks alone. Idempotent.
func Materialize(ctx context.Context, q *db.Queries, homeID int64, today time.Time) (created, archived int, err error) {
	library, err := rules.Parse()
	if err != nil {
		return 0, 0, err
	}
	rows, err := q.ListFacts(ctx, homeID)
	if err != nil {
		return 0, 0, err
	}
	facts, err := rules.FactsFrom(rows)
	if err != nil {
		return 0, 0, err
	}
	active, err := q.ListActiveTasks(ctx, homeID)
	if err != nil {
		return 0, 0, err
	}
	have := map[string]bool{}
	for _, t := range active {
		if t.RuleID != nil {
			have[*t.RuleID] = true
		}
	}
	want := map[string]bool{}
	for _, r := range rules.Matching(library, facts) {
		want[r.ID] = true
		if have[r.ID] {
			continue
		}
		id := r.ID
		var start, end *string
		if r.Task.Season != nil {
			start, end = &r.Task.Season.Start, &r.Task.Season.End
		}
		_, err := q.CreateTask(ctx, db.CreateTaskParams{
			HomeID: homeID, RuleID: &id, Title: r.Task.Title, Detail: fill(r.Task.Detail, facts),
			IntervalDays: r.Task.Interval, SeasonStart: start, SeasonEnd: end,
			StartDate: pgtype.Date{Time: today, Valid: true},
		})
		if err != nil {
			return created, archived, fmt.Errorf("task %s: %w", r.ID, err)
		}
		created++
	}
	for _, t := range active {
		if t.RuleID != nil && !want[*t.RuleID] {
			if err := q.ArchiveTask(ctx, t.ID); err != nil {
				return created, archived, err
			}
			archived++
		}
	}
	return created, archived, nil
}

// fill replaces {{fact.key}} placeholders with the fact's value; unknown keys become "?".
func fill(detail string, facts rules.Facts) string {
	for strings.Contains(detail, "{{") {
		i := strings.Index(detail, "{{")
		j := strings.Index(detail[i:], "}}")
		if j < 0 {
			break
		}
		key := strings.TrimSpace(detail[i+2 : i+j])
		val := "?"
		if v, ok := facts[key]; ok {
			val = fmt.Sprint(v)
			if f, isNum := v.(float64); isNum && f == float64(int64(f)) {
				val = fmt.Sprint(int64(f))
			}
		}
		detail = detail[:i] + val + detail[i+j+2:]
	}
	return detail
}
