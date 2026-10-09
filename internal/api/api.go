// Package api is the HTTP surface: routes, middleware, JSON helpers.
package api

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/erikmingo/upkeep-api/internal/auth"
	"github.com/erikmingo/upkeep-api/internal/db"
	"github.com/erikmingo/upkeep-api/internal/mail"
	"github.com/jackc/pgx/v5"
)

type Pinger interface {
	Ping(context.Context) error
}

// DB is a pool or a transaction: anything sqlc can query that can also open a transaction.
type DB interface {
	db.DBTX
	Begin(context.Context) (pgx.Tx, error)
}

type Deps struct {
	DB     DB
	Pinger Pinger
	Mailer mail.Mailer
	// Now overrides the clock in tests.
	Now func() time.Time
}

func (d Deps) now() time.Time {
	if d.Now != nil {
		return d.Now()
	}
	return time.Now()
}

func New(d Deps) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", health(d.Pinger))
	mux.HandleFunc("POST /v1/auth/magic-link", d.magicLink)
	mux.HandleFunc("POST /v1/auth/verify", d.verify)
	mux.HandleFunc("GET /v1/auth/verify", d.verify)
	mux.HandleFunc("POST /v1/auth/logout", d.logout)
	v1 := http.NewServeMux()
	d.routes(v1)
	mux.Handle("/v1/", http.StripPrefix("/v1", d.session(v1)))
	return mux
}

type ctxKey struct{}

// IdentityFrom returns the caller (user, home, member), set by the session middleware.
func IdentityFrom(ctx context.Context) auth.Identity { return ctx.Value(ctxKey{}).(auth.Identity) }

// HomeFrom is the caller's home.
func HomeFrom(ctx context.Context) db.Home { return IdentityFrom(ctx).Home }

func bearer(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if len(h) > 7 && strings.EqualFold(h[:7], "bearer ") {
		return strings.TrimSpace(h[7:])
	}
	return ""
}

func (d Deps) session(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := bearer(r)
		if token == "" {
			writeError(w, http.StatusUnauthorized, "sign in required")
			return
		}
		id, err := auth.Resolve(r.Context(), db.New(d.DB), token, d.now())
		if errors.Is(err, auth.ErrInvalidToken) {
			writeError(w, http.StatusUnauthorized, "session expired, sign in again")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "lookup failed")
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxKey{}, id)))
	})
}

func (d Deps) magicLink(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil || !strings.Contains(in.Email, "@") {
		writeError(w, http.StatusBadRequest, "email required")
		return
	}
	if err := auth.RequestLink(r.Context(), db.New(d.DB), d.Mailer, in.Email, d.now()); err != nil {
		log.Printf("magic-link: %v", err)
		writeError(w, http.StatusInternalServerError, "could not send")
		return
	}
	// same answer whether or not the address is known
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "sent"})
}

func (d Deps) verify(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if r.Method == http.MethodPost {
		var in struct {
			Token string `json:"token"`
		}
		_ = json.NewDecoder(r.Body).Decode(&in)
		token = in.Token
	}
	s, err := auth.Verify(r.Context(), db.New(d.DB), strings.TrimSpace(token), d.now())
	if errors.Is(err, auth.ErrInvalidToken) {
		writeError(w, http.StatusUnauthorized, "that code is invalid or expired; request a new one")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"session_token": s.SessionToken,
		"user":          map[string]any{"id": s.User.ID, "email": s.User.Email, "display_name": s.User.DisplayName},
		"home":          map[string]any{"id": s.Home.ID, "name": s.Home.Name},
	})
}

func (d Deps) logout(w http.ResponseWriter, r *http.Request) {
	if t := bearer(r); t != "" {
		_ = db.New(d.DB).DeleteSession(r.Context(), auth.Hash(t))
	}
	w.WriteHeader(http.StatusNoContent)
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
