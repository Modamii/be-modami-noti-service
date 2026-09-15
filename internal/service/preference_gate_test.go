package service

import (
	"testing"
	"time"

	"be-modami-no-service/internal/domain"
	"be-modami-no-service/pkg/contract"
)

func TestAllowed(t *testing.T) {
	const tz = "Asia/Ho_Chi_Minh"
	loc, err := time.LoadLocation(tz)
	if err != nil {
		t.Skipf("tzdata unavailable: %v", err)
	}
	quietNow := time.Date(2026, 9, 15, 23, 0, 0, 0, loc)
	awakeNow := time.Date(2026, 9, 15, 12, 0, 0, 0, loc)

	on := func() *domain.Preference {
		return &domain.Preference{InAppEnabled: true, PushEnabled: true}
	}
	quiet := &domain.QuietHours{Start: "22:00", End: "07:00", Timezone: tz}

	cases := []struct {
		name     string
		pref     *domain.Preference
		channel  string
		now      time.Time
		identity string
		want     bool
	}{
		{"everything on", on(), contract.ChannelPush, awakeNow, contract.VideoReady, true},
		{"push off", &domain.Preference{InAppEnabled: true}, contract.ChannelPush, awakeNow, contract.VideoReady, false},
		{"in-app off", &domain.Preference{PushEnabled: true}, contract.ChannelInApp, awakeNow, contract.VideoReady, false},
		{"channels are independent", &domain.Preference{InAppEnabled: true}, contract.ChannelInApp, awakeNow, contract.VideoReady, true},

		{
			"muted type blocks every channel",
			&domain.Preference{InAppEnabled: true, PushEnabled: true, MutedIdentities: []string{contract.StreakAtRisk}},
			contract.ChannelPush, awakeNow, contract.StreakAtRisk, false,
		},
		{
			"muting one type leaves the others alone",
			&domain.Preference{InAppEnabled: true, PushEnabled: true, MutedIdentities: []string{contract.StreakAtRisk}},
			contract.ChannelPush, awakeNow, contract.VideoReady, true,
		},

		{
			"quiet hours hold push back",
			&domain.Preference{InAppEnabled: true, PushEnabled: true, QuietHours: quiet},
			contract.ChannelPush, quietNow, contract.StreakAtRisk, false,
		},
		{
			"quiet hours do not touch in-app",
			&domain.Preference{InAppEnabled: true, PushEnabled: true, QuietHours: quiet},
			contract.ChannelInApp, quietNow, contract.VideoReady, true,
		},
		{
			"outside quiet hours push flows again",
			&domain.Preference{InAppEnabled: true, PushEnabled: true, QuietHours: quiet},
			contract.ChannelPush, awakeNow, contract.StreakAtRisk, true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := allowed(c.pref, c.identity, c.channel, c.now); got != c.want {
				t.Errorf("allowed = %v, want %v", got, c.want)
			}
		})
	}
}
