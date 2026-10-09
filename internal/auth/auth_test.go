package auth

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/erikmingo/upkeep-api/internal/db"
	"github.com/mhiro2/seedling/seedlingpgx"
)

type captureMailer struct{ to, text string }

func (c *captureMailer) Send(_ context.Context, to, _, text string) error {
	c.to, c.text = to, text
	return nil
}

func tokenFrom(text string) string {
	lines := strings.Split(text, "\n")
	return lines[2]
}

func testQ(t *testing.T) *db.Queries {
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
	return db.New(seedlingpgx.WithTx(t, pool))
}

func TestMagicLinkFlow(t *testing.T) {
	q := testQ(t)
	ctx := context.Background()
	now := time.Now()
	m := &captureMailer{}

	if err := RequestLink(ctx, q, m, "  Ada@Example.com ", now); err != nil {
		t.Fatal(err)
	}
	if m.to != "ada@example.com" || !strings.Contains(m.text, "sign-in code") {
		t.Fatalf("mail: to=%q text=%q", m.to, m.text)
	}
	token := tokenFrom(m.text)

	s, err := Verify(ctx, q, token, now)
	if err != nil {
		t.Fatal(err)
	}
	if s.User.Email != "ada@example.com" || s.User.DisplayName != "ada" || s.Home.Name != "My home" || s.MemberID == 0 || s.SessionToken == "" {
		t.Fatalf("signed=%+v", s)
	}

	// single use
	if _, err := Verify(ctx, q, token, now); err != ErrInvalidToken {
		t.Fatalf("reuse: err=%v", err)
	}
	// garbage
	if _, err := Verify(ctx, q, "nope", now); err != ErrInvalidToken {
		t.Fatalf("garbage: err=%v", err)
	}

	// the session resolves to the same user and home
	id, err := Resolve(ctx, q, s.SessionToken, now)
	if err != nil || id.User.ID != s.User.ID || id.Home.ID != s.Home.ID || id.MemberID != s.MemberID {
		t.Fatalf("resolve: %+v err=%v", id, err)
	}
	if _, err := Resolve(ctx, q, "nope", now); err != ErrInvalidToken {
		t.Fatalf("bad session: err=%v", err)
	}

	// second sign-in: same user, same home, no duplicates
	if err := RequestLink(ctx, q, m, "ada@example.com", now); err != nil {
		t.Fatal(err)
	}
	s2, err := Verify(ctx, q, tokenFrom(m.text), now)
	if err != nil || s2.User.ID != s.User.ID || s2.Home.ID != s.Home.ID {
		t.Fatalf("second: %+v err=%v", s2, err)
	}
	if err := q.DeleteSession(ctx, Hash(s2.SessionToken)); err != nil {
		t.Fatal(err)
	}
	if _, err := Resolve(ctx, q, s2.SessionToken, now); err != ErrInvalidToken {
		t.Fatal("deleted session still resolves")
	}
}

func TestExpiredTokenIsRejected(t *testing.T) {
	q := testQ(t)
	ctx := context.Background()
	m := &captureMailer{}
	// issued 16 minutes ago → expired now
	if err := RequestLink(ctx, q, m, "old@example.com", time.Now().Add(-16*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if _, err := Verify(ctx, q, tokenFrom(m.text), time.Now()); err != ErrInvalidToken {
		t.Fatalf("expired: err=%v", err)
	}
	// a session also expires on the injected clock
	if err := RequestLink(ctx, q, m, "s@example.com", time.Now()); err != nil {
		t.Fatal(err)
	}
	s, err := Verify(ctx, q, tokenFrom(m.text), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Resolve(ctx, q, s.SessionToken, time.Now().Add(SessionTTL+time.Minute)); err != ErrInvalidToken {
		t.Fatalf("expired session: err=%v", err)
	}
	if err := RequestLink(ctx, q, m, "no-at-sign", time.Now()); err == nil {
		t.Fatal("bad email accepted")
	}
}

func TestInvites(t *testing.T) {
	q := testQ(t)
	ctx := context.Background()
	now := time.Now()
	m := &captureMailer{}
	signUp := func(email string) Signed {
		if err := RequestLink(ctx, q, m, email, now); err != nil {
			t.Fatal(err)
		}
		s, err := Verify(ctx, q, tokenFrom(m.text), now)
		if err != nil {
			t.Fatal(err)
		}
		return s
	}
	owner := signUp("owner@example.com")
	guest := signUp("guest@example.com")

	code, err := CreateInvite(ctx, q, owner.Home.ID, owner.MemberID, now)
	if err != nil {
		t.Fatal(err)
	}
	home, err := AcceptInvite(ctx, q, code, guest.User.ID, now)
	if err != nil || home.ID != owner.Home.ID {
		t.Fatalf("accept: home=%+v err=%v", home, err)
	}
	members, _ := q.ListMembers(ctx, owner.Home.ID)
	if len(members) != 2 {
		t.Fatalf("members=%d", len(members))
	}
	// single use, bad code, expired
	if _, err := AcceptInvite(ctx, q, code, guest.User.ID, now); err != ErrInvalidToken {
		t.Fatalf("reuse: %v", err)
	}
	if _, err := AcceptInvite(ctx, q, "nope", guest.User.ID, now); err != ErrInvalidToken {
		t.Fatalf("garbage: %v", err)
	}
	old, _ := CreateInvite(ctx, q, owner.Home.ID, owner.MemberID, now.Add(-8*24*time.Hour))
	if _, err := AcceptInvite(ctx, q, old, guest.User.ID, now); err != ErrInvalidToken {
		t.Fatalf("expired: %v", err)
	}
	// accepting as an existing member is a no-op
	again, _ := CreateInvite(ctx, q, owner.Home.ID, owner.MemberID, now)
	if _, err := AcceptInvite(ctx, q, again, owner.User.ID, now); err != nil {
		t.Fatal(err)
	}
	members, _ = q.ListMembers(ctx, owner.Home.ID)
	if len(members) != 2 {
		t.Fatalf("members after self-accept=%d", len(members))
	}
}
