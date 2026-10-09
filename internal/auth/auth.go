// Package auth is passwordless sign-in: a one-time emailed token becomes a long-lived session.
package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/erikmingo/upkeep-api/internal/db"
	"github.com/erikmingo/upkeep-api/internal/mail"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

const (
	LoginTokenTTL = 15 * time.Minute
	SessionTTL    = 30 * 24 * time.Hour
)

var ErrInvalidToken = errors.New("invalid or expired token")

// Hash is what gets stored: a leaked table row cannot be replayed.
func Hash(token string) []byte {
	h := sha256.Sum256([]byte(token))
	return h[:]
}

func newToken() string {
	b := make([]byte, 24)
	rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

// RequestLink creates a login token for the email and mails it. The same response is produced
// whether or not the address is known, so the endpoint never reveals who has an account.
func RequestLink(ctx context.Context, q *db.Queries, m mail.Mailer, email string, now time.Time) error {
	email = strings.ToLower(strings.TrimSpace(email))
	if !strings.Contains(email, "@") {
		return errors.New("email required")
	}
	token := newToken()
	if _, err := q.CreateLoginToken(ctx, db.CreateLoginTokenParams{Email: email, TokenHash: Hash(token), ExpiresAt: pgtype.Timestamptz{Time: now.Add(LoginTokenTTL), Valid: true}}); err != nil {
		return err
	}
	text := fmt.Sprintf("Your Upkeep sign-in code:\n\n%s\n\nIt works once and expires in 15 minutes. Paste it into the app.", token)
	return m.Send(ctx, email, "Sign in to Upkeep", text)
}

type Signed struct {
	SessionToken string
	User         db.User
	Home         db.Home
	MemberID     int64
}

// Verify consumes a login token, creates the user and a first home when they are new, and opens a session.
func Verify(ctx context.Context, q *db.Queries, token string, now time.Time) (Signed, error) {
	lt, err := q.ConsumeLoginToken(ctx, db.ConsumeLoginTokenParams{TokenHash: Hash(token), ExpiresAt: pgtype.Timestamptz{Time: now, Valid: true}})
	if errors.Is(err, pgx.ErrNoRows) {
		return Signed{}, ErrInvalidToken
	}
	if err != nil {
		return Signed{}, err
	}
	user, err := q.GetUserByEmail(ctx, lt.Email)
	if errors.Is(err, pgx.ErrNoRows) {
		name, _, _ := strings.Cut(lt.Email, "@")
		user, err = q.CreateUser(ctx, db.CreateUserParams{Email: lt.Email, DisplayName: name})
	}
	if err != nil {
		return Signed{}, err
	}
	hm, err := q.FirstHomeForUser(ctx, user.ID)
	var home db.Home
	var memberID int64
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		home, err = q.CreateHome(ctx, "My home")
		if err != nil {
			return Signed{}, err
		}
		m, err := q.CreateMember(ctx, db.CreateMemberParams{HomeID: home.ID, UserID: user.ID})
		if err != nil {
			return Signed{}, err
		}
		memberID = m.ID
	case err != nil:
		return Signed{}, err
	default:
		home = db.Home{ID: hm.ID, Name: hm.Name, CreatedAt: hm.CreatedAt}
		memberID = hm.MemberID
	}
	session := newToken()
	if _, err := q.CreateSession(ctx, db.CreateSessionParams{UserID: user.ID, TokenHash: Hash(session), ExpiresAt: pgtype.Timestamptz{Time: now.Add(SessionTTL), Valid: true}}); err != nil {
		return Signed{}, err
	}
	return Signed{SessionToken: session, User: user, Home: home, MemberID: memberID}, nil
}

// Identity is who is calling: resolved from a session token by the API middleware.
type Identity struct {
	User     db.User
	Home     db.Home
	MemberID int64
}

func Resolve(ctx context.Context, q *db.Queries, sessionToken string, now time.Time) (Identity, error) {
	user, err := q.GetSessionUser(ctx, db.GetSessionUserParams{TokenHash: Hash(sessionToken), ExpiresAt: pgtype.Timestamptz{Time: now, Valid: true}})
	if errors.Is(err, pgx.ErrNoRows) {
		return Identity{}, ErrInvalidToken
	}
	if err != nil {
		return Identity{}, err
	}
	hm, err := q.FirstHomeForUser(ctx, user.ID)
	if err != nil {
		return Identity{}, err
	}
	return Identity{User: user, Home: db.Home{ID: hm.ID, Name: hm.Name, CreatedAt: hm.CreatedAt}, MemberID: hm.MemberID}, nil
}

const InviteTTL = 7 * 24 * time.Hour

// CreateInvite returns a one-time code that lets another signed-in user join the home.
func CreateInvite(ctx context.Context, q *db.Queries, homeID, memberID int64, now time.Time) (string, error) {
	code := newToken()
	_, err := q.CreateInvite(ctx, db.CreateInviteParams{HomeID: homeID, CreatedBy: memberID, CodeHash: Hash(code), ExpiresAt: pgtype.Timestamptz{Time: now.Add(InviteTTL), Valid: true}})
	return code, err
}

// AcceptInvite consumes the code and makes the user a member of its home. Already a member → no-op, still consumes.
func AcceptInvite(ctx context.Context, q *db.Queries, code string, userID int64, now time.Time) (db.Home, error) {
	inv, err := q.ConsumeInvite(ctx, db.ConsumeInviteParams{CodeHash: Hash(code), ExpiresAt: pgtype.Timestamptz{Time: now, Valid: true}})
	if errors.Is(err, pgx.ErrNoRows) {
		return db.Home{}, ErrInvalidToken
	}
	if err != nil {
		return db.Home{}, err
	}
	if _, err := q.CreateMember(ctx, db.CreateMemberParams{HomeID: inv.HomeID, UserID: userID}); err != nil {
		return db.Home{}, err
	}
	return q.GetHome(ctx, inv.HomeID)
}
