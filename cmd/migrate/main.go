// Command migrate runs goose against DATABASE_URL: up | down | status.
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/erikmingo/upkeep-api/internal/config"
	"github.com/erikmingo/upkeep-api/internal/db"
)

func main() {
	if len(os.Args) != 2 {
		log.Fatal("usage: migrate up|down|status")
	}
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	ctx := context.Background()
	pool, err := db.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()
	p, err := db.Migrator(pool)
	if err != nil {
		log.Fatal(err)
	}
	switch os.Args[1] {
	case "up":
		res, err := p.Up(ctx)
		if err != nil {
			log.Fatal(err)
		}
		for _, r := range res {
			fmt.Println("applied", r.Source.Path)
		}
	case "down":
		res, err := p.Down(ctx)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println("rolled back", res.Source.Path)
	case "status":
		st, err := p.Status(ctx)
		if err != nil {
			log.Fatal(err)
		}
		for _, s := range st {
			fmt.Printf("%-8s %s\n", s.State, s.Source.Path)
		}
	default:
		log.Fatal("usage: migrate up|down|status")
	}
}
