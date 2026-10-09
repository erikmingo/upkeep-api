package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
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

func TestV1WithoutAuthConfigured(t *testing.T) {
	rec := do(New(Deps{Pinger: fakePinger{}}), "GET", "/v1/anything", "")
	if rec.Code != 401 {
		t.Fatalf("code=%d", rec.Code)
	}
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
	return Deps{DB: tx, Pinger: fakePinger{}, DevHomeHeader: true, Now: func() time.Time { return time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC) }}, home
}

func TestHomeHeader(t *testing.T) {
	d, home := testDeps(t)
	h := New(d)
	if rec := do(h, "GET", "/v1/home", ""); rec.Code != 401 {
		t.Fatalf("missing header: code=%d", rec.Code)
	}
	if rec := do(h, "GET", "/v1/home", "", "X-Home", "999999"); rec.Code != 404 {
		t.Fatalf("unknown home: code=%d", rec.Code)
	}
	if rec := do(h, "GET", "/v1/home", "", "X-Home", "abc"); rec.Code != 401 {
		t.Fatalf("garbage header: code=%d", rec.Code)
	}
	rec := do(h, "GET", "/v1/home", "", "X-Home", strconv.FormatInt(home.ID, 10))
	if rec.Code != 200 {
		t.Fatalf("known home: code=%d %s", rec.Code, rec.Body.String())
	}
}
