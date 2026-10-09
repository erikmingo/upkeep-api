// Package api is the HTTP surface: routes, middleware, JSON helpers.
package api

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/erikmingo/upkeep-api/internal/db"
	"github.com/jackc/pgx/v5"
)

type Pinger interface {
	Ping(context.Context) error
}

type Deps struct {
	Queries *db.Queries
	Pinger  Pinger
	// DevHomeHeader identifies the caller by the X-Home header. Dev only; never true in production.
	DevHomeHeader bool
}

func New(d Deps) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", health(d.Pinger))
	v1 := http.NewServeMux()
	// v1 routes register here (#19)
	mux.Handle("/v1/", http.StripPrefix("/v1", d.auth(v1)))
	return mux
}

func (d Deps) auth(next http.Handler) http.Handler {
	if d.DevHomeHeader {
		return homeFromHeader(d.Queries, next)
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusUnauthorized, "no authentication configured")
	})
}

type ctxKey struct{}

// HomeFrom returns the caller's home, set by the auth middleware.
func HomeFrom(ctx context.Context) db.Home { return ctx.Value(ctxKey{}).(db.Home) }

func homeFromHeader(q *db.Queries, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.Header.Get("X-Home"), 10, 64)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "X-Home header required (dev)")
			return
		}
		home, err := q.GetHome(r.Context(), id)
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "no such home")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "lookup failed")
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxKey{}, home)))
	})
}

func health(p Pinger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), time.Second)
		defer cancel()
		body := map[string]string{"status": "ok", "service": "upkeep-api", "db": "ok"}
		code := http.StatusOK
		if err := p.Ping(ctx); err != nil {
			log.Printf("health: db ping: %v", err)
			body["status"], body["db"] = "degraded", "unreachable"
			code = http.StatusServiceUnavailable
		}
		writeJSON(w, code, body)
	}
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}
