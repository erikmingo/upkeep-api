// Command seed resets users and homes to a fixed dataset: 20 fake users and the Denver example home.
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/erikmingo/upkeep-api/internal/config"
	"github.com/erikmingo/upkeep-api/internal/db"
	"github.com/erikmingo/upkeep-api/internal/rules"
	"github.com/erikmingo/upkeep-api/internal/seed"
	"github.com/erikmingo/upkeep-api/internal/upkeep"
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
	rs, err := rules.Load(ctx, db.New(pool))
	if err != nil {
		log.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "TRUNCATE users, homes RESTART IDENTITY CASCADE"); err != nil {
		log.Fatal(err)
	}
	res, err := seed.Users().InsertManyE(ctx, pool, users, seedling.WithSeed(42), seed.Fake())
	if err != nil {
		log.Fatal(err)
	}
	home, err := seed.Denver(ctx, db.New(pool), res.MustRootAt(0).ID)
	if err != nil {
		log.Fatal(err)
	}
	created, _, err := upkeep.Materialize(ctx, db.New(pool), home.ID, time.Now())
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("loaded %d rules; seeded %d users; home %d %q with %d facts and %d tasks\n", len(rs), res.Len(), home.ID, home.Name, len(seed.DenverFacts), created)
}
