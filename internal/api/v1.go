package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/erikmingo/upkeep-api/internal/db"
	"github.com/erikmingo/upkeep-api/internal/rules"
	"github.com/erikmingo/upkeep-api/internal/upkeep"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func (d Deps) routes(v1 *http.ServeMux) {
	v1.HandleFunc("GET /home", d.getHome)
	v1.HandleFunc("PUT /home/facts", d.putFacts)
	v1.HandleFunc("GET /tasks", d.listTasks)
	v1.HandleFunc("POST /tasks", d.createTask)
	v1.HandleFunc("POST /tasks/{id}/complete", d.completeTask)
}

type homeOut struct {
	ID      int64                      `json:"id"`
	Name    string                     `json:"name"`
	Facts   map[string]json.RawMessage `json:"facts"`
	Members []memberOut                `json:"members"`
}

type memberOut struct {
	ID          int64  `json:"id"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
}

func (d Deps) getHome(w http.ResponseWriter, r *http.Request) {
	ctx, home, q := r.Context(), HomeFrom(r.Context()), db.New(d.DB)
	facts, err := q.ListFacts(ctx, home.ID)
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	members, err := q.ListMembers(ctx, home.ID)
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	out := homeOut{ID: home.ID, Name: home.Name, Facts: map[string]json.RawMessage{}, Members: []memberOut{}}
	for _, f := range facts {
		out.Facts[f.Key] = json.RawMessage(f.Value)
	}
	for _, m := range members {
		out.Members = append(out.Members, memberOut{ID: m.ID, Email: m.Email, DisplayName: m.DisplayName})
	}
	writeJSON(w, 200, out)
}

// putFacts upserts the given facts and re-materializes tasks, atomically.
func (d Deps) putFacts(w http.ResponseWriter, r *http.Request) {
	ctx, home := r.Context(), HomeFrom(r.Context())
	var in struct {
		Facts map[string]json.RawMessage `json:"facts"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil || len(in.Facts) == 0 {
		writeError(w, 400, `body must be {"facts": {"key": value, ...}}`)
		return
	}
	for key, raw := range in.Facts {
		var v any
		if err := json.Unmarshal(raw, &v); err != nil {
			writeError(w, 400, key+": not JSON")
			return
		}
		if err := rules.ValidateFact(key, v); err != nil {
			writeError(w, 400, err.Error())
			return
		}
	}
	tx, err := d.DB.Begin(ctx)
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	defer tx.Rollback(ctx)
	q := db.New(tx)
	for key, raw := range in.Facts {
		if _, err := q.UpsertFact(ctx, db.UpsertFactParams{HomeID: home.ID, Key: key, Value: raw, Source: "user"}); err != nil {
			writeError(w, 500, err.Error())
			return
		}
	}
	created, archived, err := upkeep.Materialize(ctx, q, home.ID, d.now())
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	if err := tx.Commit(ctx); err != nil {
		writeError(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]int{"facts": len(in.Facts), "tasks_created": created, "tasks_archived": archived})
}

type taskOut struct {
	ID            int64   `json:"id"`
	RuleID        *string `json:"rule_id"`
	Title         string  `json:"title"`
	Detail        string  `json:"detail"`
	IntervalDays  int32   `json:"interval_days"`
	SeasonStart   *string `json:"season_start"`
	SeasonEnd     *string `json:"season_end"`
	NextDue       string  `json:"next_due"`
	Overdue       bool    `json:"overdue"`
	LastCompleted *string `json:"last_completed"`
}

func toOut(d upkeep.Due) taskOut {
	o := taskOut{
		ID: d.Task.ID, RuleID: d.Task.RuleID, Title: d.Task.Title, Detail: d.Task.Detail, IntervalDays: d.Task.IntervalDays,
		SeasonStart: d.Task.SeasonStart, SeasonEnd: d.Task.SeasonEnd, NextDue: d.NextDue.Format(time.DateOnly), Overdue: d.Overdue,
	}
	if d.LastCompleted != nil {
		s := d.LastCompleted.Format(time.DateOnly)
		o.LastCompleted = &s
	}
	return o
}

func (d Deps) listTasks(w http.ResponseWriter, r *http.Request) {
	due, err := upkeep.ListByNextDue(r.Context(), db.New(d.DB), HomeFrom(r.Context()).ID, d.now())
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	out := make([]taskOut, 0, len(due))
	for _, x := range due {
		out = append(out, toOut(x))
	}
	writeJSON(w, 200, map[string]any{"tasks": out})
}

func (d Deps) createTask(w http.ResponseWriter, r *http.Request) {
	ctx, home := r.Context(), HomeFrom(r.Context())
	var in struct {
		Title        string  `json:"title"`
		Detail       string  `json:"detail"`
		IntervalDays int32   `json:"interval_days"`
		SeasonStart  *string `json:"season_start"`
		SeasonEnd    *string `json:"season_end"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil || in.Title == "" || in.IntervalDays <= 0 {
		writeError(w, 400, "title and interval_days > 0 are required")
		return
	}
	if (in.SeasonStart == nil) != (in.SeasonEnd == nil) || (in.SeasonStart != nil && (len(*in.SeasonStart) != 5 || len(*in.SeasonEnd) != 5)) {
		writeError(w, 400, "season_start and season_end must both be MM-DD or both absent")
		return
	}
	task, err := db.New(d.DB).CreateTask(ctx, db.CreateTaskParams{
		HomeID: home.ID, Title: in.Title, Detail: in.Detail, IntervalDays: in.IntervalDays,
		SeasonStart: in.SeasonStart, SeasonEnd: in.SeasonEnd, StartDate: pgtype.Date{Time: d.now(), Valid: true},
	})
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	writeJSON(w, 201, toOut(upkeep.Due{Task: task, NextDue: task.StartDate.Time}))
}

func (d Deps) completeTask(w http.ResponseWriter, r *http.Request) {
	ctx, home, q := r.Context(), HomeFrom(r.Context()), db.New(d.DB)
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, 404, "no such task")
		return
	}
	task, err := q.GetTask(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && (task.HomeID != home.ID || task.ArchivedAt.Valid)) {
		writeError(w, 404, "no such task")
		return
	}
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	memberID := IdentityFrom(ctx).MemberID
	if _, err := q.CreateCompletion(ctx, db.CreateCompletionParams{TaskID: task.ID, MemberID: memberID, CompletedAt: pgtype.Timestamptz{Time: d.now(), Valid: true}}); err != nil {
		writeError(w, 500, err.Error())
		return
	}
	due, err := upkeep.ListByNextDue(ctx, q, home.ID, d.now())
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	for _, x := range due {
		if x.Task.ID == task.ID {
			writeJSON(w, 200, toOut(x))
			return
		}
	}
	writeError(w, 500, "task vanished")
}
