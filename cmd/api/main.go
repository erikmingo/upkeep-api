package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/erikmingo/upkeep-api/internal/config"
	"github.com/erikmingo/upkeep-api/internal/db"
)

type pinger interface {
	Ping(context.Context) error
}

func newMux(p pinger) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), time.Second)
		defer cancel()
		body := map[string]string{"status": "ok", "service": "upkeep-api", "db": "ok"}
		w.Header().Set("Content-Type", "application/json")
		if err := p.Ping(ctx); err != nil {
			log.Printf("health: db ping: %v", err)
			body["status"], body["db"] = "degraded", "unreachable"
			w.WriteHeader(http.StatusServiceUnavailable)
		}
		json.NewEncoder(w).Encode(body)
	})
	return mux
}

func run(ctx context.Context) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	pool, err := db.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	if err := db.Migrate(ctx, pool); err != nil {
		return err
	}
	srv := &http.Server{Addr: fmt.Sprintf(":%d", cfg.Port), Handler: newMux(pool)}

	errc := make(chan error, 1)
	go func() {
		log.Printf("listening on %s", srv.Addr)
		errc <- srv.ListenAndServe()
	}()

	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
	}
	log.Print("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}
