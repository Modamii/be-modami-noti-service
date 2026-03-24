package handlers

import (
	"context"
	"fmt"

	"github.com/techinsight/be-techinsights-notification-service/internal/service"
	"github.com/techinsight/be-techinsights-notification-service/pkg/contract"
)

// ContentPublished extracts event-specific data and delegates to NotificationService.
func ContentPublished(svc *service.NotificationService) Handler {
	return func(ctx context.Context, e *contract.NotificationEvent) error {
		payload := &e.Payload
		if len(payload.Do) == 0 {
			return nil
		}
		do := payload.Do[0]

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

		return svc.Process(ctx, &service.NotificationParams{
			Identity: e.Identity,
			Title:    title,
			Body:     body,
			Link:     link,
			Extra: map[string]interface{}{
				"title": title, "body": body, "link": link,
				"content_id": do.ID, "content_type": do.Type, "actor_id": payload.Actor.ID,
			},
			UserIDs: userIDs,
		})
	}
}
