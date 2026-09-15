// Package middleware holds HTTP middleware for the notification API.
package middleware

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"gitlab.com/lifegoeson-libs/pkg-gokit/response"
)

type ctxKey int

const ctxKeyUserID ctxKey = iota

// Auth validates the caller's access token against the identity provider's JWKS.
// Mirrors be-modami-core-service so both services trust the same tokens.
type Auth struct {
	jwksURL string
	cache   *jwksCache
}

// NewAuth builds the middleware. An empty jwksURL disables signature verification
// and trusts the sub claim — only valid behind a gateway that already verified the
// token, which is why Required refuses to run that way unless allowUnverified is set.
func NewAuth(jwksURL string) *Auth {
	a := &Auth{jwksURL: jwksURL, cache: &jwksCache{keys: map[string]*rsa.PublicKey{}}}
	if jwksURL != "" {
		_ = a.cache.refresh(jwksURL) // best-effort warm-up; get() refreshes on miss
	}
	return a
}

// Required wraps a handler so it only runs for authenticated callers.
// The authenticated user ID is placed in the request context; read it with UserID.
func (a *Auth) Required(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw := r.Header.Get("Authorization")
		parts := strings.SplitN(raw, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
			response.Unauthorized(w, "missing bearer token")
			return
		}

		claims, err := a.parse(parts[1])
		if err != nil {
			response.Unauthorized(w, "invalid token")
			return
		}
		sub, _ := claims["sub"].(string)
		if sub == "" {
			response.Unauthorized(w, "token has no subject")
			return
		}

		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxKeyUserID, sub)))
	})
}

// UserID returns the authenticated user ID, or "" when the request did not pass
// through Required.
func UserID(r *http.Request) string {
	id, _ := r.Context().Value(ctxKeyUserID).(string)
	return id
}

func (a *Auth) parse(tokenStr string) (jwt.MapClaims, error) {
	if a.jwksURL == "" {
		token, _, err := jwt.NewParser().ParseUnverified(tokenStr, jwt.MapClaims{})
		if err != nil {
			return nil, err
		}
		return token.Claims.(jwt.MapClaims), nil
	}

	token, err := jwt.ParseWithClaims(tokenStr, jwt.MapClaims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		kid, _ := t.Header["kid"].(string)
		return a.cache.get(kid, a.jwksURL)
	})
	if err != nil {
		return nil, err
	}
	if !token.Valid {
		return nil, fmt.Errorf("token is not valid")
	}
	return token.Claims.(jwt.MapClaims), nil
}

// jwksCache caches the IdP's public keys by key id, refreshing on an unknown kid
// so key rotation does not require a restart.
type jwksCache struct {
	mu   sync.RWMutex
	keys map[string]*rsa.PublicKey
}

func (c *jwksCache) get(kid, url string) (*rsa.PublicKey, error) {
	c.mu.RLock()
	key, ok := c.keys[kid]
	c.mu.RUnlock()
	if ok {
		return key, nil
	}
	if err := c.refresh(url); err != nil {
		return nil, err
	}
	c.mu.RLock()
	key, ok = c.keys[kid]
	c.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("unknown key id: %s", kid)
	}
	return key, nil
}

func (c *jwksCache) refresh(url string) error {
	resp, err := (&http.Client{Timeout: 10 * time.Second}).Get(url)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	var jwks struct {
		Keys []struct {
			Kty string `json:"kty"`
			Kid string `json:"kid"`
			N   string `json:"n"`
			E   string `json:"e"`
		} `json:"keys"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&jwks); err != nil {
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	for _, k := range jwks.Keys {
		if k.Kty != "RSA" {
			continue
		}
		pub, err := parseRSAPublicKey(k.N, k.E)
		if err != nil {
			continue
		}
		c.keys[k.Kid] = pub
	}
	return nil
}

func parseRSAPublicKey(nB64, eB64 string) (*rsa.PublicKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(nB64)
	if err != nil {
		return nil, err
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(eB64)
	if err != nil {
		return nil, err
	}
	return &rsa.PublicKey{
		N: new(big.Int).SetBytes(nBytes),
		E: int(new(big.Int).SetBytes(eBytes).Int64()),
	}, nil
}
