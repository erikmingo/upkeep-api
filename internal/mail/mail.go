// Package mail sends transactional email behind one interface: log in dev, Resend in production.
package mail

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
)

type Mailer interface {
	Send(ctx context.Context, to, subject, text string) error
}

// Log prints the message instead of sending it. Dev only.
type Log struct{}

func (Log) Send(_ context.Context, to, subject, text string) error {
	log.Printf("mail to %s: %s\n%s", to, subject, text)
	return nil
}

// Resend posts to https://resend.com/docs/api-reference/emails/send-email.
type Resend struct {
	APIKey string
	From   string
	Client *http.Client
}

func (r Resend) Send(ctx context.Context, to, subject, text string) error {
	body, _ := json.Marshal(map[string]any{"from": r.From, "to": []string{to}, "subject": subject, "text": text})
	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.resend.com/emails", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+r.APIKey)
	req.Header.Set("Content-Type", "application/json")
	c := r.Client
	if c == nil {
		c = http.DefaultClient
	}
	resp, err := c.Do(req)
	if err != nil {
		return fmt.Errorf("resend: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("resend: %s: %s", resp.Status, msg)
	}
	return nil
}
