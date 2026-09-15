package gateway

import (
	"testing"

	"be-modami-no-service/pkg/centrifugo"
)

const (
	secret = "test-secret"
	alice  = "user-alice"
	bob    = "user-bob"
)

func subToken(t *testing.T, user, channel string, ttl int) string {
	t.Helper()
	tok, err := centrifugo.GenerateSubscriptionToken(secret, user, channel, ttl)
	if err != nil {
		t.Fatalf("mint subscription token: %v", err)
	}
	return tok
}

func TestAllow(t *testing.T) {
	p := NewChannelPolicy(secret)
	chl := centrifugo.ChallengeChannel("chl-1")

	cases := []struct {
		name    string
		user    string
		channel string
		token   string
		want    bool
	}{
		{"own personal channel", alice, centrifugo.NotiChannel(alice), "", true},
		{"someone else's personal channel", alice, centrifugo.NotiChannel(bob), "", false},
		{"public topic needs no token", alice, "noti:topic:release-notes", "", true},
		{"no user id", "", centrifugo.NotiChannel(alice), "", false},

		{"challenge without token", alice, chl, "", false},
		{"challenge with valid token", alice, chl, subToken(t, alice, chl, 60), true},
		{"challenge token minted for another user", alice, chl, subToken(t, bob, chl, 60), false},
		{"challenge token minted for another channel", alice, chl,
			subToken(t, alice, centrifugo.ChallengeChannel("chl-2"), 60), false},
		{"challenge token already expired", alice, chl, subToken(t, alice, chl, -60), false},
		{"challenge token signed with a different secret", alice, chl,
			mustToken(t, "other-secret", alice, chl), false},
		{"garbage token", alice, chl, "not-a-jwt", false},

		{"channel outside the noti namespace", alice, "chat:room:1", "", false},
		{"unknown channel kind", alice, "noti:secret:1", "", false},
		{"malformed channel", alice, "noti:user", "", false},
		{"empty channel id", alice, "noti:user:", "", false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := p.Allow(c.user, c.channel, c.token); got != c.want {
				t.Errorf("Allow(%q, %q) = %v, want %v", c.user, c.channel, got, c.want)
			}
		})
	}
}

func mustToken(t *testing.T, sec, user, channel string) string {
	t.Helper()
	tok, err := centrifugo.GenerateSubscriptionToken(sec, user, channel, 60)
	if err != nil {
		t.Fatalf("mint token: %v", err)
	}
	return tok
}
