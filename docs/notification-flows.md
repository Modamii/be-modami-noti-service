<title>Notification Flows</title>

# Notification & Realtime Flows

Reference for every path a notification or realtime event takes across the three
repos: `be-lingocast-core-service` (producer), `be-modami-noti-service`
(pipeline + delivery), `fe-lingocast-mobile-service` (client). Pairs with
[`CONTRACT.md`](./CONTRACT.md) (envelope shape, identity→channel table) and
[`observability.md`](./observability.md) (what to alert on when a flow breaks).

Status markers used below:

- ✅ **live-verified** — traced end to end on a running local stack (2026-09-17)
- 🔧 **code-complete, not yet live-tested** — implemented, unit-tested, never run against real infra

---

## 1. Who owns what

```mermaid
flowchart LR
    subgraph LC["be-lingocast-core-service"]
        SVC["Session / Quest /\nChallenge / video-status\nconsumer"] -->|"Create() / Emit()\nEmitBroadcast()"| NP["NotificationProducer"]
        NP -->|"enqueue\nnotification:deliver"| AQ[("Redis · asynq")]
        AQ -->|"dequeue"| JS["job_scheduler"]
    end

    JS -->|"POST /webhook\n(actor/do/io/po envelope)"| ING

    subgraph NS["be-modami-noti-service"]
        ING["ingest :7071"] --> PIPE{"NotificationService\n.Process()"}
        PIPE -->|"persist\n(skipped if transient)"| MDB[("MongoDB")]
        PIPE -->|"per channel:\nmute → channel-on → quiet-hours"| GATE{"preference\ngate"}
        GATE -->|"in_app"| WSQ[("Redis\nnotif:ws")]
        GATE -->|"push"| PSQ[("Redis\nnotif:push")]
        WSQ --> WD["worker-dispatch"]
        PSQ --> WP["worker-push"]
        API["api :7070\nlist / read / prefs / token"] --- MDB
        WSG["ws-gateway :7072\nconnect / subscribe / publish"]
    end

    WD -->|"publish"| CF["Centrifugo"]
    WP -->|"send"| FCM["FCM"]
    CF -.->|"authorize every\nsubscribe & publish"| WSG

    CF -->|"noti:user:{id}\nnoti:challenge:{id}"| APP["FE app\nRealtimeProvider"]
    FCM -->|"push"| DEV["Device\nPushProvider"]
    APP -->|"POST /auth/centrifugo-token"| API
```

The one thing worth remembering from this picture: **lingocast never talks to
Centrifugo, Mongo, or FCM directly.** It only ever does two things — POST an
envelope to `ingest`, or answer a `/challenges/{id}/realtime-token` request
using a secret it shares with noti-service. Everything downstream of `ingest`
is noti-service's problem, which is the whole point of the split.

---

## 2. Standard notification — persisted, dual-channel ✅

The path for anything a user should be able to open later: `video_ready`,
`flashcard_due`, `achievement_unlocked`, `challenge_ended`. Traced live today
with a `video_ready` envelope; see the walkthrough below the diagram.

```mermaid
sequenceDiagram
    participant Src as Lingocast service
    participant NP as NotificationProducer
    participant AQ as Redis (asynq)
    participant JS as job_scheduler
    participant ING as ingest
    participant Svc as NotificationService
    participant DB as MongoDB
    participant WSQ as Redis notif:ws
    participant WD as worker-dispatch
    participant CF as Centrifugo
    participant App as FE app
    participant PSQ as Redis notif:push
    participant WP as worker-push
    participant FCM as FCM
    participant Dev as Device

    Src->>NP: Create(userID, video_ready, title, body)
    NP->>AQ: enqueue notification:deliver
    AQ->>JS: dequeue
    JS->>ING: POST /webhook (envelope)
    ING->>Svc: Process()
    Svc->>DB: insert notification (read:false)
    Svc->>Svc: IdentityChannels(video_ready) = (in_app, push)

    par in_app
        Svc->>Svc: allowed(pref, video_ready, in_app, now)
        Svc->>WSQ: LPUSH (id, title, body, link, unread_count)
        WSQ->>WD: BRPOP
        WD->>CF: publish noti:user:id
        CF->>App: push (if subscribed)
        App->>App: setQueryData - bell + badge update
    and push
        Svc->>Svc: allowed(pref, video_ready, push, now)
        Note over Svc: quiet hours checked here only
        Svc->>Svc: enrich - device tokens
        Svc->>PSQ: LPUSH (tokens, title, body, data)
        PSQ->>WP: BRPOP
        WP->>FCM: send multicast
        FCM->>Dev: system push
        Dev->>Dev: notifee shows it (if app backgrounded)
    end
```

**Verified today:** the `in_app` half, exactly as drawn — webhook → Mongo
insert → `notif:ws` → `worker-dispatch` → Centrifugo → a subscribed WebSocket
client received `{event_type, title, body, link, id, created_at,
unread_count}` in real time, and `GET /notifications` showed the same row.
Ran it twice with two different identities (`video_ready`, then
`flashcard_due`) to confirm `unread_count` tracks the real total rather than
being hardcoded per event — second run correctly showed `1`, not `2`, because
the first notification had already been read by then.

**Verified 2026-09-18:** `worker-push` now has real FCM service-account
credentials (`config/firebase-credentials.json`, gitignored) and confirmed
auth against Firebase's API directly — a probe send to a bogus token came
back `Unregistered`, not an auth error, which only happens once Firebase has
actually authenticated the request. `SendEachForMulticast` batches ≤500
tokens per call; the client-side leg of this diagram (`FCM → Dev →
notifee`) still cannot be exercised end-to-end on the iOS Simulator —
`RNFBMessaging` explicitly skips `registerForRemoteNotifications` on ARM64
Simulator, so no real device token is ever minted there. Needs a physical
device to confirm the last hop.

---

## 3. Transient events skip half the pipeline ✅

`video_progress` and `challenge_leaderboard` take the *same* webhook →
`NotificationService.Process()` entry point as §2, but one check changes
everything downstream:

```mermaid
flowchart TD
    A["Process() receives envelope"] --> B{"contract.IsTransient\n(identity)?"}
    B -->|"no — video_ready,\nflashcard_due, …"| C["persist to Mongo"]
    C --> D["compute unread_count"]
    D --> E["dispatch to IdentityChannels\n(in_app + push)"]
    B -->|"yes — video_progress,\nchallenge_leaderboard"| F["skip persist,\nskip unread_count"]
    F --> G["dispatch to IdentityChannels\n(in_app only)"]
    E --> H["client: bell list + badge"]
    G --> I["client: isTransient() check\n→ never touches the bell,\ninvalidates a live query instead"]
```

**Why this exists:** a single video import fires roughly six `video_progress`
events. Persisting them would leave six unread rows in the bell for one
import; pushing them would ring the phone on every pipeline tick. The
transient set is declared once, in `contract.TransientIdentities`
(noti-service) and mirrored in `lib/notification-events.ts` (FE) — both sides
are covered by a test that keeps them in sync.

**Verified today, both halves:** fired a `video_progress` envelope — the row
count from `GET /notifications` stayed at 2 (no insert), then fired a second
one with a listener attached and watched it arrive live with a bare payload
(`event_type`, `status`, `step`, `videoId` — no `id`, no `created_at`, no
`unread_count`), confirming the dispatcher genuinely skips enrichment for
transient identities rather than just omitting the Mongo write.

---

## 4. Challenge live leaderboard — membership-gated broadcast

**Channel security: ✅ live-verified. Lingocast's HTTP surface: 🔧 blocked locally.**

The one flow that isn't per-user. noti-service can't decide who may watch a
challenge — it doesn't know what a challenge is — so Lingocast decides and
vouches for it with a signed token.

```mermaid
sequenceDiagram
    participant App as FE app
    participant LC as Lingocast API
    participant Part as GroupChallengeParticipantRepo
    participant CF as Centrifugo
    participant WSG as ws-gateway
    participant Sess as SessionService
    participant Chal as ChallengeService
    participant NP as NotificationProducer

    App->>LC: GET /challenges/id/realtime-token
    LC->>Part: ListByChallenge(id)
    alt caller not a participant
        LC-->>App: 403 join the challenge to watch it live
    else caller joined
        LC->>LC: sign JWT (sub, channel: noti:challenge:id)<br/>with noti_service.centrifugo_hmac_secret
        LC-->>App: token, channel, expiresAt
        App->>CF: subscribe(noti:challenge:id, token)
        CF->>WSG: POST /centrifugo/subscribe
        WSG->>WSG: ChannelPolicy.Allow -<br/>sub == token.sub AND channel == token.channel
        WSG-->>CF: allowed
        CF-->>App: subscribed
    end

    Note over Sess,Chal: later — a participant finishes a session
    Sess->>Chal: RecordSessionCompletion(challengeID, userID, accuracy)
    Chal->>Chal: shouldBroadcast? (2s window per challenge)
    alt window open
        Chal->>Part: ListByChallenge → recipients
        Chal->>NP: EmitBroadcast(challenge_leaderboard, challengeID, recipients)
        Note over NP: same ingest → dispatch pipeline as §2,<br/>but Channel is set → goes to the<br/>shared room, not each user's personal one
    else window still closed
        Chal->>Chal: drop — a broadcast already went out<br/>in the last 2s
    end
```

The broadcast payload carries only the challenge id — clients refetch the
standings themselves, so the event never has to mirror the leaderboard's
shape. The 2-second window is per-process (`sync.Map`, not Redis); noted with
a `ponytail:` comment naming the upgrade path if two API pods ever make that
visible.

**Why the split status.** Lingocast's own auth is wired to a real
`user-service` over gRPC locally (`localhost:50050` is live), so a crafted
unverified JWT — which works fine for testing noti-service directly — gets
rejected: the auth middleware calls `GetUserBasic("test-user-1")` and gets
`NotFound`. Exercising `GET /challenges/{id}/realtime-token` for real needs a
genuine logged-in account, which this session doesn't have.

What *was* tested, by minting subscription tokens directly with the same
`local-dev-token-secret` Lingocast signs with (bypassing the HTTP layer, not
the cryptography) — the actual security-critical mechanism, live against a
running Centrifugo:

| Token | Result |
|---|---|
| `sub` matches connected user, `channel` matches subscribed channel | ✅ subscribed |
| `sub` = a different user | ❌ disconnected — `"token user mismatch"` |
| `channel` = a different challenge | ❌ disconnected — `"token channel mismatch"` |
| signed with the wrong secret entirely | ❌ `code 103 "permission denied"` — falls through to `ChannelPolicy.Allow`, which independently rejects it |

The last row is worth noting: Centrifugo verifies `sub`/`channel` tokens
natively when the signature checks out (rows 1–3 never reach `ws-gateway` at
all), but a token that fails *Centrifugo's own* verification falls through to
the subscribe-proxy anyway — where `ChannelPolicy.Allow` rejects it a second,
independent way. Two layers, not one with a silent bypass.

Then subscribed with the valid token and fired a real `challenge_leaderboard`
broadcast at the webhook — it arrived on `noti:challenge:chl-demo-1` with
exactly the bare shape the design calls for: `event_type`, `challengeId` — no
`id`, `unread_count`, or `created_at`, confirming the broadcast path really is
a separate code path from the per-user one in §2, not the same one with a
field omitted by chance.

---

## 5. Cross-device read sync ✅

```mermaid
sequenceDiagram
    participant A as Device A
    participant API as noti-service api
    participant DB as MongoDB
    participant RS as ReadSyncPublisher
    participant WSQ as Redis notif:ws
    participant WD as worker-dispatch
    participant CF as Centrifugo
    participant B as Device B (same user)

    A->>API: PATCH /notifications/id/read
    API->>DB: set read=true (scoped to owner)
    API->>DB: CountUnread(userID)
    API->>RS: NotificationRead(userID, id, unread)
    RS->>WSQ: LPUSH (event: notification_read, id, unread_count)
    WSQ->>WD: BRPOP
    WD->>CF: publish noti:user:id
    CF->>B: push
    B->>B: applyRead() - flip that row, badge follows
    Note over A,B: mark-all-read is the same shape,<br/>event: notification_read_all, unread_count: 0
```

**Root-caused and confirmed today.** First pass looked broken: a `redis
MONITOR` capture during `PATCH .../read` showed zero `LPUSH` activity — only
`worker-dispatch`'s own polling. Not a code bug: the running `api` process had
been started *before* `config/config.yaml`'s `redis.pass` got the container's
real password added, so its Redis ping failed at boot and it silently
degraded to `readSync == nil` (by design — a config error there shouldn't
crash the whole API). Confirmed by restarting the process: no more
`"read-sync disabled"` warning at boot, and the retry produced exactly the
designed payload —

```json
{"event":"notification_read","payload":{"id":"f3c3e5fb-…","unread_count":0}}
```

on the same personal channel, live. The lesson for next time this looks
broken: check the `api` boot log for `"read-sync disabled: redis
unavailable"` before assuming the read-sync code itself regressed — it fails
silent by design, not loud.

---

## 5.5 Foreground banner — socket and push never double-announce ✅

Both legs of §2 can independently make the app show something while it's
open: `worker-dispatch`→Centrifugo delivers a live `publication`, and
`worker-push`→FCM delivers a push the OS would normally suppress in the
foreground. Showing a banner for both would announce the same event twice,
so `realtime.connected` is the single tie-breaker — whichever leg the socket
state favors wins, and the other silently no-ops.

```mermaid
flowchart LR
    CF["Centrifugo\npublication"] --> RTP["RealtimeProvider\nhandler"]
    RTP -->|"non-transient"| CACHE["write to\nnotification cache"]
    RTP --> DECIDE1{"isTransient?"}
    DECIDE1 -->|"no"| BANNER1["displayRealtimeBanner()"]

    FCM["FCM\nonMessage (foreground)"] --> DECIDE2{"realtime.connected?"}
    DECIDE2 -->|"yes — socket already\nshowed it"| SKIP["no-op"]
    DECIDE2 -->|"no"| BANNER2["displayForeground()"]

    BANNER1 --> PERM{"hasPushPermission()?"}
    BANNER2 --> PERM
    PERM -->|"no"| NOOP["silent no-op —\niOS throws on\ndisplayNotification()\nwithout authorization"]
    PERM -->|"yes"| NOTIFEE["notifee.displayNotification()"]
```

**Verified 2026-09-18** against a real device: with the socket connected, a
Centrifugo publish to `noti:user:{id}` produced a native-styled banner via
`notifee.displayNotification()` while the app was in the foreground — the
same call `displayForeground()` uses for FCM, so the two paths render
identically regardless of which transport delivered the event.

One gate both paths share, easy to miss: iOS only exposes a **Notifications**
row in Settings for an app *after* it has called `requestAuthorization` at
least once — before that first call there is nothing to toggle, and
`displayNotification()` throws rather than silently no-op'ing. `displayBanner()`
checks `hasPushPermission()` first specifically so a denied-or-never-asked
permission degrades to a silent no-op instead of an uncaught promise
rejection surfacing as a redbox.

---

## 6. Push device lifecycle

```mermaid
flowchart LR
    ON["Onboarding\nnotifications screen"] -->|"user opts in"| PERM["OS permission\nprompt (once, ever)"]
    PERM -->|"granted"| TOKEN["FCM getToken()"]
    TOKEN --> REG["POST /subscribers\n{device_token, platform}"]
    REG --> SUB[("Mongo subscribers\nunique(user_id, device_token)")]

    ROT["FCM onTokenRefresh"] -->|"new token"| REG

    SEND["worker-push: Send(tokens)"] -->|"per-token result"| RES{"FCM response"}
    RES -->|"UNREGISTERED /\nINVALID_ARGUMENT"| PRUNE["DeleteByToken(userID, token)"]
    RES -->|"other failure"| LOG["log, keep the token —\nmight be transient"]
    RES -->|"success"| DONE["counted in\nnotif_push_sent_total"]
    PRUNE --> SUB
```

Push is per-recipient (`PushMessage` carries one `user_id`), specifically so
`PRUNE` above knows whose token to delete — a flattened token list would lose
that.

---

## 7. Preference gate — the order matters

```mermaid
flowchart TD
    START(("notification about to\ngo out on one channel")) --> MUTE{"IsMuted(identity)?"}
    MUTE -->|"yes"| DENY(("blocked —\nno matter the channel"))
    MUTE -->|"no"| CH{"channel switch on?\n(in_app_enabled / push_enabled)"}
    CH -->|"no"| DENY
    CH -->|"yes"| Q{"channel == push?"}
    Q -->|"no (in_app)"| ALLOW(("delivered"))
    Q -->|"yes"| QUIET{"InQuietHours(now)?"}
    QUIET -->|"yes"| DENY
    QUIET -->|"no"| ALLOW
```

Quiet hours only ever gate `push` — in-app is silent and the user is already
looking at the screen, so holding it back would just make the bell lag behind
reality for no reason.

---

## 8. Identity quick reference

| Identity | Channels | Persisted? | WS target |
|---|---|---|---|
| `video_ready`, `video_failed` | in_app, push | yes | `noti:user:{id}` |
| `flashcard_due`, `achievement_unlocked`, `challenge_ended` | in_app, push | yes | `noti:user:{id}` |
| `streak_at_risk` | push only | yes | — |
| `video_progress` | in_app only | **no** | `noti:user:{id}` |
| `challenge_leaderboard` | in_app only | **no** | `noti:challenge:{id}` (broadcast) |
| `content_published`, `comment_created` | in_app, push | yes | `noti:user:{id}` |

Full field-level contract (envelope shape, `do[0].data` keys per identity)
lives in [`CONTRACT.md`](./CONTRACT.md).
