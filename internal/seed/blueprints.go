// Package seed holds the seedling blueprints shared by tests and cmd/seed.
// Scaffolded once with `go tool seedling-gen sqlc --config sqlc.yaml -pkg seed`;
// when a table is added, regenerate to a scratch path and merge by hand.
package seed

import (
	"context"
	"fmt"
	"math/rand/v2"
	"strings"
	"time"

	"github.com/erikmingo/upkeep-api/internal/db"
	"github.com/jackc/pgx/v5/pgtype"
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
	seedling.MustRegisterTo(reg, seedling.Blueprint[db.Home]{
		Name:     "home",
		Table:    "homes",
		PKField:  "ID",
		Defaults: func() db.Home { return db.Home{Name: "Test Home"} },
		Insert: func(ctx context.Context, dbtx seedling.DBTX, v db.Home) (db.Home, error) {
			return db.New(dbtx.(db.DBTX)).CreateHome(ctx, v.Name)
		},
	})
	seedling.MustRegisterTo(reg, seedling.Blueprint[db.Member]{
		Name:     "member",
		Table:    "members",
		PKField:  "ID",
		Defaults: func() db.Member { return db.Member{} },
		Relations: []seedling.Relation{
			{Name: "home", Kind: seedling.BelongsTo, LocalField: "HomeID", RefBlueprint: "home"},
			{Name: "user", Kind: seedling.BelongsTo, LocalField: "UserID", RefBlueprint: "user"},
		},
		Insert: func(ctx context.Context, dbtx seedling.DBTX, v db.Member) (db.Member, error) {
			return db.New(dbtx.(db.DBTX)).CreateMember(ctx, db.CreateMemberParams{HomeID: v.HomeID, UserID: v.UserID})
		},
	})
	seedling.MustRegisterTo(reg, seedling.Blueprint[db.Fact]{
		Name:     "fact",
		Table:    "facts",
		PKField:  "ID",
		Defaults: func() db.Fact { return db.Fact{Key: "home.type", Value: []byte(`"house"`), Source: "wizard"} },
		Relations: []seedling.Relation{
			{Name: "home", Kind: seedling.BelongsTo, LocalField: "HomeID", RefBlueprint: "home"},
		},
		Insert: func(ctx context.Context, dbtx seedling.DBTX, v db.Fact) (db.Fact, error) {
			return db.New(dbtx.(db.DBTX)).UpsertFact(ctx, db.UpsertFactParams{HomeID: v.HomeID, Key: v.Key, Value: v.Value, Source: v.Source})
		},
	})
	seedling.MustRegisterTo(reg, seedling.Blueprint[db.Task]{
		Name:     "task",
		Table:    "tasks",
		PKField:  "ID",
		Defaults: func() db.Task { return db.Task{Title: "Change furnace filter", IntervalDays: 90} },
		Relations: []seedling.Relation{
			{Name: "home", Kind: seedling.BelongsTo, LocalField: "HomeID", RefBlueprint: "home"},
		},
		Insert: func(ctx context.Context, dbtx seedling.DBTX, v db.Task) (db.Task, error) {
			start := v.StartDate
			if !start.Valid {
				start = pgtype.Date{Time: time.Now().UTC().Truncate(24 * time.Hour), Valid: true}
			}
			return db.New(dbtx.(db.DBTX)).CreateTask(ctx, db.CreateTaskParams{
				HomeID: v.HomeID, RuleID: v.RuleID, Title: v.Title, Detail: v.Detail, IntervalDays: v.IntervalDays,
				SeasonStart: v.SeasonStart, SeasonEnd: v.SeasonEnd, StartDate: start,
			})
		},
	})
	seedling.MustRegisterTo(reg, seedling.Blueprint[db.Completion]{
		Name:     "completion",
		Table:    "completions",
		PKField:  "ID",
		Defaults: func() db.Completion { return db.Completion{} },
		Relations: []seedling.Relation{
			{Name: "task", Kind: seedling.BelongsTo, LocalField: "TaskID", RefBlueprint: "task"},
			{Name: "member", Kind: seedling.BelongsTo, LocalField: "MemberID", RefBlueprint: "member"},
		},
		Insert: func(ctx context.Context, dbtx seedling.DBTX, v db.Completion) (db.Completion, error) {
			at := v.CompletedAt
			if !at.Valid {
				at = pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true}
			}
			return db.New(dbtx.(db.DBTX)).CreateCompletion(ctx, db.CreateCompletionParams{TaskID: v.TaskID, MemberID: v.MemberID, CompletedAt: at})
		},
	})
	return reg
}

// Users returns a session for inserting users: seed.Users().InsertOne(t, tx, seed.Fake()).
func Users() seedling.Session[db.User] { return seedling.NewSession[db.User](NewRegistry()) }

// Session returns a session for any registered model; parents (home, user, task) are inserted on demand.
func Session[T any]() seedling.Session[T] { return seedling.NewSession[T](NewRegistry()) }

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
