package db

import (
	"context"
	"os"
	"testing"
)

func TestMigrateAndUsers(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	pool, err := Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err := Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	q := New(tx)

	u, err := q.CreateUser(ctx, CreateUserParams{Email: "t@example.com", DisplayName: "T"})
	if err != nil {
		t.Fatal(err)
	}
	got, err := q.GetUser(ctx, u.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Email != "t@example.com" || !got.CreatedAt.Valid {
		t.Fatalf("got %+v", got)
	}
	if _, err := q.CreateUser(ctx, CreateUserParams{Email: "t@example.com", DisplayName: "dup"}); err == nil {
		t.Fatal("expected unique violation on duplicate email")
	}
}
