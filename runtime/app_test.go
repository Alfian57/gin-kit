package runtime

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Alfian57/gin-kit/runtime/httpx"
	"github.com/Alfian57/gin-kit/runtime/queue"
	"github.com/Alfian57/gin-kit/runtime/validation"
	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

func TestNewInstallsHealthSecurityAndRequestIDDefaults(t *testing.T) {
	app, err := New(Options{})
	if err != nil {
		t.Fatal(err)
	}
	app.Router().GET("/hello", func(c *gin.Context) {
		httpx.OK(c, gin.H{"hello": "world"})
	})
	recorder := httptest.NewRecorder()
	app.Router().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/hello", nil))
	if recorder.Code != http.StatusOK || recorder.Header().Get("X-Request-ID") == "" {
		t.Fatalf("unexpected response: %d headers=%v", recorder.Code, recorder.Header())
	}
	if recorder.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal("security headers not installed")
	}
}

func TestBindersUseOptionsValidator(t *testing.T) {
	custom := validation.New()
	if err := custom.RegisterRule("adult", func(fl validator.FieldLevel) bool {
		return fl.Field().Int() >= 18
	}); err != nil {
		t.Fatal(err)
	}
	custom.RegisterMessage("adult", "The {field} field requires an adult.")
	app, err := New(Options{Validator: custom})
	if err != nil {
		t.Fatal(err)
	}
	type signupInput struct {
		Age int `json:"age" validate:"adult"`
	}
	app.Router().POST("/signups", func(c *gin.Context) {
		input, ok := httpx.BindJSON[signupInput](c)
		if !ok {
			return
		}
		httpx.Created(c, input)
	})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/signups", strings.NewReader(`{"age":12}`))
	request.Header.Set("Content-Type", "application/json")
	app.Router().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusUnprocessableEntity ||
		!strings.Contains(recorder.Body.String(), "The age field requires an adult.") {
		t.Fatalf("binder ignored Options.Validator: %d %s", recorder.Code, recorder.Body)
	}
}

func TestRecoveryUsesStableInternalError(t *testing.T) {
	app, err := New(Options{})
	if err != nil {
		t.Fatal(err)
	}
	app.Router().GET("/panic", func(*gin.Context) { panic("secret panic") })
	recorder := httptest.NewRecorder()
	app.Router().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/panic", nil))
	if recorder.Code != http.StatusInternalServerError || strings.Contains(recorder.Body.String(), "secret panic") {
		t.Fatalf("panic leaked or wrong status: %d %s", recorder.Code, recorder.Body)
	}
}

func TestNotFoundAndMethodNotAllowedUseCanonicalEnvelope(t *testing.T) {
	app, err := New(Options{})
	if err != nil {
		t.Fatal(err)
	}
	app.Router().GET("/known", func(c *gin.Context) { httpx.OK(c, "ok") })
	for _, test := range []struct {
		method string
		path   string
		status int
		code   string
	}{
		{http.MethodGet, "/missing", http.StatusNotFound, "not_found"},
		{http.MethodPost, "/known", http.StatusMethodNotAllowed, "method_not_allowed"},
	} {
		recorder := httptest.NewRecorder()
		app.Router().ServeHTTP(recorder, httptest.NewRequest(test.method, test.path, nil))
		if recorder.Code != test.status || !strings.Contains(recorder.Body.String(), `"code":"`+test.code+`"`) {
			t.Fatalf("%s %s: %d %s", test.method, test.path, recorder.Code, recorder.Body)
		}
	}
}

func TestReadinessAndShutdownHooks(t *testing.T) {
	app, err := New(Options{Readiness: map[string]Check{
		"database": func(context.Context) error { return errors.New("offline") },
	}})
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	app.Router().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/health/ready", nil))
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("readiness status = %d", recorder.Code)
	}
	var called bool
	app.OnShutdown(func(context.Context) error {
		called = true
		return nil
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := app.Run(ctx); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("shutdown hook was not called")
	}
}

func TestRateLimitRejectsExcessRequests(t *testing.T) {
	app, err := New(Options{HTTP: HTTPOptions{
		RateLimit: RateLimitOptions{Enabled: true, RequestsPerMinute: 1, Burst: 1},
	}})
	if err != nil {
		t.Fatal(err)
	}
	app.Router().GET("/limited", func(c *gin.Context) { httpx.OK(c, "ok") })
	first := httptest.NewRecorder()
	app.Router().ServeHTTP(first, httptest.NewRequest(http.MethodGet, "/limited", nil))
	second := httptest.NewRecorder()
	app.Router().ServeHTTP(second, httptest.NewRequest(http.MethodGet, "/limited", nil))
	if first.Code != http.StatusOK || second.Code != http.StatusTooManyRequests {
		t.Fatalf("unexpected statuses: %d, %d", first.Code, second.Code)
	}
}

func TestTrustedProxiesControlClientIP(t *testing.T) {
	for _, test := range []struct {
		name      string
		proxies   []string
		remote    string
		forwarded string
		want      string
	}{
		{"default distrusts forwarded headers", nil, "203.0.113.7:1234", "1.2.3.4", "203.0.113.7"},
		{"trusted proxy honored", []string{"10.0.0.0/8"}, "10.1.2.3:1234", "1.2.3.4", "1.2.3.4"},
		{"untrusted peer ignored", []string{"10.0.0.0/8"}, "203.0.113.7:1234", "1.2.3.4", "203.0.113.7"},
	} {
		t.Run(test.name, func(t *testing.T) {
			app, err := New(Options{HTTP: HTTPOptions{TrustedProxies: test.proxies}})
			if err != nil {
				t.Fatal(err)
			}
			app.Router().GET("/ip", func(c *gin.Context) {
				httpx.OK(c, gin.H{"ip": c.ClientIP()})
			})
			request := httptest.NewRequest(http.MethodGet, "/ip", nil)
			request.RemoteAddr = test.remote
			request.Header.Set("X-Forwarded-For", test.forwarded)
			recorder := httptest.NewRecorder()
			app.Router().ServeHTTP(recorder, request)
			if !strings.Contains(recorder.Body.String(), `"ip":"`+test.want+`"`) {
				t.Fatalf("client IP: %s", recorder.Body.String())
			}
		})
	}
}

func TestNewRejectsInvalidTrustedProxy(t *testing.T) {
	if _, err := New(Options{HTTP: HTTPOptions{TrustedProxies: []string{"not-a-proxy"}}}); err == nil {
		t.Fatal("expected error for invalid trusted proxy")
	}
}

func TestMetricsEndpointExposesRoutePatterns(t *testing.T) {
	app, err := New(Options{Metrics: MetricsOptions{Enabled: true}})
	if err != nil {
		t.Fatal(err)
	}
	app.Router().GET("/tasks/:id", func(c *gin.Context) { httpx.OK(c, "ok") })
	for _, path := range []string{"/tasks/123", "/tasks/456", "/absent"} {
		recorder := httptest.NewRecorder()
		app.Router().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
	}
	scrape := httptest.NewRecorder()
	app.Router().ServeHTTP(scrape, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if scrape.Code != http.StatusOK {
		t.Fatalf("scrape status = %d", scrape.Code)
	}
	body := scrape.Body.String()
	if !strings.Contains(body, `http_requests_total{method="GET",route="/tasks/:id",status="200"} 2`) {
		t.Fatalf("route pattern label missing:\n%s", body)
	}
	if !strings.Contains(body, `route="unmatched"`) || !strings.Contains(body, "http_request_duration_seconds") {
		t.Fatalf("unmatched label or duration histogram missing:\n%s", body)
	}
	if app.Metrics() == nil || app.Metrics().Registry() == nil {
		t.Fatal("metrics accessor not wired")
	}
}

func TestMetricsAndPProfDisabledByDefault(t *testing.T) {
	app, err := New(Options{})
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/metrics", "/debug/pprof/"} {
		recorder := httptest.NewRecorder()
		app.Router().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
		if recorder.Code != http.StatusNotFound || !strings.Contains(recorder.Body.String(), `"code":"not_found"`) {
			t.Fatalf("%s: %d %s", path, recorder.Code, recorder.Body)
		}
	}
	if app.Metrics() != nil {
		t.Fatal("metrics should be nil when disabled")
	}
}

func TestPProfMountsWhenEnabled(t *testing.T) {
	app, err := New(Options{PProf: PProfOptions{Enabled: true}})
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	app.Router().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/debug/pprof/", nil))
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), "profiles") {
		t.Fatalf("pprof index: %d", recorder.Code)
	}
}

func TestRequestScopedLoggerCarriesRequestID(t *testing.T) {
	var buffer strings.Builder
	app, err := New(Options{Logger: slog.New(slog.NewJSONHandler(&buffer, nil))})
	if err != nil {
		t.Fatal(err)
	}
	app.Router().GET("/log", func(c *gin.Context) {
		httpx.Logger(c).Info("handled")
		httpx.OK(c, "ok")
	})
	recorder := httptest.NewRecorder()
	app.Router().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/log", nil))
	requestID := recorder.Header().Get("X-Request-ID")
	if requestID == "" || !strings.Contains(buffer.String(), `"request_id":"`+requestID+`"`) {
		t.Fatalf("request-scoped logger missing request ID %q:\n%s", requestID, buffer.String())
	}
	if !strings.Contains(buffer.String(), `"path":"/log"`) {
		t.Fatalf("request-scoped logger missing path:\n%s", buffer.String())
	}
}

func TestRunnerErrorShutsDownApplication(t *testing.T) {
	app, err := New(Options{HTTP: HTTPOptions{Address: "127.0.0.1:0", ShutdownTimeout: time.Second}})
	if err != nil {
		t.Fatal(err)
	}
	var hookCalled bool
	app.OnShutdown(func(context.Context) error {
		hookCalled = true
		return nil
	})
	sentinel := errors.New("worker exploded")
	app.Go("worker", func(context.Context) error { return sentinel })
	runErr := app.Run(context.Background())
	if !errors.Is(runErr, sentinel) || !strings.Contains(runErr.Error(), `runner "worker"`) {
		t.Fatalf("runner error not propagated: %v", runErr)
	}
	if !hookCalled {
		t.Fatal("shutdown hook did not run after runner failure")
	}
}

func TestRunnerReceivesCancelOnShutdown(t *testing.T) {
	app, err := New(Options{HTTP: HTTPOptions{Address: "127.0.0.1:0", ShutdownTimeout: time.Second}})
	if err != nil {
		t.Fatal(err)
	}
	canceled := make(chan struct{})
	app.Go("watcher", func(ctx context.Context) error {
		<-ctx.Done()
		close(canceled)
		return ctx.Err()
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := app.Run(ctx); err != nil {
		t.Fatalf("canceled runner should not produce an error: %v", err)
	}
	select {
	case <-canceled:
	default:
		t.Fatal("runner did not observe cancellation")
	}
}

func TestRunnerPanicBecomesError(t *testing.T) {
	app, err := New(Options{HTTP: HTTPOptions{Address: "127.0.0.1:0", ShutdownTimeout: time.Second}})
	if err != nil {
		t.Fatal(err)
	}
	app.Go("panicky", func(context.Context) error { panic("boom") })
	runErr := app.Run(context.Background())
	if runErr == nil || !strings.Contains(runErr.Error(), "panic") {
		t.Fatalf("runner panic not converted to error: %v", runErr)
	}
}

func TestShutdownHooksRemainLIFOWithRunners(t *testing.T) {
	app, err := New(Options{HTTP: HTTPOptions{Address: "127.0.0.1:0", ShutdownTimeout: time.Second}})
	if err != nil {
		t.Fatal(err)
	}
	var order []string
	app.OnShutdown(func(context.Context) error {
		order = append(order, "first")
		return nil
	})
	app.OnShutdown(func(context.Context) error {
		order = append(order, "second")
		return nil
	})
	app.Go("noop", func(ctx context.Context) error {
		<-ctx.Done()
		return nil
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := app.Run(ctx); err != nil {
		t.Fatal(err)
	}
	if len(order) != 2 || order[0] != "second" || order[1] != "first" {
		t.Fatalf("hooks not LIFO: %v", order)
	}
}

func TestGoAfterRunIsDropped(t *testing.T) {
	app, err := New(Options{HTTP: HTTPOptions{Address: "127.0.0.1:0", ShutdownTimeout: time.Second}})
	if err != nil {
		t.Fatal(err)
	}
	running := make(chan struct{})
	app.Go("blocker", func(ctx context.Context) error {
		close(running)
		<-ctx.Done()
		return nil
	})
	ctx, cancel := context.WithCancel(context.Background())
	finished := make(chan error, 1)
	go func() { finished <- app.Run(ctx) }()
	<-running
	var lateRan bool
	app.Go("late", func(context.Context) error {
		lateRan = true
		return nil
	})
	cancel()
	if err := <-finished; err != nil {
		t.Fatal(err)
	}
	if lateRan {
		t.Fatal("runner registered after Run was executed")
	}
}

func TestCacheDefaultsToMemory(t *testing.T) {
	app, err := New(Options{HTTP: HTTPOptions{Address: "127.0.0.1:0", ShutdownTimeout: time.Second}})
	if err != nil {
		t.Fatal(err)
	}
	store := app.Cache()
	if store == nil {
		t.Fatal("cache accessor returned nil")
	}
	ctx := context.Background()
	if err := store.Set(ctx, "greeting", "hello", 0); err != nil {
		t.Fatal(err)
	}
	if value, ok, _ := store.Get(ctx, "greeting"); !ok || value != "hello" {
		t.Fatalf("memory cache round trip failed: %q %v", value, ok)
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if err := app.Run(canceled); err != nil {
		t.Fatalf("shutdown with cache failed: %v", err)
	}
}

func TestRedisCacheRegistersReadiness(t *testing.T) {
	server := miniredis.RunT(t)
	app, err := New(Options{Cache: CacheOptions{Driver: "redis", RedisURL: "redis://" + server.Addr()}})
	if err != nil {
		t.Fatal(err)
	}
	ready := httptest.NewRecorder()
	app.Router().ServeHTTP(ready, httptest.NewRequest(http.MethodGet, "/health/ready", nil))
	if ready.Code != http.StatusOK {
		t.Fatalf("readiness with live redis: %d %s", ready.Code, ready.Body)
	}
	server.Close()
	notReady := httptest.NewRecorder()
	app.Router().ServeHTTP(notReady, httptest.NewRequest(http.MethodGet, "/health/ready", nil))
	if notReady.Code != http.StatusServiceUnavailable || !strings.Contains(notReady.Body.String(), "redis") {
		t.Fatalf("readiness with dead redis: %d %s", notReady.Code, notReady.Body)
	}
}

func TestNewRejectsInvalidCacheConfiguration(t *testing.T) {
	for _, test := range []struct {
		name    string
		options CacheOptions
	}{
		{"unknown driver", CacheOptions{Driver: "memcached"}},
		{"redis without url", CacheOptions{Driver: "redis"}},
		{"redis with bad url", CacheOptions{Driver: "redis", RedisURL: "http://wrong"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := New(Options{Cache: test.options}); err == nil {
				t.Fatal("expected constructor error")
			}
		})
	}
}

func TestQueueDefaultsToSyncDriver(t *testing.T) {
	app, err := New(Options{})
	if err != nil {
		t.Fatal(err)
	}
	q := app.Queue()
	if q == nil {
		t.Fatal("queue accessor returned nil")
	}
	var handled bool
	queue.Register(q, "inline", func(context.Context, struct{}) error {
		handled = true
		return nil
	})
	if err := queue.Dispatch(context.Background(), q, "inline", struct{}{}); err != nil {
		t.Fatal(err)
	}
	if !handled {
		t.Fatal("sync queue did not execute inline")
	}
}

func TestNewRejectsInvalidQueueConfiguration(t *testing.T) {
	for _, test := range []struct {
		name    string
		options QueueOptions
	}{
		{"unknown driver", QueueOptions{Driver: "kafka"}},
		{"redis without url", QueueOptions{Driver: "redis"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := New(Options{Queue: test.options}); err == nil {
				t.Fatal("expected constructor error")
			}
		})
	}
}

func TestQueueRedisDriverRegistersReadiness(t *testing.T) {
	server := miniredis.RunT(t)
	app, err := New(Options{Queue: QueueOptions{Driver: "redis", RedisURL: "redis://" + server.Addr()}})
	if err != nil {
		t.Fatal(err)
	}
	ready := httptest.NewRecorder()
	app.Router().ServeHTTP(ready, httptest.NewRequest(http.MethodGet, "/health/ready", nil))
	if ready.Code != http.StatusOK {
		t.Fatalf("readiness with live redis: %d %s", ready.Code, ready.Body)
	}
}

func TestCloseRunsHooksExactlyOnce(t *testing.T) {
	app, err := New(Options{})
	if err != nil {
		t.Fatal(err)
	}
	var calls int
	app.OnShutdown(func(context.Context) error {
		calls++
		return nil
	})
	if err := app.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := app.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("hooks ran %d times", calls)
	}
}

func TestCloseAfterRunIsNoop(t *testing.T) {
	app, err := New(Options{HTTP: HTTPOptions{Address: "127.0.0.1:0", ShutdownTimeout: time.Second}})
	if err != nil {
		t.Fatal(err)
	}
	var calls int
	app.OnShutdown(func(context.Context) error {
		calls++
		return nil
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := app.Run(ctx); err != nil {
		t.Fatal(err)
	}
	if err := app.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("hooks ran %d times after Run+Close", calls)
	}
}

func TestRunReturnsServerError(t *testing.T) {
	app, err := New(Options{HTTP: HTTPOptions{Address: "bad address", ShutdownTimeout: time.Millisecond}})
	if err != nil {
		t.Fatal(err)
	}
	if err := app.Run(context.Background()); err == nil {
		t.Fatal("expected listen error")
	}
}
