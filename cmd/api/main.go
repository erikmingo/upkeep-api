package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/erikmingo/upkeep-api/internal/api"
	"github.com/erikmingo/upkeep-api/internal/config"
	"github.com/erikmingo/upkeep-api/internal/db"
	"github.com/erikmingo/upkeep-api/internal/rules"
)

func run(ctx context.Context) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if cfg.Env == "production" {
		// the only auth today is the dev X-Home header, which must never run in production
		return errors.New("no production authentication exists yet; refusing to start with ENV=production")
	}
	pool, err := db.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	if err := db.Migrate(ctx, pool); err != nil {
		return err
	}
	q := db.New(pool)
	if _, err := rules.Load(ctx, q); err != nil {
		return err
	}
	srv := &http.Server{Addr: fmt.Sprintf(":%d", cfg.Port), Handler: api.New(api.Deps{DB: pool, Pinger: pool, DevHomeHeader: true})}

	errc := make(chan error, 1)
	go func() {
		log.Printf("listening on %s (env=%s, auth=X-Home header)", srv.Addr, cfg.Env)
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
