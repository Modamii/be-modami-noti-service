package handlers

import (
	"context"
	"fmt"

	"github.com/techinsight/be-techinsights-notification-service/internal/queue"
	"github.com/techinsight/be-techinsights-notification-service/pkg/contract"
	"github.com/techinsight/be-techinsights-notification-service/pkg/event"
)

// ContentPublished handles identity content_published: build in_app + push from envelope payload.
// Recipients from extra.To or derived from payload (e.g. audience in do[0].data).
func ContentPublished(q *queue.Queue, queueWS, queuePush string) Handler {
	return func(ctx context.Context, e *contract.NotificationEvent) error {
		payload := &e.Payload
		if len(payload.Do) == 0 {
			return nil
		}
		do := payload.Do[0]
		actorID := payload.Actor.ID
		title := getStr(do.Data, "title")
		if title == "" {
			title = "New content"
		}
		body := getStr(do.Data, "body")
		link := fmt.Sprintf("/%ss/%s", do.Type, do.ID)

		userIDs := resolveRecipients(e)
		if len(userIDs) == 0 {
			return nil
		}

		inApp := map[string]interface{}{
			"title": title, "body": body, "link": link,
			"content_id": do.ID, "content_type": do.Type, "actor_id": actorID,
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

// resolveRecipients returns user IDs from extra.To or payload (e.g. audience_ids in do[0].data).
func resolveRecipients(e *contract.NotificationEvent) []string {
	if e.Extra != nil && e.Extra.To != nil {
		return sliceFromInterface(e.Extra.To)
	}
	if len(e.Payload.Do) == 0 {
		return nil
	}
	data := e.Payload.Do[0].Data
	if data == nil {
		return nil
	}
	if ids, ok := data["audience_ids"]; ok {
		return sliceFromInterface(ids)
	}
	return nil
}

func getStr(m map[string]interface{}, key string) string {
	if m == nil {
		return ""
	}
	v, _ := m[key].(string)
	return v
}

func sliceFromInterface(v interface{}) []string {
	switch x := v.(type) {
	case []string:
		return x
	case []interface{}:
		var out []string
		for _, i := range x {
			if s, ok := i.(string); ok {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}
