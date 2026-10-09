package seed

import (
	"context"
	"os"
	"testing"

	"github.com/erikmingo/upkeep-api/internal/db"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mhiro2/seedling"
	"github.com/mhiro2/seedling/seedlingpgx"
)

func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	pool, err := db.Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := db.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	return pool
}

func TestInsertOneUser(t *testing.T) {
	pool := testPool(t)
	tx := seedlingpgx.WithTx(t, pool)

	u := Users().InsertOne(t, tx, Fake()).Root()
	if u.ID == 0 || u.Email == "" || u.DisplayName == "" {
		t.Fatalf("got %+v", u)
	}
	got, err := db.New(tx).GetUser(context.Background(), u.ID)
	if err != nil || got.Email != u.Email {
		t.Fatalf("row not in db: %+v %v", got, err)
	}
}

func TestFakeIsDeterministic(t *testing.T) {
	pool := testPool(t)
	// each batch in a subtest so its transaction rolls back before the next one inserts the same emails
	emails := func(name string) (out []string) {
		t.Run(name, func(t *testing.T) {
			tx := seedlingpgx.WithTx(t, pool)
			for _, u := range Users().InsertMany(t, tx, 3, seedling.WithSeed(7), Fake()) {
				out = append(out, u.Email)
			}
		})
		return out
	}
	a, b := emails("run1"), emails("run2")
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("run 1 %v != run 2 %v", a, b)
		}
	}
	if a[0] == a[1] {
		t.Fatalf("rows share an email: %v", a)
	}
}

func TestTaskResolvesItsHome(t *testing.T) {
	pool := testPool(t)
	tx := seedlingpgx.WithTx(t, pool)

	res := Session[db.Task]().InsertOne(t, tx)
	task := res.Root()
	if task.ID == 0 || task.HomeID == 0 {
		t.Fatalf("got %+v", task)
	}
	home, err := db.New(tx).GetHome(context.Background(), task.HomeID)
	if err != nil || home.Name != "Test Home" {
		t.Fatalf("home not inserted: %+v %v", home, err)
	}
	c := Session[db.Completion]().InsertOne(t, tx, seedling.Use("task", task)).Root()
	if c.TaskID != task.ID || c.MemberID == 0 {
		t.Fatalf("got %+v", c)
	}
}

func TestDenverSeed(t *testing.T) {
	pool := testPool(t)
	tx := seedlingpgx.WithTx(t, pool)
	q := db.New(tx)
	ctx := context.Background()

	u := Users().InsertOne(t, tx, Fake()).Root()
	home, err := Denver(ctx, q, u.ID)
	if err != nil {
		t.Fatal(err)
	}
	facts, err := q.ListFacts(ctx, home.ID)
	if err != nil || len(facts) != len(DenverFacts) {
		t.Fatalf("facts=%d want %d err=%v", len(facts), len(DenverFacts), err)
	}
	members, err := q.ListMembers(ctx, home.ID)
	if err != nil || len(members) != 1 || members[0].Email != u.Email {
		t.Fatalf("members=%+v err=%v", members, err)
	}
	// upsert: re-running a fact keeps the row count
	if _, err := q.UpsertFact(ctx, db.UpsertFactParams{HomeID: home.ID, Key: "home.beds", Value: []byte("3"), Source: "user"}); err != nil {
		t.Fatal(err)
	}
	facts, _ = q.ListFacts(ctx, home.ID)
	if len(facts) != len(DenverFacts) {
		t.Fatalf("upsert duplicated a fact: %d", len(facts))
	}
}
