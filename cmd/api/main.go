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
	"github.com/erikmingo/upkeep-api/internal/mail"
	"github.com/erikmingo/upkeep-api/internal/rules"
)

func run(ctx context.Context) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	var mailer mail.Mailer = mail.Log{}
	if cfg.ResendKey != "" {
		mailer = mail.Resend{APIKey: cfg.ResendKey, From: cfg.MailFrom}
	} else if cfg.Env == "production" {
		return errors.New("RESEND_API_KEY is required in production; sign-in codes cannot go to the log")
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
	srv := &http.Server{Addr: fmt.Sprintf(":%d", cfg.Port), Handler: api.New(api.Deps{DB: pool, Pinger: pool, Mailer: mailer})}

	errc := make(chan error, 1)
	go func() {
		log.Printf("listening on %s (env=%s, mailer=%T)", srv.Addr, cfg.Env, mailer)
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
