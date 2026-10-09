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
