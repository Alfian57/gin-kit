package realtime

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/Alfian57/gin-kit/framework/events"
	"github.com/coder/websocket"
	"github.com/gin-gonic/gin"
)

func TestWebSocketSubscribePublishAndUnsubscribe(t *testing.T) {
	gin.SetMode(gin.TestMode)
	hub := New()
	if err := hub.Public("orders"); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(webSocketRouter(hub))
	defer server.Close()
	connection := dial(t, server.URL)
	defer connection.CloseNow()
	writeJSON(t, connection, subscriptionRequest{Action: "subscribe", Channel: "orders"})
	ack := readMessage(t, connection)
	if ack.Event != "subscribed" || ack.Channel != "orders" {
		t.Fatalf("ack = %#v", ack)
	}
	if err := hub.Publish("orders", "created", map[string]string{"id": "42"}); err != nil {
		t.Fatal(err)
	}
	event := readMessage(t, connection)
	if event.Channel != "orders" || event.Event != "created" || string(event.Data) != `{"id":"42"}` {
		t.Fatalf("event = %#v", event)
	}
	writeJSON(t, connection, subscriptionRequest{Action: "unsubscribe", Channel: "orders"})
	if got := readMessage(t, connection); got.Event != "unsubscribed" {
		t.Fatalf("unsubscribe ack = %#v", got)
	}
	if err := hub.Publish("orders", "created", map[string]string{"id": "43"}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
	defer cancel()
	if _, _, err := connection.Read(ctx); err == nil {
		t.Fatal("received an event after unsubscribe")
	}
}

func TestWebSocketPrivateChannelUsesAuthFunc(t *testing.T) {
	gin.SetMode(gin.TestMode)
	hub := New()
	if err := hub.Private("admin", func(c *gin.Context) error {
		if c.GetHeader("X-Role") == "admin" {
			return nil
		}
		return errors.New("not an admin")
	}); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(webSocketRouter(hub))
	defer server.Close()
	connection := dial(t, server.URL)
	defer connection.CloseNow()
	writeJSON(t, connection, subscriptionRequest{Action: "subscribe", Channel: "admin"})
	denied := readMessage(t, connection)
	if denied.Event != "error" || denied.Error == nil || denied.Error.Code != "forbidden" {
		t.Fatalf("denied = %#v", denied)
	}
	allowed := dialWithOptions(t, server.URL, &websocket.DialOptions{HTTPHeader: http.Header{"X-Role": []string{"admin"}}})
	defer allowed.CloseNow()
	writeJSON(t, allowed, subscriptionRequest{Action: "subscribe", Channel: "admin"})
	if got := readMessage(t, allowed); got.Event != "subscribed" {
		t.Fatalf("allowed subscription = %#v", got)
	}
}

func TestWebSocketOriginPatterns(t *testing.T) {
	gin.SetMode(gin.TestMode)
	hub := New(Options{OriginPatterns: []string{"console.example.com"}})
	server := httptest.NewServer(webSocketRouter(hub))
	defer server.Close()
	connection := dialWithOptions(t, server.URL, &websocket.DialOptions{HTTPHeader: http.Header{"Origin": []string{"https://console.example.com"}}})
	defer connection.CloseNow()
}

func TestWebSocketRejectsInvalidProtocolAndLargeMessages(t *testing.T) {
	gin.SetMode(gin.TestMode)
	hub := New()
	server := httptest.NewServer(webSocketRouter(hub))
	defer server.Close()
	connection := dial(t, server.URL)
	defer connection.CloseNow()
	if err := connection.Write(context.Background(), websocket.MessageText, []byte("not json")); err != nil {
		t.Fatal(err)
	}
	if got := readMessage(t, connection); got.Error == nil || got.Error.Code != "invalid_message" {
		t.Fatalf("invalid JSON response = %#v", got)
	}
	if err := connection.Write(context.Background(), websocket.MessageText, []byte(strings.Repeat("x", readLimit+1))); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if _, _, err := connection.Read(ctx); err == nil {
		t.Fatal("large message did not close the connection")
	}
}

func TestSSESharesSubscriptionsAndStreamsEvents(t *testing.T) {
	gin.SetMode(gin.TestMode)
	hub := New()
	if err := hub.Public("orders"); err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	router.GET("/events", hub.SSE())
	server := httptest.NewServer(router)
	defer server.Close()
	response, err := http.Get(server.URL + "/events?channel=orders")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if contentType := response.Header.Get("Content-Type"); !strings.HasPrefix(contentType, "text/event-stream") {
		t.Fatalf("Content-Type = %q", contentType)
	}
	reader := bufio.NewReader(response.Body)
	ack := readSSE(t, reader)
	if ack.Event != "subscribed" || ack.Channel != "orders" {
		t.Fatalf("SSE ack = %#v", ack)
	}
	if err := hub.Publish("orders", "created", map[string]string{"id": "42"}); err != nil {
		t.Fatal(err)
	}
	if got := readSSE(t, reader); got.Event != "created" || got.Channel != "orders" {
		t.Fatalf("SSE event = %#v", got)
	}
}

func TestPublishEvictsSlowClients(t *testing.T) {
	hub := New(Options{ClientBuffer: 1})
	if err := hub.Public("orders"); err != nil {
		t.Fatal(err)
	}
	client := hub.newClient()
	if !hub.add(client) {
		t.Fatal("client was not added")
	}
	hub.mu.Lock()
	hub.channels["orders"].clients[client] = struct{}{}
	hub.mu.Unlock()
	if err := hub.Publish("orders", "created", 1); err != nil {
		t.Fatal(err)
	}
	if err := hub.Publish("orders", "created", 2); err != nil {
		t.Fatal(err)
	}
	select {
	case <-client.done:
	default:
		t.Fatal("slow client was not evicted")
	}
	hub.mu.RLock()
	defer hub.mu.RUnlock()
	if len(hub.clients) != 0 || len(hub.channels["orders"].clients) != 0 {
		t.Fatal("evicted client remains subscribed")
	}
}

func TestForwardBridgesTypedEventsAndUnsubscribes(t *testing.T) {
	type created struct{ ID string }
	hub := New()
	if err := hub.Public("orders"); err != nil {
		t.Fatal(err)
	}
	client := hub.newClient()
	if !hub.add(client) {
		t.Fatal("client was not added")
	}
	hub.mu.Lock()
	hub.channels["orders"].clients[client] = struct{}{}
	hub.mu.Unlock()
	bus := events.NewBus()
	stop := Forward[created](bus, hub, "orders", "created", func(value created) any { return map[string]string{"id": value.ID} })
	if err := events.Emit(context.Background(), bus, created{ID: "42"}); err != nil {
		t.Fatal(err)
	}
	var got wireMessage
	if err := json.Unmarshal(<-client.send, &got); err != nil {
		t.Fatal(err)
	}
	if got.Event != "created" || string(got.Data) != `{"id":"42"}` {
		t.Fatalf("forwarded event = %#v", got)
	}
	stop()
	if err := events.Emit(context.Background(), bus, created{ID: "43"}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-client.send:
		t.Fatal("forward bridge remained subscribed")
	default:
	}
}

func TestRunClosesClientsOnContextCancellation(t *testing.T) {
	hub := New()
	client := hub.newClient()
	if !hub.add(client) {
		t.Fatal("client was not added")
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- hub.Run(ctx) }()
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	select {
	case <-client.done:
	default:
		t.Fatal("Run did not close client")
	}
}

func TestChannelRegistrationAndPublishErrors(t *testing.T) {
	hub := New()
	if err := hub.Public(""); err == nil {
		t.Fatal("empty channel was accepted")
	}
	if err := hub.Private("secret", nil); err == nil {
		t.Fatal("private channel without auth was accepted")
	}
	if err := hub.Public("orders"); err != nil {
		t.Fatal(err)
	}
	if err := hub.Public("orders"); err == nil {
		t.Fatal("duplicate channel was accepted")
	}
	if err := hub.Publish("missing", "created", nil); !errors.Is(err, ErrUnknownChannel) {
		t.Fatalf("publish error = %v", err)
	}
}

func webSocketRouter(hub *Hub) *gin.Engine {
	router := gin.New()
	router.GET("/ws", hub.Handler())
	return router
}

func dial(t *testing.T, serverURL string) *websocket.Conn {
	return dialWithOptions(t, serverURL, nil)
}

func dialWithOptions(t *testing.T, serverURL string, options *websocket.DialOptions) *websocket.Conn {
	t.Helper()
	address, err := url.Parse(serverURL)
	if err != nil {
		t.Fatal(err)
	}
	address.Scheme, address.Path = "ws", "/ws"
	connection, _, err := websocket.Dial(context.Background(), address.String(), options)
	if err != nil {
		t.Fatal(err)
	}
	return connection
}

func writeJSON(t *testing.T, connection *websocket.Conn, value any) {
	t.Helper()
	payload, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := connection.Write(context.Background(), websocket.MessageText, payload); err != nil {
		t.Fatal(err)
	}
}

type wireMessage struct {
	Channel string          `json:"channel"`
	Event   string          `json:"event"`
	Data    json.RawMessage `json:"data"`
	Error   *protocolError  `json:"error"`
}

func readMessage(t *testing.T, connection *websocket.Conn) wireMessage {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, payload, err := connection.Read(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var message wireMessage
	if err := json.Unmarshal(payload, &message); err != nil {
		t.Fatal(err)
	}
	return message
}

func readSSE(t *testing.T, reader *bufio.Reader) wireMessage {
	t.Helper()
	var data string
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatal(err)
		}
		line = strings.TrimSuffix(line, "\n")
		if line == "" {
			break
		}
		if strings.HasPrefix(line, "data: ") {
			data = strings.TrimPrefix(line, "data: ")
		}
	}
	var message wireMessage
	if err := json.Unmarshal([]byte(data), &message); err != nil {
		t.Fatal(err)
	}
	return message
}
