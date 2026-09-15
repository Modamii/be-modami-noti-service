package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"be-modami-no-service/config"
	"be-modami-no-service/internal/api/middleware"
	"be-modami-no-service/internal/store/memory"

	"github.com/golang-jwt/jwt/v5"
)

// userToken mints an unsigned-verification token; the api runs with jwks_url
// empty in this test, matching the gateway-terminated deployment mode.
func userToken(t *testing.T, sub string) string {
	t.Helper()
	tok, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"sub": sub}).SignedString([]byte("x"))
	if err != nil {
		t.Fatal(err)
	}
	return tok
}

func newServer(t *testing.T) http.Handler {
	t.Helper()
	cfg := &configs.Config{}
	cfg.Centrifugo.HMACSecret = "secret"
	cfg.Centrifugo.TokenTTL = 3600

	apiMux := http.NewServeMux()
	RegisterAll(apiMux,
		NewAuthHandler(cfg),
		NewNotificationHandler(memory.NewNotificationStore()),
	)
	root := http.NewServeMux()
	root.Handle("/v1/noti-services/", middleware.NewAuth("").Required(apiMux))
	return root
}

func do(t *testing.T, h http.Handler, method, path, bearer string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(""))
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestCentrifugoTokenRequiresAuth(t *testing.T) {
	h := newServer(t)

	if got := do(t, h, "POST", "/v1/noti-services/auth/centrifugo-token", "").Code; got != http.StatusUnauthorized {
		t.Errorf("no bearer → %d, want 401", got)
	}

	rec := do(t, h, "POST", "/v1/noti-services/auth/centrifugo-token", userToken(t, "alice"))
	if rec.Code != http.StatusOK {
		t.Fatalf("with bearer → %d, want 200 (body %s)", rec.Code, rec.Body)
	}

	// The minted token must name the caller, no matter what the caller asked for.
	var body struct {
		Data struct{ Token string } `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode %s: %v", rec.Body, err)
	}
	parsed, _, err := jwt.NewParser().ParseUnverified(body.Data.Token, jwt.MapClaims{})
	if err != nil {
		t.Fatal(err)
	}
	if sub := parsed.Claims.(jwt.MapClaims)["sub"]; sub != "alice" {
		t.Errorf("minted token sub = %v, want alice", sub)
	}
}

func TestCannotReadAnotherUsersNotifications(t *testing.T) {
	h := newServer(t)
	alice := userToken(t, "alice")

	if got := do(t, h, "GET", "/v1/noti-services/users/alice/notifications", alice).Code; got != http.StatusOK {
		t.Errorf("own notifications → %d, want 200", got)
	}
	if got := do(t, h, "GET", "/v1/noti-services/users/bob/notifications", alice).Code; got != http.StatusForbidden {
		t.Errorf("bob's notifications with alice's token → %d, want 403", got)
	}
	if got := do(t, h, "GET", "/v1/noti-services/users/bob/notifications", "").Code; got != http.StatusUnauthorized {
		t.Errorf("no bearer → %d, want 401", got)
	}
	// An id-addressed notification the caller does not own reads as missing.
	if got := do(t, h, "PATCH", "/v1/noti-services/notifications/n-999/read", alice).Code; got != http.StatusNotFound {
		t.Errorf("foreign notification id → %d, want 404", got)
	}
}
