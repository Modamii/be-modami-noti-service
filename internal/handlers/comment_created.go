package handlers

import (
	"context"
	"fmt"

	"be-modami-no-service/internal/service"
	"be-modami-no-service/pkg/contract"
)

// CommentCreated extracts event-specific data and delegates to NotificationService.
func CommentCreated(svc *service.NotificationService) Handler {
	return func(ctx context.Context, e *contract.NotificationEvent) error {
		payload := &e.Payload
		if len(payload.Do) == 0 {
			return nil
		}
		do := payload.Do[0]

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

		return svc.Process(ctx, &service.NotificationParams{
			Identity: e.Identity,
			Title:    title,
			Body:     body,
			Link:     link,
			Extra: map[string]interface{}{
				"title": title, "body": body, "link": link,
				"comment_id": do.ID, "actor_id": payload.Actor.ID,
			},
			UserIDs: userIDs,
		})
	}
}
