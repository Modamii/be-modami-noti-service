// Package push delivers notifications to devices through Firebase Cloud Messaging.
//
// iOS goes through FCM too rather than a direct APNs connection: the APNs auth
// key is uploaded to the Firebase console, Firebase forwards to Apple, and the
// server keeps one SDK and one code path for both platforms.
package push

import (
	"context"
	"errors"
	"fmt"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/messaging"
	"google.golang.org/api/option"
)

// ErrNotConfigured is returned by NewFCMSender when no credentials are supplied.
var ErrNotConfigured = errors.New("push: FCM credentials not configured")

// Result reports the outcome of one multi-device send.
type Result struct {
	Sent int
	// Unregistered lists tokens the device no longer accepts — the app was
	// uninstalled or the token was rotated. Callers must delete these; keeping
	// them means every future send wastes a request.
	Unregistered []string
	// Failed lists tokens that failed for some other reason, paired with why.
	// These are transient or configuration problems, not reasons to delete.
	Failed map[string]string
}

// Sender delivers push notifications.
type Sender interface {
	Send(ctx context.Context, tokens []string, n Notification) (*Result, error)
}

// Notification is one alert plus the data payload the app receives with it.
type Notification struct {
	Title string
	Body  string
	Data  map[string]string
}

type fcmSender struct {
	client *messaging.Client
}

// NewFCMSender builds a sender from a service-account JSON file.
// Returns ErrNotConfigured when credentialsPath is empty, so the caller can run
// without push rather than failing to start.
func NewFCMSender(ctx context.Context, credentialsPath string) (Sender, error) {
	if credentialsPath == "" {
		return nil, ErrNotConfigured
	}
	app, err := firebase.NewApp(ctx, nil, option.WithCredentialsFile(credentialsPath))
	if err != nil {
		return nil, fmt.Errorf("push: init firebase app: %w", err)
	}
	client, err := app.Messaging(ctx)
	if err != nil {
		return nil, fmt.Errorf("push: init messaging client: %w", err)
	}
	return &fcmSender{client: client}, nil
}

// Send delivers to every token and reports per-token outcomes. A partial failure
// is not an error: some devices succeeding is the normal case.
func (s *fcmSender) Send(ctx context.Context, tokens []string, n Notification) (*Result, error) {
	if len(tokens) == 0 {
		return &Result{}, nil
	}

	resp, err := s.client.SendEachForMulticast(ctx, &messaging.MulticastMessage{
		Tokens: tokens,
		Notification: &messaging.Notification{
			Title: n.Title,
			Body:  n.Body,
		},
		Data: n.Data,
		Android: &messaging.AndroidConfig{
			Priority: "high",
		},
		APNS: &messaging.APNSConfig{
			Payload: &messaging.APNSPayload{
				Aps: &messaging.Aps{Sound: "default"},
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("push: send multicast: %w", err)
	}

	result := &Result{Sent: resp.SuccessCount, Failed: map[string]string{}}
	for i, r := range resp.Responses {
		if r.Success || i >= len(tokens) {
			continue
		}
		token := tokens[i]
		if messaging.IsUnregistered(r.Error) || messaging.IsInvalidArgument(r.Error) {
			// The device is gone or the token is malformed — either way it will
			// never succeed again.
			result.Unregistered = append(result.Unregistered, token)
			continue
		}
		result.Failed[token] = r.Error.Error()
	}
	return result, nil
}
