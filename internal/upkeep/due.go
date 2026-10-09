package upkeep

import (
	"context"
	"sort"
	"time"

	"github.com/erikmingo/upkeep-api/internal/db"
)

type Due struct {
	Task          db.Task
	LastCompleted *time.Time
	NextDue       time.Time
	Overdue       bool
}

// ListByNextDue returns the home's active tasks with their next-due date, soonest first.
func ListByNextDue(ctx context.Context, q *db.Queries, homeID int64, today time.Time) ([]Due, error) {
	tasks, err := q.ListActiveTasks(ctx, homeID)
	if err != nil {
		return nil, err
	}
	latest, err := q.LatestCompletions(ctx, homeID)
	if err != nil {
		return nil, err
	}
	last := map[int64]time.Time{}
	for _, c := range latest {
		last[c.TaskID] = c.CompletedAt.Time
	}
	today = day(today)
	out := make([]Due, 0, len(tasks))
	for _, t := range tasks {
		d := Due{Task: t}
		base := day(t.StartDate.Time)
		if at, ok := last[t.ID]; ok {
			at = day(at)
			d.LastCompleted = &at
			base = at.AddDate(0, 0, int(t.IntervalDays))
		}
		d.NextDue = clamp(base, t.SeasonStart, t.SeasonEnd)
		d.Overdue = d.NextDue.Before(today)
		out = append(out, d)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].NextDue.Before(out[j].NextDue) })
	return out, nil
}

// ponytail: due dates are UTC calendar days; a per-home time zone can replace this when it matters
func day(t time.Time) time.Time {
	t = t.UTC()
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

// clamp moves a date forward into the next MM-DD season window when it falls outside one.
// Windows that cross New Year (e.g. 11-15 → 01-15) are handled by testing both year offsets.
func clamp(d time.Time, start, end *string) time.Time {
	if start == nil || end == nil {
		return d
	}
	sm, sd := mmdd(*start)
	em, ed := mmdd(*end)
	for yearOffset := -1; yearOffset <= 1; yearOffset++ {
		s := time.Date(d.Year()+yearOffset, sm, sd, 0, 0, 0, 0, time.UTC)
		e := time.Date(d.Year()+yearOffset, em, ed, 0, 0, 0, 0, time.UTC)
		if e.Before(s) {
			e = e.AddDate(1, 0, 0)
		}
		if !d.Before(s) && !d.After(e) {
			return d
		}
		if d.Before(s) {
			return s
		}
	}
	return d
}

func mmdd(s string) (time.Month, int) {
	t, err := time.Parse("01-02", s)
	if err != nil {
		return time.January, 1
	}
	return t.Month(), t.Day()
}
