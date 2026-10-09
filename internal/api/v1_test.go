package api

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/erikmingo/upkeep-api/internal/db"
)

type tasksBody struct {
	Tasks []struct {
		ID            int64   `json:"id"`
		RuleID        *string `json:"rule_id"`
		Title         string  `json:"title"`
		NextDue       string  `json:"next_due"`
		Overdue       bool    `json:"overdue"`
		LastCompleted *string `json:"last_completed"`
	} `json:"tasks"`
}

func TestFactsToTasksFlow(t *testing.T) {
	d, home := testDeps(t)
	h := New(d)
	m := d.Mailer.(*captureMailer)
	// sign in as the Denver home's seeded member
	members, _ := db.New(d.DB).ListMembers(context.Background(), home.ID)
	H := signIn(t, h, m, members[0].Email)

	// home shows the seeded facts and the member
	rec := do(h, "GET", "/v1/home", "", H...)
	var homeBody struct {
		ID      int64                      `json:"id"`
		Facts   map[string]json.RawMessage `json:"facts"`
		Members []struct{ Email string }   `json:"members"`
	}
	json.Unmarshal(rec.Body.Bytes(), &homeBody)
	if rec.Code != 200 || homeBody.ID != home.ID || len(homeBody.Facts) != 35 || len(homeBody.Members) != 1 || string(homeBody.Facts["furnace.type"]) != `"gas"` {
		t.Fatalf("code=%d facts=%d members=%d", rec.Code, len(homeBody.Facts), len(homeBody.Members))
	}

	// no tasks until facts are put
	rec = do(h, "GET", "/v1/tasks", "", H...)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"tasks":[]`) {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}

	// bad facts are rejected before anything is written
	for _, body := range []string{`{}`, `{"facts":{"nope.key":1}}`, `{"facts":{"furnace.type":"nuclear"}}`, `{"facts":{"gutters.present":"yes"}}`, `{"facts":{"home.beds":2.5}}`} {
		if rec = do(h, "PUT", "/v1/home/facts", body, H...); rec.Code != 400 {
			t.Fatalf("%s: code=%d %s", body, rec.Code, rec.Body.String())
		}
	}

	// a valid put re-materializes: Denver facts (unchanged) → 23 tasks
	rec = do(h, "PUT", "/v1/home/facts", `{"facts":{"home.beds":2}}`, H...)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"tasks_created":23`) {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	rec = do(h, "GET", "/v1/tasks", "", H...)
	var tb tasksBody
	json.Unmarshal(rec.Body.Bytes(), &tb)
	if len(tb.Tasks) != 23 {
		t.Fatalf("tasks=%d", len(tb.Tasks))
	}
	var filter int64
	for _, x := range tb.Tasks {
		if x.RuleID != nil && *x.RuleID == "furnace.filter.change" {
			filter = x.ID
		}
		if x.Overdue {
			t.Fatalf("fresh task overdue: %+v", x)
		}
	}

	// completing moves next-due by the interval and records who
	rec = do(h, "POST", fmt.Sprintf("/v1/tasks/%d/complete", filter), `{}`, H...)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"next_due":"2027-01-07"`) || !strings.Contains(rec.Body.String(), `"last_completed":"2026-10-09"`) {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	if rec = do(h, "POST", "/v1/tasks/999999/complete", `{}`, H...); rec.Code != 404 {
		t.Fatalf("unknown task: code=%d", rec.Code)
	}

	// hand-added task
	rec = do(h, "POST", "/v1/tasks", `{"title":"Fix the gate latch","interval_days":365}`, H...)
	if rec.Code != 201 || !strings.Contains(rec.Body.String(), `"rule_id":null`) || !strings.Contains(rec.Body.String(), `"next_due":"2026-10-09"`) {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	for _, body := range []string{`{"title":"x"}`, `{"interval_days":5}`, `{"title":"x","interval_days":5,"season_start":"10-01"}`} {
		if rec = do(h, "POST", "/v1/tasks", body, H...); rec.Code != 400 {
			t.Fatalf("%s: code=%d", body, rec.Code)
		}
	}

	// changing a fact archives tasks whose rule stopped matching; the hand task stays
	rec = do(h, "PUT", "/v1/home/facts", `{"facts":{"water_heater.type":"tankless"}}`, H...)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"tasks_created":1`) || !strings.Contains(rec.Body.String(), `"tasks_archived":2`) {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	rec = do(h, "GET", "/v1/tasks", "", H...)
	json.Unmarshal(rec.Body.Bytes(), &tb)
	if len(tb.Tasks) != 23 { // 23 − 2 + 1 + 1 hand
		t.Fatalf("tasks=%d", len(tb.Tasks))
	}

	// another user (own fresh home) cannot complete this home's task
	other := signIn(t, h, m, "other@example.com")
	rec = do(h, "POST", fmt.Sprintf("/v1/tasks/%d/complete", filter), `{}`, other...)
	if rec.Code != 404 {
		t.Fatalf("cross-home: code=%d", rec.Code)
	}
}
