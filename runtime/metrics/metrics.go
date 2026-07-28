// Package metrics provides opt-in Prometheus instrumentation for gin-kit
// applications.
package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Options struct {
	// Registry receives the HTTP collectors. When nil, a new registry with the
	// standard Go and process collectors is created.
	Registry *prometheus.Registry
	// Namespace prefixes every metric name.
	Namespace string
	// Buckets are the request-duration histogram buckets in seconds.
	Buckets []float64
}

type Metrics struct {
	registry *prometheus.Registry
	requests *prometheus.CounterVec
	duration *prometheus.HistogramVec
	inFlight prometheus.Gauge
}

func New(options Options) (*Metrics, error) {
	registry := options.Registry
	if registry == nil {
		registry = prometheus.NewRegistry()
		if err := registry.Register(collectors.NewGoCollector()); err != nil {
			return nil, err
		}
		if err := registry.Register(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{})); err != nil {
			return nil, err
		}
	}
	buckets := options.Buckets
	if len(buckets) == 0 {
		buckets = prometheus.DefBuckets
	}
	m := &Metrics{
		registry: registry,
		requests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: options.Namespace,
			Name:      "http_requests_total",
			Help:      "Requests processed, labeled by method, route pattern, and status.",
		}, []string{"method", "route", "status"}),
		duration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: options.Namespace,
			Name:      "http_request_duration_seconds",
			Help:      "Request duration in seconds, labeled by method and route pattern.",
			Buckets:   buckets,
		}, []string{"method", "route"}),
		inFlight: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: options.Namespace,
			Name:      "http_requests_in_flight",
			Help:      "Requests currently being served.",
		}),
	}
	for _, collector := range []prometheus.Collector{m.requests, m.duration, m.inFlight} {
		if err := registry.Register(collector); err != nil {
			return nil, err
		}
	}
	return m, nil
}

// Middleware records request count, duration, and an in-flight gauge. The
// route label uses the matched route pattern so path parameters do not explode
// label cardinality; unmatched requests are grouped under "unmatched".
func (m *Metrics) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		started := time.Now()
		m.inFlight.Inc()
		defer m.inFlight.Dec()
		c.Next()
		route := c.FullPath()
		if route == "" {
			route = "unmatched"
		}
		method := c.Request.Method
		m.requests.WithLabelValues(method, route, strconv.Itoa(c.Writer.Status())).Inc()
		m.duration.WithLabelValues(method, route).Observe(time.Since(started).Seconds())
	}
}

// Handler serves the registry in Prometheus exposition format.
func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{})
}

// Registry exposes the underlying registry for custom application metrics.
func (m *Metrics) Registry() *prometheus.Registry { return m.registry }
