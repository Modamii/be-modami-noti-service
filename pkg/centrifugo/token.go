package centrifugo

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// GenerateConnectionToken creates a JWT for Centrifugo client connection.
// The token contains the user ID (sub) and expiration time.
func GenerateConnectionToken(secret string, userID string, ttlSeconds int) (string, error) {
	now := time.Now()
	claims := jwt.MapClaims{
		"sub": userID,
		"iat": now.Unix(),
		"exp": now.Add(time.Duration(ttlSeconds) * time.Second).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		return "", fmt.Errorf("centrifugo: sign connection token: %w", err)
	}
	return signed, nil
}

// GenerateSubscriptionToken creates a JWT for subscribing to a specific Centrifugo channel.
func GenerateSubscriptionToken(secret string, userID string, channel string, ttlSeconds int) (string, error) {
	now := time.Now()
	claims := jwt.MapClaims{
		"sub":     userID,
		"channel": channel,
		"iat":     now.Unix(),
		"exp":     now.Add(time.Duration(ttlSeconds) * time.Second).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		return "", fmt.Errorf("centrifugo: sign subscription token: %w", err)
	}
	return signed, nil
}

// ParseSubscriptionToken verifies a channel subscription token and returns the
// subject and channel it was minted for. Callers must check both against the
// request — a valid token for a different channel or user must not grant access.
func ParseSubscriptionToken(secret, tokenStr string) (sub, channel string, err error) {
	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return []byte(secret), nil
	})
	if err != nil {
		return "", "", err
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return "", "", jwt.ErrSignatureInvalid
	}
	sub, _ = claims["sub"].(string)
	channel, _ = claims["channel"].(string)
	if sub == "" || channel == "" {
		return "", "", fmt.Errorf("centrifugo: subscription token missing sub or channel")
	}
	return sub, channel, nil
}
