package server

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var metricsRegistry = prometheus.NewRegistry()

var (
	httpRequestsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "aidocs_http_requests_total",
		Help: "Total HTTP requests handled by the aidocs server.",
	}, []string{"method", "route", "status"})
	httpRequestDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "aidocs_http_request_duration_seconds",
		Help:    "HTTP request latency by route.",
		Buckets: prometheus.DefBuckets,
	}, []string{"method", "route", "status"})
	httpRequestSizeBytes = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "aidocs_http_request_size_bytes",
		Help:    "HTTP request body size by route.",
		Buckets: []float64{0, 512, 1024, 10 * 1024, 100 * 1024, 1024 * 1024, 5 * 1024 * 1024, 10 * 1024 * 1024},
	}, []string{"method", "route"})
	httpResponseSizeBytes = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "aidocs_http_response_size_bytes",
		Help:    "HTTP response body size by route.",
		Buckets: []float64{0, 512, 1024, 10 * 1024, 100 * 1024, 1024 * 1024, 5 * 1024 * 1024, 10 * 1024 * 1024},
	}, []string{"method", "route", "status"})
	authAttemptsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "aidocs_auth_attempts_total",
		Help: "Authentication attempts by kind and outcome.",
	}, []string{"kind", "outcome"})
	documentEventsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "aidocs_document_events_total",
		Help: "Document lifecycle and interaction events.",
	}, []string{"event", "visibility", "actor_type"})
	versionEventsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "aidocs_version_events_total",
		Help: "Document version lifecycle and interaction events.",
	}, []string{"event", "actor_type"})
	commentEventsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "aidocs_comment_events_total",
		Help: "Comment lifecycle events.",
	}, []string{"event", "status", "actor_type"})
	grantEventsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "aidocs_grant_events_total",
		Help: "Document grant lifecycle events.",
	}, []string{"event", "role", "principal_type", "actor_type"})
	serviceAccountEventsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "aidocs_service_account_events_total",
		Help: "Service account lifecycle and key events.",
	}, []string{"event", "actor_type"})
	renderEventsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "aidocs_render_events_total",
		Help: "Render token and document render events.",
	}, []string{"event", "outcome"})
	htmlBytes = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "aidocs_html_bytes",
		Help:    "Uploaded or downloaded HTML payload sizes.",
		Buckets: []float64{1024, 10 * 1024, 100 * 1024, 1024 * 1024, 5 * 1024 * 1024, 10 * 1024 * 1024},
	}, []string{"operation"})
)

func init() {
	metricsRegistry.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		httpRequestsTotal,
		httpRequestDuration,
		httpRequestSizeBytes,
		httpResponseSizeBytes,
		authAttemptsTotal,
		documentEventsTotal,
		versionEventsTotal,
		commentEventsTotal,
		grantEventsTotal,
		serviceAccountEventsTotal,
		renderEventsTotal,
		htmlBytes,
	)
}

func metricsHandler() gin.HandlerFunc {
	h := promhttp.HandlerFor(metricsRegistry, promhttp.HandlerOpts{})
	return func(c *gin.Context) { h.ServeHTTP(c.Writer, c.Request) }
}

func prometheusMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		if c.Request.ContentLength > 0 {
			httpRequestSizeBytes.WithLabelValues(c.Request.Method, routeLabel(c)).Observe(float64(c.Request.ContentLength))
		}
		c.Next()
		status := strconv.Itoa(c.Writer.Status())
		route := routeLabel(c)
		httpRequestsTotal.WithLabelValues(c.Request.Method, route, status).Inc()
		httpRequestDuration.WithLabelValues(c.Request.Method, route, status).Observe(time.Since(start).Seconds())
		if size := c.Writer.Size(); size >= 0 {
			httpResponseSizeBytes.WithLabelValues(c.Request.Method, route, status).Observe(float64(size))
		}
	}
}

func routeLabel(c *gin.Context) string {
	if p := c.FullPath(); p != "" {
		return p
	}
	if c.Request.URL.Path == "/metrics" {
		return "/metrics"
	}
	return "unmatched"
}

func actorType(c *gin.Context) string {
	p := current(c)
	if p == nil || p.Type == "" {
		return "anonymous"
	}
	return string(p.Type)
}

func incAuth(kind, outcome string) { authAttemptsTotal.WithLabelValues(kind, outcome).Inc() }
func incDocument(event, visibility, actor string) {
	documentEventsTotal.WithLabelValues(event, visibility, actor).Inc()
}
func incVersion(event, actor string) { versionEventsTotal.WithLabelValues(event, actor).Inc() }
func incComment(event, status, actor string) {
	commentEventsTotal.WithLabelValues(event, status, actor).Inc()
}
func incGrant(event, role, principalType, actor string) {
	grantEventsTotal.WithLabelValues(event, role, principalType, actor).Inc()
}
func incServiceAccount(event, actor string) {
	serviceAccountEventsTotal.WithLabelValues(event, actor).Inc()
}
func incRender(event, outcome string) { renderEventsTotal.WithLabelValues(event, outcome).Inc() }
func observeHTML(operation string, size int) {
	htmlBytes.WithLabelValues(operation).Observe(float64(size))
}

var _ http.Handler
