package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/erikmingo/upkeep-api/internal/db"
	"github.com/erikmingo/upkeep-api/internal/rules"
	"github.com/erikmingo/upkeep-api/internal/seed"
	"github.com/mhiro2/seedling/seedlingpgx"
)

type fakePinger struct{ err error }

func (f fakePinger) Ping(context.Context) error { return f.err }

func do(h http.Handler, method, path string, body string, headers ...string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestHealth(t *testing.T) {
	rec := do(New(Deps{Pinger: fakePinger{}}), "GET", "/health", "")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"db":"ok"`) {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	rec = do(New(Deps{Pinger: fakePinger{err: errors.New("down")}}), "GET", "/health", "")
	if rec.Code != 503 || !strings.Contains(rec.Body.String(), `"db":"unreachable"`) {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
}

type captureMailer struct {
	mu   sync.Mutex
	last string
}

func (c *captureMailer) Send(_ context.Context, _, _, text string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.last = text
	return nil
}

func (c *captureMailer) code() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return strings.Split(c.last, "\n")[2]
}

// signIn runs the magic-link flow for an email and returns the Authorization header pair.
func signIn(t *testing.T, h http.Handler, m *captureMailer, email string) []string {
	t.Helper()
	if rec := do(h, "POST", "/v1/auth/magic-link", `{"email":"`+email+`"}`); rec.Code != 202 {
		t.Fatalf("magic-link: code=%d %s", rec.Code, rec.Body.String())
	}
	rec := do(h, "POST", "/v1/auth/verify", `{"token":"`+m.code()+`"}`)
	if rec.Code != 200 {
		t.Fatalf("verify: code=%d %s", rec.Code, rec.Body.String())
	}
	var out struct {
		SessionToken string `json:"session_token"`
	}
	json.Unmarshal(rec.Body.Bytes(), &out)
	return []string{"Authorization", "Bearer " + out.SessionToken}
}

// testDeps returns a handler wired to a rolled-back transaction holding the Denver home.
func testDeps(t *testing.T) (Deps, db.Home) {
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
	return Deps{DB: tx, Pinger: fakePinger{}, Mailer: &captureMailer{}, Now: func() time.Time { return time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC) }}, home
}

func TestSessionAuth(t *testing.T) {
	d, _ := testDeps(t)
	h := New(d)
	m := d.Mailer.(*captureMailer)
	if rec := do(h, "GET", "/v1/home", ""); rec.Code != 401 {
		t.Fatalf("no bearer: code=%d", rec.Code)
	}
	if rec := do(h, "GET", "/v1/home", "", "Authorization", "Bearer nope"); rec.Code != 401 {
		t.Fatalf("bad bearer: code=%d", rec.Code)
	}
	if rec := do(h, "POST", "/v1/auth/magic-link", `{"email":"nope"}`); rec.Code != 400 {
		t.Fatalf("bad email: code=%d", rec.Code)
	}
	if rec := do(h, "POST", "/v1/auth/verify", `{"token":"nope"}`); rec.Code != 401 {
		t.Fatalf("bad code: code=%d", rec.Code)
	}

	// a brand-new address gets a user and a first home
	H := signIn(t, h, m, "new@example.com")
	rec := do(h, "GET", "/v1/home", "", H...)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"name":"My home"`) {
		t.Fatalf("new user home: code=%d %s", rec.Code, rec.Body.String())
	}
	// the emailed code is single-use
	if rec := do(h, "POST", "/v1/auth/verify", `{"token":"`+m.code()+`"}`); rec.Code != 401 {
		t.Fatalf("reuse: code=%d", rec.Code)
	}
	// logout kills the session
	if rec := do(h, "POST", "/v1/auth/logout", "", H...); rec.Code != 204 {
		t.Fatalf("logout: code=%d", rec.Code)
	}
	if rec := do(h, "GET", "/v1/home", "", H...); rec.Code != 401 {
		t.Fatalf("after logout: code=%d", rec.Code)
	}
}
