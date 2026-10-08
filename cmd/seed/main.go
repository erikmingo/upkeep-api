// Command seed resets the users table to a fixed, fake dataset: same rows every run.
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/erikmingo/upkeep-api/internal/config"
	"github.com/erikmingo/upkeep-api/internal/db"
	"github.com/erikmingo/upkeep-api/internal/seed"
	"github.com/mhiro2/seedling"
)

const users = 20

func main() {
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
	if err := db.Migrate(ctx, pool); err != nil {
		log.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "TRUNCATE users RESTART IDENTITY CASCADE"); err != nil {
		log.Fatal(err)
	}
	res, err := seed.Users().InsertManyE(ctx, pool, users, seedling.WithSeed(42), seed.Fake())
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("seeded %d users\n", res.Len())
}
