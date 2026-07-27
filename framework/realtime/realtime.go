// Package realtime provides explicit, in-process fan-out over WebSocket and
// server-sent events. It is intentionally a single-process primitive: use a
// broker-backed adapter when messages must cross application instances.
package realtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/Alfian57/gin-kit/framework/events"
	"github.com/coder/websocket"
	"github.com/gin-gonic/gin"
)

const (
	defaultClientBuffer = 16
	pingInterval        = 25 * time.Second
	readLimit           = 4096
)

var (
	ErrUnknownChannel = errors.New("realtime: unknown channel")
	ErrForbidden      = errors.New("realtime: channel subscription forbidden")
)

// AuthFunc decides whether the current HTTP request may subscribe to a
// private channel. It is called before the subscription is created.
type AuthFunc func(*gin.Context) error

// Options controls a Hub. A zero ClientBuffer uses the safe default of 16.
// OriginPatterns are passed to coder/websocket and match origin hosts, not
// full URLs. The request host remains allowed by the WebSocket library.
type Options struct {
	ClientBuffer   int
	OriginPatterns []string
}

type channel struct {
	private bool
	auth    AuthFunc
	clients map[*client]struct{}
}

// Hub owns named public and private channels and their current subscribers.
// Register channels during application setup, before accepting connections.
type Hub struct {
	mu        sync.RWMutex
	channels  map[string]*channel
	clients   map[*client]struct{}
	buffer    int
	origins   []string
	closed    bool
	done      chan struct{}
	closeOnce sync.Once
}

// New creates a Hub. The optional form supports New() with secure defaults
// as well as New(Options{...}). Only the first options value is used.
func New(options ...Options) *Hub {
	option := Options{}
	if len(options) > 0 {
		option = options[0]
	}
	if option.ClientBuffer <= 0 {
		option.ClientBuffer = defaultClientBuffer
	}
	return &Hub{
		channels: make(map[string]*channel),
		clients:  make(map[*client]struct{}),
		buffer:   option.ClientBuffer,
		origins:  append([]string(nil), option.OriginPatterns...),
		done:     make(chan struct{}),
	}
}

// NewHub is an alias for New.
func NewHub(options ...Options) *Hub { return New(options...) }

// Public registers name as a public channel. Re-registering a name returns an
// error so channel access cannot be changed accidentally at runtime.
func (h *Hub) Public(name string) error { return h.register(name, false, nil) }

// Private registers name as a private channel. auth must authorize every
// WebSocket or SSE subscription to that channel.
func (h *Hub) Private(name string, auth AuthFunc) error {
	if auth == nil {
		return errors.New("realtime: private channel requires an AuthFunc")
	}
	return h.register(name, true, auth)
}

func (h *Hub) register(name string, private bool, auth AuthFunc) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("realtime: channel name is required")
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, exists := h.channels[name]; exists {
		return fmt.Errorf("realtime: channel %q already registered", name)
	}
	if h.closed {
		return errors.New("realtime: hub is closed")
	}
	h.channels[name] = &channel{private: private, auth: auth, clients: make(map[*client]struct{})}
	return nil
}

// Publish sends an event to every current subscriber of channel. A client
// whose outbound queue is full is evicted rather than allowing one slow peer
// to block application work.
func (h *Hub) Publish(channelName, event string, data any) error {
	if strings.TrimSpace(event) == "" {
		return errors.New("realtime: event is required")
	}
	payload, err := json.Marshal(message{Channel: channelName, Event: event, Data: data})
	if err != nil {
		return fmt.Errorf("realtime: encode event: %w", err)
	}

	h.mu.RLock()
	channel := h.channels[channelName]
	if channel == nil {
		h.mu.RUnlock()
		return fmt.Errorf("%w: %s", ErrUnknownChannel, channelName)
	}
	clients := make([]*client, 0, len(channel.clients))
	for item := range channel.clients {
		clients = append(clients, item)
	}
	h.mu.RUnlock()

	for _, item := range clients {
		if !item.enqueue(payload) {
			h.remove(item)
		}
	}
	return nil
}

// Handler returns the WebSocket endpoint. Clients send
// {"action":"subscribe","channel":"name"} and
// {"action":"unsubscribe","channel":"name"}. Successful requests emit
// acknowledgements; malformed or unauthorized requests emit error messages.
func (h *Hub) Handler() gin.HandlerFunc {
	return func(c *gin.Context) {
		connection, err := websocket.Accept(c.Writer, c.Request, &websocket.AcceptOptions{OriginPatterns: h.originPatterns()})
		if err != nil {
			return
		}
		connection.SetReadLimit(readLimit)

		ctx, cancel := context.WithCancel(c.Request.Context())
		defer cancel()
		client := h.newClient()
		if !h.add(client) {
			_ = connection.Close(websocket.StatusGoingAway, "server shutting down")
			return
		}
		writerDone := make(chan struct{})
		go func() {
			defer close(writerDone)
			h.writeWebSocket(ctx, connection, client)
		}()
		defer func() {
			h.remove(client)
			cancel()
			<-writerDone
		}()
		go h.ping(ctx, connection, client)

		for {
			_, raw, err := connection.Read(ctx)
			if err != nil {
				return
			}
			var request subscriptionRequest
			if err := json.Unmarshal(raw, &request); err != nil {
				if !client.error("invalid_message", "message must be valid JSON") {
					h.remove(client)
					return
				}
				continue
			}
			switch request.Action {
			case "subscribe":
				if err := h.subscribe(c, client, request.Channel); err != nil {
					if !client.subscriptionError(err) {
						h.remove(client)
						return
					}
					continue
				}
				if !client.ack("subscribed", request.Channel) {
					h.remove(client)
					return
				}
			case "unsubscribe":
				h.unsubscribe(client, request.Channel)
				if !client.ack("unsubscribed", request.Channel) {
					h.remove(client)
					return
				}
			default:
				if !client.error("invalid_action", "action must be subscribe or unsubscribe") {
					h.remove(client)
					return
				}
			}
		}
	}
}

// WebSocket is an alias for Handler.
func (h *Hub) WebSocket() gin.HandlerFunc { return h.Handler() }

// SSE returns a server-sent-events endpoint. Provide one or more channel
// query values (for example ?channel=orders&channel=alerts); comma-separated
// names are also accepted. It uses the same authorization and subscription
// lifecycle as WebSocket connections.
func (h *Hub) SSE() gin.HandlerFunc {
	return func(c *gin.Context) {
		client := h.newClient()
		if !h.add(client) {
			c.Status(http.StatusServiceUnavailable)
			return
		}
		defer h.remove(client)

		for _, name := range requestedChannels(c) {
			if err := h.subscribe(c, client, name); err != nil {
				c.JSON(http.StatusForbidden, gin.H{"error": "realtime subscription forbidden"})
				return
			}
			if !client.ack("subscribed", name) {
				return
			}
		}
		if len(requestedChannels(c)) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "channel query parameter is required"})
			return
		}

		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-cache")
		c.Header("Connection", "keep-alive")
		// net/http servers often set write deadlines for normal responses. A
		// streaming response must clear it before the first event is flushed.
		_ = http.NewResponseController(c.Writer).SetWriteDeadline(time.Time{})
		c.Status(http.StatusOK)
		c.Writer.Flush()

		for {
			select {
			case <-c.Request.Context().Done():
				return
			case <-client.done:
				return
			case payload := <-client.send:
				if _, err := fmt.Fprintf(c.Writer, "event: message\ndata: %s\n\n", payload); err != nil {
					return
				}
				c.Writer.Flush()
			}
		}
	}
}

// SSEHandler is an alias for SSE.
func (h *Hub) SSEHandler() gin.HandlerFunc { return h.SSE() }

// Run waits for ctx cancellation, closes all live connections, and returns.
// Register it with application.Go("realtime", hub.Run) so it participates in
// the application's graceful shutdown.
func (h *Hub) Run(ctx context.Context) error {
	select {
	case <-ctx.Done():
		h.Close()
		return nil
	case <-h.done:
		return nil
	}
}

// Close ends all live client streams. It is safe to call more than once.
func (h *Hub) Close() {
	h.closeOnce.Do(func() {
		h.mu.Lock()
		h.closed = true
		clients := make([]*client, 0, len(h.clients))
		for item := range h.clients {
			clients = append(clients, item)
		}
		h.mu.Unlock()
		for _, item := range clients {
			h.remove(item)
		}
		close(h.done)
	})
}

// Forward subscribes to typed in-process events and publishes each value to a
// realtime channel. With no transform, the event value becomes data. The
// returned function unsubscribes the bridge from the events bus.
func Forward[T any](bus *events.Bus, hub *Hub, channel, event string, transform ...func(T) any) func() {
	return events.On(bus, func(ctx context.Context, value T) error {
		data := any(value)
		if len(transform) > 0 && transform[0] != nil {
			data = transform[0](value)
		}
		return hub.Publish(channel, event, data)
	})
}

func (h *Hub) originPatterns() []string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return append([]string(nil), h.origins...)
}

func (h *Hub) newClient() *client {
	return &client{send: make(chan []byte, h.buffer), done: make(chan struct{})}
}

func (h *Hub) add(client *client) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return false
	}
	h.clients[client] = struct{}{}
	return true
}

func (h *Hub) subscribe(request *gin.Context, client *client, name string) error {
	name = strings.TrimSpace(name)
	h.mu.RLock()
	channel := h.channels[name]
	h.mu.RUnlock()
	if channel == nil {
		return fmt.Errorf("%w: %s", ErrUnknownChannel, name)
	}
	if channel.private && channel.auth(request) != nil {
		return ErrForbidden
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return errors.New("realtime: hub is closed")
	}
	// Re-read in case registration changes between authorization and lock.
	channel = h.channels[name]
	if channel == nil {
		return fmt.Errorf("%w: %s", ErrUnknownChannel, name)
	}
	channel.clients[client] = struct{}{}
	return nil
}

func (h *Hub) unsubscribe(client *client, name string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if channel := h.channels[strings.TrimSpace(name)]; channel != nil {
		delete(channel.clients, client)
	}
}

func (h *Hub) remove(client *client) {
	client.once.Do(func() {
		h.mu.Lock()
		delete(h.clients, client)
		for _, channel := range h.channels {
			delete(channel.clients, client)
		}
		h.mu.Unlock()
		close(client.done)
	})
}

func (h *Hub) writeWebSocket(ctx context.Context, connection *websocket.Conn, client *client) {
	for {
		select {
		case <-ctx.Done():
			_ = connection.Close(websocket.StatusGoingAway, "server shutting down")
			return
		case <-client.done:
			_ = connection.Close(websocket.StatusGoingAway, "connection closed")
			return
		case payload := <-client.send:
			if err := connection.Write(ctx, websocket.MessageText, payload); err != nil {
				h.remove(client)
				return
			}
		}
	}
}

func (h *Hub) ping(ctx context.Context, connection *websocket.Conn, client *client) {
	ticker := time.NewTicker(pingInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-client.done:
			return
		case <-ticker.C:
			pingCtx, cancel := context.WithTimeout(ctx, pingInterval)
			err := connection.Ping(pingCtx)
			cancel()
			if err != nil {
				h.remove(client)
				return
			}
		}
	}
}

type client struct {
	send chan []byte
	done chan struct{}
	once sync.Once
}

func (c *client) enqueue(payload []byte) bool {
	select {
	case <-c.done:
		return false
	default:
	}
	select {
	case c.send <- payload:
		return true
	default:
		return false
	}
}

func (c *client) ack(event, channel string) bool {
	return c.enqueue(mustJSON(message{Event: event, Channel: channel}))
}

func (c *client) error(code, text string) bool {
	return c.enqueue(mustJSON(message{Event: "error", Error: &protocolError{Code: code, Message: text}}))
}

func (c *client) subscriptionError(err error) bool {
	code := "subscription_failed"
	message := "subscription failed"
	if errors.Is(err, ErrUnknownChannel) {
		code, message = "unknown_channel", "channel is not registered"
	}
	if errors.Is(err, ErrForbidden) {
		code, message = "forbidden", "channel subscription is forbidden"
	}
	return c.error(code, message)
}

func mustJSON(value any) []byte {
	encoded, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return encoded
}

type subscriptionRequest struct {
	Action  string `json:"action"`
	Channel string `json:"channel"`
}

type message struct {
	Channel string         `json:"channel,omitempty"`
	Event   string         `json:"event"`
	Data    any            `json:"data,omitempty"`
	Error   *protocolError `json:"error,omitempty"`
}

type protocolError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func requestedChannels(c *gin.Context) []string {
	var result []string
	for _, raw := range c.QueryArray("channel") {
		for _, name := range strings.Split(raw, ",") {
			if name = strings.TrimSpace(name); name != "" {
				result = append(result, name)
			}
		}
	}
	return result
}
