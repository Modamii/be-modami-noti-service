package domain

import "time"

// Preference is a user's notification settings.
//
// The model is opt-out: when no document is stored the defaults allow everything,
// so a user who never opened the settings screen still gets notified.
type Preference struct {
	UserID       string `bson:"user_id" json:"user_id"`
	InAppEnabled bool   `bson:"in_app_enabled" json:"in_app_enabled"`
	PushEnabled  bool   `bson:"push_enabled" json:"push_enabled"`

	// MutedIdentities turns off individual notification types while leaving the
	// channel on — "keep push, but stop telling me about streaks".
	MutedIdentities []string `bson:"muted_identities,omitempty" json:"muted_identities,omitempty"`

	// QuietHours holds push back during a daily window. In-app is unaffected:
	// it makes no sound and the user is already looking at the screen.
	QuietHours *QuietHours `bson:"quiet_hours,omitempty" json:"quiet_hours,omitempty"`
}

// QuietHours is a daily window in the user's own timezone. It may cross midnight
// (22:00 → 07:00), which is the common case.
type QuietHours struct {
	// Start and End are "HH:MM" in 24-hour time.
	Start string `bson:"start" json:"start"`
	End   string `bson:"end" json:"end"`
	// Timezone is an IANA name, e.g. "Asia/Ho_Chi_Minh". An empty or unresolvable
	// zone disables the window rather than guessing — silently shifting someone's
	// quiet hours by several zones is worse than not having them.
	Timezone string `bson:"timezone" json:"timezone"`
}

// IsMuted reports whether the user turned off this notification type.
func (p *Preference) IsMuted(identity string) bool {
	for _, m := range p.MutedIdentities {
		if m == identity {
			return true
		}
	}
	return false
}

// InQuietHours reports whether now falls inside the user's quiet window.
// False when no window is set, the timezone cannot be resolved, or the times
// are malformed.
func (p *Preference) InQuietHours(now time.Time) bool {
	q := p.QuietHours
	if q == nil || q.Timezone == "" {
		return false
	}
	loc, err := time.LoadLocation(q.Timezone)
	if err != nil {
		return false
	}
	start, ok := parseHHMM(q.Start)
	if !ok {
		return false
	}
	end, ok := parseHHMM(q.End)
	if !ok {
		return false
	}
	if start == end {
		return false // a zero-length window mutes nothing
	}

	local := now.In(loc)
	mins := local.Hour()*60 + local.Minute()

	if start < end {
		// Same-day window, e.g. 13:00 → 14:00.
		return mins >= start && mins < end
	}
	// Window crosses midnight, e.g. 22:00 → 07:00.
	return mins >= start || mins < end
}

// parseHHMM converts "HH:MM" to minutes past midnight.
func parseHHMM(s string) (int, bool) {
	t, err := time.Parse("15:04", s)
	if err != nil {
		return 0, false
	}
	return t.Hour()*60 + t.Minute(), true
}
