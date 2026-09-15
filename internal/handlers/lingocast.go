package handlers

import (
	"context"

	"be-modami-no-service/internal/service"
	"be-modami-no-service/pkg/contract"
	"be-modami-no-service/pkg/utils"
)

// Lingocast handles every lingocast identity with one implementation.
//
// Unlike the techinsight handlers, which compose copy from the event's objects,
// lingocast sends ready-made title, body and link in do[0].data. Wording is a
// product decision owned by the producing service — keeping it there means adding
// a new lingocast notification type needs an identity constant here and nothing
// else. Anything else in do[0].data is passed through to the client untouched, so
// screens can carry their own fields (videoId, challengeId, dueCount, …).
func Lingocast(svc *service.NotificationService) Handler {
	return func(ctx context.Context, e *contract.NotificationEvent) error {
		if len(e.Payload.Do) == 0 {
			return nil
		}
		do := e.Payload.Do[0]

		recipients := utils.ResolveRecipients(e)
		if len(recipients) == 0 {
			return nil
		}

		title := utils.GetStr(do.Data, "title")
		body := utils.GetStr(do.Data, "body")
		link := utils.GetStr(do.Data, "link")

		extra := make(map[string]interface{}, len(do.Data)+2)
		for k, v := range do.Data {
			extra[k] = v
		}
		extra["object_id"] = do.ID
		extra["object_type"] = do.Type

		return svc.Process(ctx, &service.NotificationParams{
			Identity: e.Identity,
			Title:    title,
			Body:     body,
			Link:     link,
			Extra:    extra,
			// Channel stays empty for per-user delivery. Events that fan out to a
			// shared channel (a challenge leaderboard) set it explicitly below.
			Channel: sharedChannel(e, do),
			UserIDs: recipients,
		})
	}
}

// sharedChannel returns the channel a broadcast event should land on instead of
// each recipient's personal channel. Only challenge leaderboards use one today,
// and the object the event is about is the challenge itself.
func sharedChannel(e *contract.NotificationEvent, do contract.EventObject) string {
	if e.Identity != contract.ChallengeLeaderboard {
		return ""
	}
	return do.ID
}
