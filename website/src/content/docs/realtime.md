---
title: Realtime updates
description: Explicit WebSocket and server-sent event fan-out.
---

`framework/realtime` delivers in-process events to WebSocket and SSE clients.
It is deliberately explicit: the application registers named channels, decides
who may subscribe to private channels, and publishes each event itself. It is
not a distributed broker; use a broker-backed adapter when your application
runs on more than one process.

```go
hub := realtime.New(realtime.Options{OriginPatterns: []string{"app.example.com"}})
must(hub.Public("orders"))
must(hub.Private("admin", func(c *gin.Context) error {
    if auth.UserID(c) == "" { return errors.New("sign in required") }
    return nil
}))
app.Router().GET("/ws", hub.WebSocket())
app.Router().GET("/events", hub.SSE())
app.Go("realtime", hub.Run)
```

`OriginPatterns` is passed to the WebSocket acceptor and matches origin hosts,
not full URLs. Leave it empty to accept only the request host. Avoid `*`:
unrestricted cross-origin WebSockets expose authenticated browser users to
cross-site request forgery.

## WebSocket protocol

Clients send `{"action":"subscribe","channel":"orders"}` or
`{"action":"unsubscribe","channel":"orders"}`. Successful actions send
an acknowledgement. Published messages use the channel/event/data shape:

```json
{"channel":"orders","event":"created","data":{"id":"42"}}
```

Protocol failures return `event: "error"` with `invalid_message`,
`invalid_action`, `unknown_channel`, `forbidden`, or `subscription_failed`.
Authorization errors are deliberately not forwarded to the client.

```go
if err := hub.Publish("orders", "created", order); err != nil { return err }
```

Each connection has a 16-message outbound buffer by default (change it with
`Options.ClientBuffer`). A client that cannot keep up is disconnected; a slow
browser can never hold up a publish call. WebSocket input is limited to 4096
bytes and the server sends a ping every 25 seconds.

## Server-sent events

The SSE endpoint shares the exact channel authorization and subscription
bookkeeping. Subscribe in the request URL, repeating `channel` when needed:

```text
GET /events?channel=orders&channel=alerts
Accept: text/event-stream
```

It emits `event: message` frames whose `data` is the same JSON message used by
WebSockets, including subscription acknowledgements. The handler clears the
HTTP write deadline before it starts streaming.

## Forward typed application events

Bridge the dependency-free event bus without hidden registration. The returned
function disconnects the bridge when a test or feature is torn down.

```go
stop := realtime.Forward[OrderCreated](bus, hub, "orders", "created",
    func(event OrderCreated) any { return map[string]any{"id": event.OrderID} },
)
defer stop()
```

Omit the transform to publish the typed event value itself:

```go
stop := realtime.Forward[OrderCreated](bus, hub, "orders", "created")
```
