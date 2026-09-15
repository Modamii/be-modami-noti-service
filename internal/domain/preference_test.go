package domain

import (
	"testing"
	"time"
)

const tz = "Asia/Ho_Chi_Minh" // UTC+7, no DST

func at(t *testing.T, clock string) time.Time {
	t.Helper()
	loc, err := time.LoadLocation(tz)
	if err != nil {
		t.Skipf("tzdata unavailable: %v", err)
	}
	parsed, err := time.ParseInLocation("2006-01-02 15:04", "2026-09-15 "+clock, loc)
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}

func TestInQuietHoursCrossingMidnight(t *testing.T) {
	p := &Preference{QuietHours: &QuietHours{Start: "22:00", End: "07:00", Timezone: tz}}

	cases := map[string]bool{
		"21:59": false, // just before it starts
		"22:00": true,  // inclusive start
		"23:30": true,
		"03:00": true, // after midnight, still inside
		"06:59": true,
		"07:00": false, // exclusive end
		"12:00": false,
	}
	for clock, want := range cases {
		if got := p.InQuietHours(at(t, clock)); got != want {
			t.Errorf("at %s: InQuietHours = %v, want %v", clock, got, want)
		}
	}
}

func TestInQuietHoursSameDayWindow(t *testing.T) {
	p := &Preference{QuietHours: &QuietHours{Start: "13:00", End: "14:00", Timezone: tz}}

	for clock, want := range map[string]bool{
		"12:59": false,
		"13:00": true,
		"13:59": true,
		"14:00": false,
		"23:00": false,
	} {
		if got := p.InQuietHours(at(t, clock)); got != want {
			t.Errorf("at %s: InQuietHours = %v, want %v", clock, got, want)
		}
	}
}

func TestInQuietHoursUsesTheUsersTimezone(t *testing.T) {
	// 23:00 in Ho Chi Minh is 16:00 UTC. Evaluating the window in UTC would say
	// "not quiet"; evaluating in the user's zone says "quiet".
	p := &Preference{QuietHours: &QuietHours{Start: "22:00", End: "07:00", Timezone: tz}}
	utcInstant := at(t, "23:00").UTC()
	if !p.InQuietHours(utcInstant) {
		t.Error("a UTC timestamp must still be judged against the user's local clock")
	}
}

func TestInQuietHoursDisabledCases(t *testing.T) {
	now := at(t, "23:00")
	cases := map[string]*Preference{
		"no window":         {},
		"nil quiet hours":   {QuietHours: nil},
		"empty timezone":    {QuietHours: &QuietHours{Start: "22:00", End: "07:00"}},
		"unknown timezone":  {QuietHours: &QuietHours{Start: "22:00", End: "07:00", Timezone: "Mars/Olympus"}},
		"malformed start":   {QuietHours: &QuietHours{Start: "25:99", End: "07:00", Timezone: tz}},
		"malformed end":     {QuietHours: &QuietHours{Start: "22:00", End: "nope", Timezone: tz}},
		"zero-length range": {QuietHours: &QuietHours{Start: "22:00", End: "22:00", Timezone: tz}},
	}
	for name, p := range cases {
		if p.InQuietHours(now) {
			t.Errorf("%s: should not mute, but reported quiet hours", name)
		}
	}
}

func TestIsMuted(t *testing.T) {
	p := &Preference{MutedIdentities: []string{"streak_at_risk", "flashcard_due"}}
	if !p.IsMuted("streak_at_risk") {
		t.Error("muted identity reported as allowed")
	}
	if p.IsMuted("video_ready") {
		t.Error("unmuted identity reported as muted")
	}
	// Opt-out model: nothing configured means nothing muted.
	if (&Preference{}).IsMuted("streak_at_risk") {
		t.Error("a user with no preferences must still receive notifications")
	}
}
