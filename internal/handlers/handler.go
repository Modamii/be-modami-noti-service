package handlers

import (
	"context"

	"github.com/techinsight/be-techinsights-notification-service/pkg/contract"
)

// Handler processes an event: build notification, resolve recipients, enqueue by channels.
type Handler func(ctx context.Context, e *contract.NotificationEvent) error

// Registry maps identity (string) -> Handler.
type Registry map[string]Handler

func NewRegistry() Registry {
	return make(Registry)
}

func (r Registry) Register(identity string, h Handler) {
	r[identity] = h
}

func (r Registry) Get(identity string) (Handler, bool) {
	h, ok := r[identity]
	return h, ok
}
