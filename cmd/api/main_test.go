package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type fakePinger struct{ err error }

func (f fakePinger) Ping(context.Context) error { return f.err }

func health(t *testing.T, p pinger) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	newMux(p).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/health", nil))
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("content-type = %q", ct)
	}
	return rec
}

func TestHealth(t *testing.T) {
	rec := health(t, fakePinger{})
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"db":"ok"`) {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestHealthDBDown(t *testing.T) {
	rec := health(t, fakePinger{err: errors.New("connection refused")})
	if rec.Code != http.StatusServiceUnavailable || !strings.Contains(rec.Body.String(), `"status":"degraded"`) {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
}
