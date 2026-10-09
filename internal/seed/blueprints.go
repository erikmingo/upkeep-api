// Package seed holds the seedling blueprints shared by tests and cmd/seed.
// Scaffolded once with `go tool seedling-gen sqlc --config sqlc.yaml -pkg seed`;
// when a table is added, regenerate to a scratch path and merge by hand.
package seed

import (
	"context"
	"fmt"
	"math/rand/v2"
	"strings"

	"github.com/erikmingo/upkeep-api/internal/db"
	"github.com/mhiro2/seedling"
	"github.com/mhiro2/seedling/faker"
)

func NewRegistry() *seedling.Registry {
	reg := seedling.NewRegistry()
	seedling.MustRegisterTo(reg, seedling.Blueprint[db.User]{
		Name:    "user",
		Table:   "users",
		PKField: "ID",
		Defaults: func() db.User {
			return db.User{Email: "user@example.com", DisplayName: "User"}
		},
		Insert: func(ctx context.Context, dbtx seedling.DBTX, v db.User) (db.User, error) {
			return db.New(dbtx.(db.DBTX)).CreateUser(ctx, db.CreateUserParams{
				Email:       v.Email,
				DisplayName: v.DisplayName,
			})
		},
	})
	return reg
}

// Users returns a session for inserting users: seed.Users().InsertOne(t, tx, seed.Fake()).
func Users() seedling.Session[db.User] { return seedling.NewSession[db.User](NewRegistry()) }

// Fake fills a user from the plan's RNG; pair with seedling.WithSeed for a reproducible set.
func Fake() seedling.Option {
	return seedling.Generate(func(r *rand.Rand, u *db.User) {
		f := faker.New(r)
		first, last := f.FirstName(), f.LastName()
		u.DisplayName = first + " " + last
		// ponytail: random suffix instead of a uniqueness registry; fine below ~10k rows
		u.Email = fmt.Sprintf("%s.%s%d@example.com", strings.ToLower(first), strings.ToLower(last), r.IntN(10000))
	})
}
