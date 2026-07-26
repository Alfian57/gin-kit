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
	"sync"
	"time"

	frameworkdb "github.com/Alfian57/gin-kit/framework/database"
	"github.com/Alfian57/gin-kit/framework/httpx"
	"github.com/Alfian57/gin-kit/framework/validation"
	"github.com/gin-gonic/gin"
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

type Options struct {
	Environment string
	Logger      *slog.Logger
	HTTP        HTTPOptions
	Validator   *validation.Validator
	ErrorMapper httpx.Mapper
	Readiness   map[string]Check
	Database    *frameworkdb.Config
}

type Application struct {
	router          *gin.Engine
	server          *http.Server
	logger          *slog.Logger
	mapper          httpx.Mapper
	validator       *validation.Validator
	readiness       map[string]Check
	database        *frameworkdb.Connection
	shutdownTimeout time.Duration
	hooksMu         sync.Mutex
	shutdownHooks   []func(context.Context) error
}

func New(options Options) (*Application, error) {
	applyDefaults(&options)
	if options.HTTP.MaxBodyBytes < 1 {
		return nil, errors.New("framework: HTTP max body bytes must be positive")
	}
	if options.HTTP.RateLimit.Enabled && options.HTTP.RateLimit.RequestsPerMinute < 1 {
		return nil, errors.New("framework: rate limit requests per minute must be positive")
	}

	if options.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}
	router := gin.New()
	router.HandleMethodNotAllowed = true
	if err := router.SetTrustedProxies(options.HTTP.TrustedProxies); err != nil {
		return nil, fmt.Errorf("framework: invalid trusted proxies: %w", err)
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
	app.server = &http.Server{
		Addr:         options.HTTP.Address,
		Handler:      router,
		ReadTimeout:  options.HTTP.ReadTimeout,
		WriteTimeout: options.HTTP.WriteTimeout,
		IdleTimeout:  options.HTTP.IdleTimeout,
	}

	router.Use(
		requestID(),
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
	app.registerHealthRoutes()
	router.NoRoute(func(c *gin.Context) {
		httpx.Fail(c, httpx.NewError(http.StatusNotFound, "not_found", "The requested resource was not found."))
	})
	router.NoMethod(func(c *gin.Context) {
		httpx.Fail(c, httpx.NewError(http.StatusMethodNotAllowed, "method_not_allowed", "The requested method is not allowed."))
	})
	return app, nil
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

func (a *Application) OnShutdown(hook func(context.Context) error) {
	if hook == nil {
		return
	}
	a.hooksMu.Lock()
	a.shutdownHooks = append(a.shutdownHooks, hook)
	a.hooksMu.Unlock()
}

// Run serves until the context is canceled or the server fails, then performs
// graceful shutdown and invokes hooks in reverse registration order.
func (a *Application) Run(ctx context.Context) error {
	serverErrors := make(chan error, 1)
	go func() {
		err := a.server.ListenAndServe()
		if errors.Is(err, http.ErrServerClosed) {
			err = nil
		}
		serverErrors <- err
	}()

	var serveErr error
	select {
	case <-ctx.Done():
	case serveErr = <-serverErrors:
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), a.shutdownTimeout)
	defer cancel()
	shutdownErr := a.server.Shutdown(shutdownCtx)
	hooksErr := a.runShutdownHooks(shutdownCtx)
	return errors.Join(serveErr, shutdownErr, hooksErr)
}

func (a *Application) runShutdownHooks(ctx context.Context) error {
	a.hooksMu.Lock()
	hooks := append([]func(context.Context) error(nil), a.shutdownHooks...)
	a.hooksMu.Unlock()
	var result error
	for index := len(hooks) - 1; index >= 0; index-- {
		result = errors.Join(result, hooks[index](ctx))
	}
	return result
}

func (a *Application) registerHealthRoutes() {
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
