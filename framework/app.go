// Package framework provides gin-kit's explicit application lifecycle and
// production-safe HTTP defaults on top of Gin.
package framework

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/Alfian57/gin-kit/framework/cache"
	frameworkdb "github.com/Alfian57/gin-kit/framework/database"
	"github.com/Alfian57/gin-kit/framework/httpx"
	"github.com/Alfian57/gin-kit/framework/metrics"
	"github.com/Alfian57/gin-kit/framework/openapi"
	"github.com/Alfian57/gin-kit/framework/queue"
	"github.com/Alfian57/gin-kit/framework/validation"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/redis/go-redis/v9"
	"golang.org/x/sync/errgroup"
)

type Check func(context.Context) error

type HTTPOptions struct {
	Address         string
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	IdleTimeout     time.Duration
	ShutdownTimeout time.Duration
	MaxBodyBytes    int64
	UI              bool
	CORSOrigins     []string
	RateLimit       RateLimitOptions
	// TrustedProxies lists proxy IPs or CIDRs whose forwarded headers
	// (X-Forwarded-For) are honored when resolving client addresses. When
	// empty, no proxy is trusted and the socket peer address is used.
	TrustedProxies []string
}

type MetricsOptions struct {
	Enabled bool
	// Path is the scrape endpoint, defaulting to /metrics.
	Path string
	// Registry optionally receives the HTTP collectors instead of a new
	// registry with the standard Go and process collectors.
	Registry *prometheus.Registry
}

type DocsOptions struct {
	// Enabled serves the OpenAPI spec and Swagger UI.
	Enabled bool
	// Path is the Swagger UI page, defaulting to /docs.
	Path string
	// SpecPath is the JSON document, defaulting to /openapi.json.
	SpecPath string
	// Title defaults to "API".
	Title string
	// Version defaults to "0.1.0".
	Version     string
	Description string
	// Servers lists the base URLs shown in the spec.
	Servers []string
	// BasicAuthUsername and BasicAuthPassword, when both set, protect the
	// docs and spec endpoints with HTTP basic auth.
	BasicAuthUsername string
	BasicAuthPassword string
}

type QueueOptions struct {
	// Driver selects the job backend: "sync" (default, inline execution) or
	// "redis" (asynq worker supervised by Run).
	Driver string
	// RedisURL configures the redis driver, e.g. redis://localhost:6379/0.
	RedisURL string
	// Concurrency is the redis worker goroutine count, defaulting to 10.
	Concurrency int
}

type CacheOptions struct {
	// Driver selects the cache store: "memory" (default) or "redis".
	Driver string
	// Prefix is prepended to every cache key on shared stores.
	Prefix string
	// RedisURL configures the redis driver, e.g. redis://localhost:6379/0.
	RedisURL string
}

type PProfOptions struct {
	Enabled bool
	// Prefix is the mount point, defaulting to /debug/pprof. The endpoints
	// expose process internals and must never be reachable publicly.
	Prefix string
}

type Options struct {
	Environment string
	Logger      *slog.Logger
	HTTP        HTTPOptions
	Validator   *validation.Validator
	ErrorMapper httpx.Mapper
	Readiness   map[string]Check
	Database    *frameworkdb.Config
	Metrics     MetricsOptions
	PProf       PProfOptions
	Cache       CacheOptions
	Queue       QueueOptions
	Docs        DocsOptions
}

type Application struct {
	router          *gin.Engine
	server          *http.Server
	logger          *slog.Logger
	mapper          httpx.Mapper
	validator       *validation.Validator
	readiness       map[string]Check
	database        *frameworkdb.Connection
	metrics         *metrics.Metrics
	cache           cache.Store
	queue           *queue.Queue
	docs            *openapi.Registry
	shutdownTimeout time.Duration
	hooksMu         sync.Mutex
	shutdownHooks   []func(context.Context) error
	hooksRan        bool
	runnersMu       sync.Mutex
	runners         []runner
	started         bool
}

// runner is a named long-running goroutine supervised by Run.
type runner struct {
	name string
	run  func(context.Context) error
}

func New(options Options) (*Application, error) {
	applyDefaults(&options)
	if options.HTTP.MaxBodyBytes < 1 {
		return nil, errors.New("framework: HTTP max body bytes must be positive")
	}
	if options.HTTP.RateLimit.Enabled && options.HTTP.RateLimit.RequestsPerMinute < 1 {
		return nil, errors.New("framework: rate limit requests per minute must be positive")
	}
	if options.Docs.Enabled {
		if (options.Docs.BasicAuthUsername == "") != (options.Docs.BasicAuthPassword == "") {
			return nil, errors.New("framework: docs basic auth requires both a username and a password")
		}
		if options.Docs.Path == options.Docs.SpecPath {
			return nil, errors.New("framework: docs path and spec path must differ")
		}
		if !strings.HasPrefix(options.Docs.Path, "/") || !strings.HasPrefix(options.Docs.SpecPath, "/") {
			return nil, errors.New("framework: docs paths must start with /")
		}
	}

	if options.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}
	router := gin.New()
	router.HandleMethodNotAllowed = true
	if err := router.SetTrustedProxies(options.HTTP.TrustedProxies); err != nil {
		return nil, fmt.Errorf("framework: invalid trusted proxies: %w", err)
	}
	var httpMetrics *metrics.Metrics
	if options.Metrics.Enabled {
		var err error
		httpMetrics, err = metrics.New(metrics.Options{Registry: options.Metrics.Registry})
		if err != nil {
			return nil, fmt.Errorf("framework: metrics: %w", err)
		}
	}
	redisOptions, err := resolveRedisOptions(&options)
	if err != nil {
		return nil, err
	}
	jobs, err := queue.New(queue.Options{
		Driver:      options.Queue.Driver,
		RedisURL:    options.Queue.RedisURL,
		Concurrency: options.Queue.Concurrency,
		Logger:      options.Logger,
	})
	if err != nil {
		return nil, fmt.Errorf("framework: %w", err)
	}
	var connection *frameworkdb.Connection
	if options.Database != nil {
		var err error
		connection, err = frameworkdb.Open(context.Background(), *options.Database)
		if err != nil {
			return nil, err
		}
	}
	app := &Application{
		router:          router,
		logger:          options.Logger,
		mapper:          options.ErrorMapper,
		validator:       options.Validator,
		readiness:       options.Readiness,
		database:        connection,
		metrics:         httpMetrics,
		docs:            openapi.NewRegistry(),
		shutdownTimeout: options.HTTP.ShutdownTimeout,
	}
	if connection != nil {
		if app.readiness == nil {
			app.readiness = make(map[string]Check)
		}
		app.readiness["database"] = func(ctx context.Context) error {
			return connection.SQL.PingContext(ctx)
		}
	}
	if connection != nil {
		app.OnShutdown(func(context.Context) error { return connection.Close() })
	}
	if err := app.installCache(options, redisOptions); err != nil {
		hookErr := app.runShutdownHooks(context.Background())
		return nil, errors.Join(err, hookErr, jobs.Close())
	}
	if err := app.installQueue(options, jobs); err != nil {
		hookErr := app.runShutdownHooks(context.Background())
		return nil, errors.Join(err, hookErr, jobs.Close())
	}
	app.server = &http.Server{
		Addr:         options.HTTP.Address,
		Handler:      router,
		ReadTimeout:  options.HTTP.ReadTimeout,
		WriteTimeout: options.HTTP.WriteTimeout,
		IdleTimeout:  options.HTTP.IdleTimeout,
	}

	router.Use(requestID(), contextLogger(options.Logger), validatorContext(options.Validator))
	if httpMetrics != nil {
		router.Use(httpMetrics.Middleware())
	}
	router.Use(
		accessLog(options.Logger),
		recovery(options.Logger, options.ErrorMapper),
		errorHandler(options.Logger, options.ErrorMapper),
		securityHeaders(options.HTTP.UI),
		bodyLimit(options.HTTP.MaxBodyBytes),
		cors(options.HTTP.CORSOrigins),
	)
	if options.HTTP.RateLimit.Enabled {
		router.Use(NewRateLimiter(options.HTTP.RateLimit).Middleware())
	}
	if httpMetrics != nil {
		router.GET(options.Metrics.Path, gin.WrapH(httpMetrics.Handler()))
	}
	if options.PProf.Enabled {
		mountPProf(router, options.PProf.Prefix)
	}
	if options.Docs.Enabled {
		mountDocs(router, app.docs, options)
	}
	app.registerHealthRoutes()
	router.NoRoute(func(c *gin.Context) {
		httpx.Fail(c, httpx.NewError(http.StatusNotFound, "not_found", "The requested resource was not found."))
	})
	router.NoMethod(func(c *gin.Context) {
		httpx.Fail(c, httpx.NewError(http.StatusMethodNotAllowed, "method_not_allowed", "The requested method is not allowed."))
	})
	return app, nil
}

// resolveRedisOptions validates the Redis configuration up front, before any
// connection is opened, so a bad URL fails fast without leaking resources.
func resolveRedisOptions(options *Options) (*redis.Options, error) {
	switch options.Cache.Driver {
	case "", "memory":
		return nil, nil
	case "redis":
	default:
		return nil, fmt.Errorf("framework: unknown cache driver %q", options.Cache.Driver)
	}
	if options.Cache.RedisURL == "" {
		return nil, errors.New("framework: the redis cache driver requires a Redis URL")
	}
	parsed, err := redis.ParseURL(options.Cache.RedisURL)
	if err != nil {
		return nil, fmt.Errorf("framework: invalid Redis URL: %w", err)
	}
	return parsed, nil
}

// installCache builds the cache store and, when Redis is used, the shared
// Redis client with its readiness check. Hook order matters: the client's
// close hook registers before the store's, so LIFO shutdown closes consumers
// before their connection.
func (a *Application) installCache(options Options, redisOptions *redis.Options) error {
	if redisOptions == nil {
		a.cache = cache.NewMemory(cache.MemoryOptions{})
		a.OnShutdown(func(context.Context) error { return a.cache.Close() })
		return nil
	}
	client := redis.NewClient(redisOptions)
	if a.readiness == nil {
		a.readiness = make(map[string]Check)
	}
	a.readiness["redis"] = func(ctx context.Context) error {
		return client.Ping(ctx).Err()
	}
	a.OnShutdown(func(context.Context) error { return client.Close() })
	store, err := cache.NewRedis(cache.RedisOptions{Client: client, Prefix: options.Cache.Prefix})
	if err != nil {
		return err
	}
	a.cache = store
	a.OnShutdown(func(context.Context) error { return a.cache.Close() })
	return nil
}

// installQueue stores the queue, and for the redis driver registers the
// worker runner, a redis readiness check, and the drain hook. The close hook
// registers after the cache's so LIFO shutdown drains jobs first.
func (a *Application) installQueue(options Options, jobs *queue.Queue) error {
	a.queue = jobs
	a.OnShutdown(func(context.Context) error { return jobs.Close() })
	if options.Queue.Driver != "redis" {
		return nil
	}
	a.Go("queue", jobs.Start)
	if a.readiness == nil {
		a.readiness = make(map[string]Check)
	}
	if _, exists := a.readiness["redis"]; !exists {
		parsed, err := redis.ParseURL(options.Queue.RedisURL)
		if err != nil {
			return fmt.Errorf("framework: invalid Redis URL: %w", err)
		}
		pingClient := redis.NewClient(parsed)
		a.readiness["redis"] = func(ctx context.Context) error {
			return pingClient.Ping(ctx).Err()
		}
		a.OnShutdown(func(context.Context) error { return pingClient.Close() })
	}
	return nil
}

func applyDefaults(options *Options) {
	if options.Environment == "" {
		options.Environment = "development"
	}
	if options.Logger == nil {
		options.Logger = slog.New(slog.NewJSONHandler(os.Stderr, nil))
	}
	if options.Validator == nil {
		options.Validator = validation.New()
	}
	if options.ErrorMapper == nil {
		options.ErrorMapper = httpx.DefaultMapper
	}
	if options.HTTP.Address == "" {
		options.HTTP.Address = ":8080"
	}
	if options.HTTP.ReadTimeout == 0 {
		options.HTTP.ReadTimeout = 10 * time.Second
	}
	if options.HTTP.WriteTimeout == 0 {
		options.HTTP.WriteTimeout = 30 * time.Second
	}
	if options.HTTP.IdleTimeout == 0 {
		options.HTTP.IdleTimeout = 60 * time.Second
	}
	if options.HTTP.ShutdownTimeout == 0 {
		options.HTTP.ShutdownTimeout = 10 * time.Second
	}
	if options.HTTP.MaxBodyBytes == 0 {
		options.HTTP.MaxBodyBytes = 1 << 20
	}
	if options.Metrics.Path == "" {
		options.Metrics.Path = "/metrics"
	}
	if options.PProf.Prefix == "" {
		options.PProf.Prefix = "/debug/pprof"
	}
	if options.Docs.Path == "" {
		options.Docs.Path = "/docs"
	}
	if options.Docs.SpecPath == "" {
		options.Docs.SpecPath = "/openapi.json"
	}
	if options.Docs.Title == "" {
		options.Docs.Title = "API"
	}
	if options.Docs.Version == "" {
		options.Docs.Version = "0.1.0"
	}
}

func (a *Application) Router() *gin.Engine { return a.router }

// Use installs application middleware after gin-kit's safety middleware and
// before routes registered subsequently.
func (a *Application) Use(middleware ...gin.HandlerFunc) {
	a.router.Use(middleware...)
}

func (a *Application) Validator() *validation.Validator { return a.validator }

// Database returns the selected SQL/GORM/sqlx connection, when configured.
func (a *Application) Database() *frameworkdb.Connection { return a.database }

// Metrics returns the Prometheus instrumentation, or nil when disabled. Use
// its Registry to register custom application metrics.
func (a *Application) Metrics() *metrics.Metrics { return a.metrics }

// Cache returns the application cache store. It is never nil: without
// configuration an in-memory store is used, and CacheOptions selects Redis.
func (a *Application) Cache() cache.Store { return a.cache }

// Queue returns the application job queue. It is never nil: without
// configuration the sync driver executes jobs inline, and QueueOptions
// selects the supervised Redis worker.
func (a *Application) Queue() *queue.Queue { return a.queue }

// OpenAPI returns the documentation registry. It is never nil, so generated
// code can describe operations unconditionally; the docs endpoints serve
// only when DocsOptions.Enabled is set.
func (a *Application) OpenAPI() *openapi.Registry { return a.docs }

// Logger returns the application's base structured logger. Handlers should
// prefer the request-scoped httpx.Logger(c).
func (a *Application) Logger() *slog.Logger { return a.logger }

// Go registers a named background runner that Run supervises alongside the
// HTTP server: all runners share cancellation, and a runner error triggers a
// graceful application shutdown. Runners must return promptly once their
// context is canceled. Register runners before calling Run; later
// registrations are dropped with a warning.
func (a *Application) Go(name string, run func(context.Context) error) {
	if name == "" || run == nil {
		return
	}
	a.runnersMu.Lock()
	defer a.runnersMu.Unlock()
	if a.started {
		a.logger.Warn("runner registered after Run; ignored", "runner", name)
		return
	}
	a.runners = append(a.runners, runner{name: name, run: run})
}

func (a *Application) OnShutdown(hook func(context.Context) error) {
	if hook == nil {
		return
	}
	a.hooksMu.Lock()
	a.shutdownHooks = append(a.shutdownHooks, hook)
	a.hooksMu.Unlock()
}

// Run serves until the context is canceled, the server fails, or a runner
// fails, then performs graceful shutdown: the HTTP server stops, runners are
// waited on, and hooks execute in reverse registration order.
func (a *Application) Run(ctx context.Context) error {
	a.runnersMu.Lock()
	a.started = true
	runners := append([]runner(nil), a.runners...)
	a.runnersMu.Unlock()

	group, groupCtx := errgroup.WithContext(ctx)
	group.Go(func() error {
		err := a.server.ListenAndServe()
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	})
	for _, item := range runners {
		group.Go(func() error {
			err := runRunner(groupCtx, item.run)
			if err != nil && !errors.Is(err, context.Canceled) {
				return fmt.Errorf("framework: runner %q: %w", item.name, err)
			}
			return nil
		})
	}

	<-groupCtx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), a.shutdownTimeout)
	defer cancel()
	shutdownErr := a.server.Shutdown(shutdownCtx)
	waitErr := waitForGroup(shutdownCtx, group)
	hooksErr := a.runShutdownHooks(shutdownCtx)
	return errors.Join(waitErr, shutdownErr, hooksErr)
}

// runRunner converts runner panics into errors so one failing runner triggers
// a graceful shutdown instead of crashing the process.
func runRunner(ctx context.Context, run func(context.Context) error) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("panic: %v", recovered)
		}
	}()
	return run(ctx)
}

func waitForGroup(ctx context.Context, group *errgroup.Group) error {
	done := make(chan error, 1)
	go func() { done <- group.Wait() }()
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return errors.New("framework: runners did not exit before the shutdown timeout")
	}
}

// Close releases application resources without serving: shutdown hooks run
// in reverse registration order, exactly once across Close and Run. Intended
// for tests and short-lived binaries that never call Run.
func (a *Application) Close(ctx context.Context) error {
	return a.runShutdownHooks(ctx)
}

func (a *Application) runShutdownHooks(ctx context.Context) error {
	a.hooksMu.Lock()
	if a.hooksRan {
		a.hooksMu.Unlock()
		return nil
	}
	a.hooksRan = true
	hooks := append([]func(context.Context) error(nil), a.shutdownHooks...)
	a.hooksMu.Unlock()
	var result error
	for index := len(hooks) - 1; index >= 0; index-- {
		result = errors.Join(result, hooks[index](ctx))
	}
	return result
}

func (a *Application) registerHealthRoutes() {
	a.docs.Describe(
		openapi.Operation{
			Method: http.MethodGet, Path: "/health/live",
			Summary: "Liveness probe", Tags: []string{"health"},
			Response: struct {
				Status string `json:"status"`
			}{},
		},
		openapi.Operation{
			Method: http.MethodGet, Path: "/health/ready",
			Summary: "Readiness probe", Tags: []string{"health"},
			Response: struct {
				Status string `json:"status"`
			}{},
			ErrorCodes: []string{"not_ready"},
		},
	)
	a.router.GET("/health/live", func(c *gin.Context) {
		httpx.OK(c, gin.H{"status": "up"})
	})
	a.router.GET("/health/ready", func(c *gin.Context) {
		failed := make([]string, 0)
		for name, check := range a.readiness {
			ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
			err := check(ctx)
			cancel()
			if err != nil {
				failed = append(failed, name)
				a.logger.WarnContext(c.Request.Context(), "readiness check failed", "check", name, "error", err)
			}
		}
		if len(failed) > 0 {
			httpx.Fail(c, &httpx.Error{
				Status:  http.StatusServiceUnavailable,
				Code:    "not_ready",
				Message: "The application is not ready.",
				Details: gin.H{"failed_checks": failed},
			})
			return
		}
		httpx.OK(c, gin.H{"status": "ready"})
	})
}

func (a *Application) String() string {
	return fmt.Sprintf("gin-kit application listening on %s", a.server.Addr)
}
