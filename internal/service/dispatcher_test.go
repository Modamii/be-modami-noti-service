package service

import (
	"testing"
	"time"

	"be-modami-no-service/internal/domain"
	"be-modami-no-service/pkg/centrifugo"
	"be-modami-no-service/pkg/contract"
)

func TestBasePayloadCarriesRenderableFieldsAndHidesTokens(t *testing.T) {
	p := basePayload(&NotificationParams{
		Identity: contract.VideoReady,
		Title:    "Video ready",
		Body:     "Your video is ready to study",
		Link:     "/videos/v-1",
		Extra: map[string]interface{}{
			"video_id":      "v-1",
			"device_tokens": []string{"tok-1"},
		},
	})

	for key, want := range map[string]interface{}{
		"event_type": contract.VideoReady,
		"title":      "Video ready",
		"body":       "Your video is ready to study",
		"link":       "/videos/v-1",
		"video_id":   "v-1",
	} {
		if p[key] != want {
			t.Errorf("payload[%q] = %v, want %v", key, p[key], want)
		}
	}
	if _, leaked := p["device_tokens"]; leaked {
		t.Error("device_tokens must never reach a client")
	}
}

func TestBasePayloadDoesNotShareExtraBetweenMessages(t *testing.T) {
	params := &NotificationParams{Extra: map[string]interface{}{"k": "v"}}

	first := basePayload(params)
	first["id"] = "only-mine"

	if _, bled := basePayload(params)["id"]; bled {
		t.Error("per-recipient fields bled into another recipient's payload")
	}
	if _, mutated := params.Extra["id"]; mutated {
		t.Error("basePayload mutated the shared Extra map")
	}
}

func TestBasePayloadOmitsEmptyLink(t *testing.T) {
	if _, ok := basePayload(&NotificationParams{Identity: contract.StreakAtRisk})["link"]; ok {
		t.Error("empty link should be omitted, not sent as an empty string")
	}
}

func TestDispatchRoomIDs(t *testing.T) {
	// Per-user delivery addresses each recipient's own channel; a broadcast
	// addresses the shared challenge channel once.
	perUser := &NotificationParams{
		Identity:    contract.VideoReady,
		UserIDs:     []string{"u-1"},
		Persisted:   map[string]*domain.Notification{"u-1": {ID: "n-1", CreatedAt: time.Now()}},
		UnreadCount: map[string]int64{"u-1": 7},
	}
	if got, want := centrifugo.ChannelFromRoomID(centrifugo.KindUser+":u-1"), centrifugo.NotiChannel("u-1"); got != want {
		t.Fatalf("per-user room id resolves to %q, want %q", got, want)
	}
	if perUser.Channel != "" {
		t.Fatal("per-user params must not set Channel")
	}

	broadcast := &NotificationParams{Identity: contract.ChallengeLeaderboard, Channel: "chl-1"}
	if got, want := centrifugo.ChannelFromRoomID(centrifugo.KindChallenge+":"+broadcast.Channel),
		centrifugo.ChallengeChannel("chl-1"); got != want {
		t.Fatalf("broadcast room id resolves to %q, want %q", got, want)
	}
}

func TestWithRecipientsKeepsEveryOtherField(t *testing.T) {
	orig := &NotificationParams{
		Identity: contract.ChallengeLeaderboard,
		Title:    "t",
		Body:     "b",
		Link:     "/l",
		Channel:  "chl-1",
		Extra:    map[string]interface{}{"k": "v"},
		UserIDs:  []string{"u-1", "u-2"},
	}
	got := orig.withRecipients([]string{"u-2"})

	if len(got.UserIDs) != 1 || got.UserIDs[0] != "u-2" {
		t.Fatalf("recipients = %v, want [u-2]", got.UserIDs)
	}
	// Channel is the field most recently added to the pipeline — if the copy
	// helper ever regresses to field-by-field rebuilding, this catches it.
	if got.Channel != orig.Channel || got.Identity != orig.Identity ||
		got.Title != orig.Title || got.Body != orig.Body || got.Link != orig.Link {
		t.Errorf("filtering recipients dropped a field: %+v", got)
	}
	if len(orig.UserIDs) != 2 {
		t.Error("withRecipients must not mutate the original")
	}
}

func TestTransientIdentitiesAreInAppOnlyAndUnstored(t *testing.T) {
	// A transient identity that also pushed would ring the phone on every
	// pipeline tick, and one that was stored would flood the notification list.
	for identity := range contract.TransientIdentities {
		if !contract.IsTransient(identity) {
			t.Errorf("%s: IsTransient disagrees with the table", identity)
		}
		channels := contract.IdentityChannels[identity]
		if len(channels) != 1 || channels[0] != contract.ChannelInApp {
			t.Errorf("%s: channels = %v, want [in_app] only", identity, channels)
		}
	}

	// Everything else must stay storable, or the bell would be empty.
	for _, identity := range contract.AllIdentities {
		if contract.IsTransient(identity) {
			continue
		}
		if len(contract.IdentityChannels[identity]) == 0 {
			t.Errorf("%s: no delivery channels — the notification would go nowhere", identity)
		}
	}
}
