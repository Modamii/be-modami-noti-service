# Services Overview

## Chức năng từng service

```
┌─────────────────────────────────────────────────────────────────┐
│                     be-modami-noti-service                      │
├──────────────┬──────────────────────────────────────────────────┤
│ cmd/api      │ REST API — client đọc/quản lý notification       │
│ :7070        │ GET notifications, mark read, preferences,       │
│              │ subscribers, lấy Centrifugo JWT token            │
├──────────────┼──────────────────────────────────────────────────┤
│ cmd/ingest   │ Nhận event từ bên ngoài vào                      │
│ :7071        │ - Kafka consumer (các service khác publish)      │
│              │ - HTTP webhook (fallback hoặc test)              │
├──────────────┼──────────────────────────────────────────────────┤
│ cmd/ws-      │ Centrifugo proxy gateway                         │
│ gateway      │ - Validate JWT khi client connect                │
│ :7072        │ - Kiểm tra quyền subscribe channel noti:*        │
│              │ - Rate limit publish từ client                   │
├──────────────┼──────────────────────────────────────────────────┤
│ cmd/worker-  │ Đọc queue notif:ws → publish lên Centrifugo      │
│ dispatch     │ → client nhận real-time qua WebSocket            │
│ :7073        │                                                  │
├──────────────┼──────────────────────────────────────────────────┤
│ cmd/worker-  │ Đọc queue notif:push → gửi FCM/Web Push          │
│ push         │ → thiết bị nhận push notification                │
│ :7074        │ (stub — chưa kết nối FCM)                        │
└──────────────┴──────────────────────────────────────────────────┘
```

---

## Flow 1 — FE connect WebSocket và nhận notification

```
┌──────┐         ┌───────────┐    ┌──────────┐    ┌─────────────┐
│  FE  │         │noti-svc   │    │Centrifugo│    │ ws-gateway  │
│      │         │api :7070  │    │  :8000   │    │   :7072     │
└──┬───┘         └─────┬─────┘    └────┬─────┘    └──────┬──────┘
   │                   │               │                  │
   │ POST /auth/       │               │                  │
   │ centrifugo-token  │               │                  │
   │──────────────────▶│               │                  │
   │◀── {token: JWT} ──│               │                  │
   │                   │               │                  │
   │ WS connect        │               │                  │
   │ {token: JWT} ─────────────────────▶                  │
   │                   │               │ POST /connect    │
   │                   │               │─────────────────▶│
   │                   │               │◀── {user:user-2} │
   │◀── connected ─────────────────────│                  │
   │                   │               │                  │
   │ subscribe         │               │                  │
   │ noti:user:{id} ───────────────────▶                  │
   │◀── subscribed ────────────────────│                  │
   │                   │               │                  │
   │    (chờ realtime) │               │                  │
```

---

## Flow 2 — Core service trigger notification qua Kafka

```
┌─────────────┐   ┌──────────────┐   ┌──────────┐   ┌──────────────┐   ┌──────┐
│ be-core-svc │   │    Kafka     │   │  ingest  │   │   worker-    │   │  FE  │
│(post,order) │   │              │   │  :7071   │   │  dispatch    │   │      │
└──────┬──────┘   └──────┬───────┘   └────┬─────┘   └──────┬───────┘   └──┬───┘
       │                 │                │                 │              │
       │ publish         │                │                 │              │
       │ modami.content  │                │                 │              │
       │ .published ────▶│                │                 │              │
       │                 │ consume ──────▶│                 │              │
       │                 │                │ handler()       │              │
       │                 │                │ lưu MongoDB     │              │
       │                 │                │ LPUSH notif:ws ▶│              │
       │                 │                │ LPUSH notif:push│              │
       │                 │                │                 │ BRPOP        │
       │                 │                │                 │ centrifugo   │
       │                 │                │                 │ .publish()───▶
       │                 │                │                 │  noti:user   │
       │                 │                │                 │  :user-2     │
```

---

## Flow 3 — Chat service real-time (shared Centrifugo)

```
┌──────┐    ┌─────────────┐    ┌──────────────┐    ┌──────┐
│  FE  │    │be-chat-svc  │    │  Centrifugo  │    │  FE  │
│userA │    │             │    │    :8000     │    │userB │
└──┬───┘    └──────┬──────┘    └──────┬───────┘    └──┬───┘
   │               │                  │               │
   │ POST /rooms/  │                  │               │
   │ room-1/msg ──▶│                  │               │
   │               │ lưu DB           │               │
   │               │ POST /api        │               │
   │               │ publish ────────▶│               │
   │               │ chat:room:room-1 │               │
   │               │                  │──────────────▶│
   │◀──────────────────────────────────── push msg    │
```

Chat service và Noti service dùng chung 1 Centrifugo. FE chỉ mở **1 WebSocket connection** nhưng subscribe 2 namespace:
- `noti:user:{id}` — notification cá nhân
- `chat:room:{id}` — tin nhắn chat room

---

## Flow 4 — Push notification (mobile/browser)

```
ingest :7071
  └─▶ PushDispatcher → LPUSH notif:push

worker-push :7074
  └─▶ BRPOP notif:push
  └─▶ FCM batch send → iOS / Android
  └─▶ Web Push (VAPID) → browser
```

Áp dụng khi user **không online** (WebSocket disconnected). Cả 2 channel được enqueue song song — nếu user online thì nhận qua WS, nếu offline thì nhận qua push.

---

## FE chỉ cần làm

```
App start
  └─▶ POST /v1/noti-services/auth/centrifugo-token
  └─▶ centrifuge.connect({ token })        ← 1 lần duy nhất

Nhận notification tự động
  └─▶ noti:user:{id} auto-subscribed khi connect

Mở chat room
  └─▶ centrifuge.newSubscription("chat:room:{roomId}").subscribe()

Gửi tin nhắn
  └─▶ POST /v1/chat/rooms/{id}/messages    ← qua REST, KHÔNG publish WS trực tiếp

Đọc notification
  └─▶ GET /v1/noti-services/users/{id}/notifications
  └─▶ PATCH /v1/noti-services/notifications/{id}/read
  └─▶ PATCH /v1/noti-services/users/{id}/notifications/read-all

Đăng ký push (mobile)
  └─▶ POST /v1/noti-services/users/{id}/subscribers
      { device_token, platform: "android"|"ios"|"web" }

Logout
  └─▶ DELETE /v1/noti-services/users/{id}/subscribers/{token}
  └─▶ centrifuge.disconnect()
```

---

## Kafka topics

| Topic | Produced by | Identity | Notify |
|-------|------------|----------|--------|
| `modami.content.published` | be-core / be-post | `content_published` | Followers của author |
| `modami.comment.created` | be-core / be-post | `comment_created` | Chủ bài viết |

Thêm loại notification mới → xem [development.md](development.md#adding-a-new-notification-type).

---

## Ports

| Service | Port | Giao thức |
|---------|------|-----------|
| api | 7070 | HTTP REST |
| ingest | 7071 | HTTP webhook + Kafka |
| ws-gateway | 7072 | HTTP (Centrifugo proxy) |
| worker-dispatch | 7073 | health only |
| worker-push | 7074 | health only |
| Centrifugo | 8000 | WebSocket + HTTP API |
| Centrifugo Admin | 10000 | HTTP UI |
