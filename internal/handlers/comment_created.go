package handlers

import (
	"context"
	"fmt"

	"github.com/techinsight/be-techinsights-notification-service/internal/queue"
	"github.com/techinsight/be-techinsights-notification-service/pkg/contract"
	"github.com/techinsight/be-techinsights-notification-service/pkg/event"
)

// CommentCreated handles identity comment_created: actor (user) do (comment) io (post/article/comment parent).
// Recipients from extra.To or derived from io (content owner / participants).
func CommentCreated(q *queue.Queue, queueWS, queuePush string) Handler {
	return func(ctx context.Context, e *contract.NotificationEvent) error {
		payload := &e.Payload
		if len(payload.Do) == 0 {
			return nil
		}
		do := payload.Do[0]
		actorID := payload.Actor.ID
		title := getStr(do.Data, "title")
		if title == "" {
			title = "New comment"
		}
		body := getStr(do.Data, "content")
		if body == "" {
			body = getStr(do.Data, "body")
		}
		link := fmt.Sprintf("/comments/%s", do.ID)

		userIDs := resolveRecipients(e)
		if len(userIDs) == 0 {
			return nil
		}

		inApp := map[string]interface{}{
			"title": title, "body": body, "link": link,
			"comment_id": do.ID, "actor_id": actorID,
		}

		channels := contract.IdentityChannels[e.Identity]
		for _, ch := range channels {
			switch ch {
			case contract.ChannelInApp:
				for _, uid := range userIDs {
					roomID := "user:" + uid
					wsMsg := event.WSMessage{RoomID: roomID, Event: e.Identity, Payload: inApp}
					if err := q.Enqueue(ctx, queueWS, wsMsg); err != nil {
						return err
					}
				}
			case contract.ChannelPush:
				pushMsg := event.PushMessage{
					DeviceTokens: nil,
					Title:        title,
					Body:         body,
					Link:         link,
				}
				if err := q.Enqueue(ctx, queuePush, pushMsg); err != nil {
					return err
				}
			}
		}
		return nil
	}
}
