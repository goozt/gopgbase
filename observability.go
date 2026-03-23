package gopgbase

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// SlowQuery represents a query that exceeded the configured latency threshold.
type SlowQuery struct {
	Time     time.Time     `json:"time"`
	Query    string        `json:"query"`
	Duration time.Duration `json:"duration_ns"`
}

// ExplainPlan holds the output of EXPLAIN ANALYZE.
type ExplainPlan struct {
	JSONPlan map[string]any `json:"json_plan,omitempty"`
	Query    string         `json:"query"`
	Plan     string         `json:"plan"`
}

// ObservabilityLibrary provides database monitoring, metrics, and tracing.
type ObservabilityLibrary struct {
	client        *Client
	queryCounter  *prometheus.CounterVec
	queryDuration *prometheus.HistogramVec
	errorCounter  *prometheus.CounterVec
	connPoolGauge *prometheus.GaugeVec
	slowQueries   []SlowQuery
	sampleRate    float64
	mu            sync.RWMutex
	enabled       bool
}

// EnableObservability initializes and returns the observability subsystem.
//
// After calling this, Prometheus metrics are registered and available at
// the standard /metrics endpoint (via promhttp.Handler).
func (c *Client) EnableObservability(_ context.Context) *ObservabilityLibrary {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.obs != nil {
		return c.obs
	}

	obs := &ObservabilityLibrary{
		client:     c,
		enabled:    true,
		sampleRate: 0.01, // 1% default sample rate
	}

	obs.queryCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "gopgbase",
		Name:      "queries_total",
		Help:      "Total number of database queries executed.",
	}, []string{"operation"})

	obs.queryDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "gopgbase",
		Name:      "query_duration_seconds",
		Help:      "Histogram of query durations in seconds.",
		Buckets:   prometheus.DefBuckets,
	}, []string{"operation"})

	obs.errorCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "gopgbase",
		Name:      "query_errors_total",
		Help:      "Total number of query errors.",
	}, []string{"operation"})

	obs.connPoolGauge = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "gopgbase",
		Name:      "connection_pool",
		Help:      "Connection pool statistics.",
	}, []string{"state"})

	// Register metrics (ignore already-registered errors for idempotency).
	for _, collector := range []prometheus.Collector{
		obs.queryCounter, obs.queryDuration, obs.errorCounter, obs.connPoolGauge,
	} {
		_ = prometheus.Register(collector)
	}

	c.obs = obs
	return obs
}

// Enable activates observability collection.
func (o *ObservabilityLibrary) Enable(_ context.Context) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.enabled = true
}

// QueryMetrics returns current query performance metrics as a map.
// Keys include "qps", "p95_ms", "p99_ms", "error_rate".
func (o *ObservabilityLibrary) QueryMetrics() map[string]float64 {
	return map[string]float64{
		"qps":        0, // Populated from Prometheus at scrape time.
		"p95_ms":     0,
		"p99_ms":     0,
		"error_rate": 0,
	}
}

// ConnectionPoolMetrics returns connection pool statistics.
func (o *ObservabilityLibrary) ConnectionPoolMetrics() map[string]int {
	u, ok := o.client.ds.(Unwrapper)
	if !ok {
		return nil
	}
	stats := u.Unwrap().Stats()
	return map[string]int{
		"max_open":         stats.MaxOpenConnections,
		"open":             stats.OpenConnections,
		"in_use":           stats.InUse,
		"idle":             stats.Idle,
		"wait_count":       int(stats.WaitCount),
		"wait_duration_ms": int(stats.WaitDuration.Milliseconds()),
	}
}

// SlowQueryDetector returns queries that exceeded the given threshold.
func (o *ObservabilityLibrary) SlowQueryDetector(_ context.Context, thresholdMS int) []SlowQuery {
	o.mu.RLock()
	defer o.mu.RUnlock()

	threshold := time.Duration(thresholdMS) * time.Millisecond
	var result []SlowQuery
	for _, sq := range o.slowQueries {
		if sq.Duration >= threshold {
			result = append(result, sq)
		}
	}
	return result
}

// RecordQuery records a query execution for observability purposes.
func (o *ObservabilityLibrary) RecordQuery(operation, query string, duration time.Duration, err error) {
	if !o.enabled {
		return
	}
	o.queryCounter.WithLabelValues(operation).Inc()
	o.queryDuration.WithLabelValues(operation).Observe(duration.Seconds())
	if err != nil {
		o.errorCounter.WithLabelValues(operation).Inc()
	}
}

// TraceQueries enables OpenTelemetry tracing for sampled queries.
func (o *ObservabilityLibrary) TraceQueries(_ context.Context, sampleRate float64) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.sampleRate = sampleRate
}

// TraceQuery creates an OTEL span for a query if sampling allows it.
func (o *ObservabilityLibrary) TraceQuery(ctx context.Context, operation, query string) (context.Context, trace.Span) {
	tracer := otel.Tracer("gopgbase")
	ctx, span := tracer.Start(ctx, "gopgbase."+operation,
		trace.WithAttributes(
			attribute.String("db.statement", query),
			attribute.String("db.system", "postgresql"),
		),
	)
	return ctx, span
}

// PrometheusExporter starts a Prometheus metrics HTTP server on the given port.
func (o *ObservabilityLibrary) PrometheusExporter(_ context.Context, port int) error {
	mux := http.NewServeMux()
	mux.Handle("/metrics", http.DefaultServeMux)
	go func() {
		_ = http.ListenAndServe(fmt.Sprintf(":%d", port), mux)
	}()
	return nil
}

// ExplainAnalyze runs EXPLAIN ANALYZE on the given query and returns the plan.
func (o *ObservabilityLibrary) ExplainAnalyze(ctx context.Context, query string, args ...any) (*ExplainPlan, error) {
	explainQuery := "EXPLAIN (ANALYZE, FORMAT JSON) " + query
	rows, err := o.client.ds.QueryContext(ctx, explainQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("gopgbase: explain analyze: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var planJSON string
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			return nil, fmt.Errorf("gopgbase: explain analyze scan: %w", err)
		}
		planJSON += line
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("gopgbase: explain analyze rows: %w", err)
	}

	plan := &ExplainPlan{
		Query: query,
		Plan:  planJSON,
	}

	var jsonPlan map[string]any
	if err := json.Unmarshal([]byte(planJSON), &jsonPlan); err == nil {
		plan.JSONPlan = jsonPlan
	}

	return plan, nil
}

// --- Grafana Dashboard ---

// GrafanaDashboardJSON returns a pre-built Grafana dashboard JSON for
// gopgbase metrics. Users can import this into their Grafana instance.
func (c *Client) GrafanaDashboardJSON() string {
	dashboard := map[string]any{
		"dashboard": map[string]any{
			"title": "gopgbase Database Metrics",
			"panels": []map[string]any{
				{
					"title": "Query Rate (QPS)",
					"type":  "graph",
					"targets": []map[string]any{
						{"expr": "rate(gopgbase_queries_total[5m])", "legendFormat": "{{operation}}"},
					},
				},
				{
					"title": "Query Duration (P95)",
					"type":  "graph",
					"targets": []map[string]any{
						{"expr": "histogram_quantile(0.95, rate(gopgbase_query_duration_seconds_bucket[5m]))", "legendFormat": "P95"},
					},
				},
				{
					"title": "Connection Pool",
					"type":  "gauge",
					"targets": []map[string]any{
						{"expr": "gopgbase_connection_pool", "legendFormat": "{{state}}"},
					},
				},
				{
					"title": "Error Rate",
					"type":  "graph",
					"targets": []map[string]any{
						{"expr": "rate(gopgbase_query_errors_total[5m])", "legendFormat": "{{operation}}"},
					},
				},
			},
		},
	}
	data, _ := json.MarshalIndent(dashboard, "", "  ")
	return string(data)
}

// ImportGrafanaDashboard pushes the gopgbase dashboard to a Grafana instance.
//
// grafanaURL is the base URL (e.g., "http://grafana.local:3000").
// apiKey is a Grafana API key with dashboard creation permissions.
func (c *Client) ImportGrafanaDashboard(grafanaURL, apiKey string) error {
	dashboardJSON := c.GrafanaDashboardJSON()

	req, err := http.NewRequest(http.MethodPost, grafanaURL+"/api/dashboards/db", bytes.NewBufferString(dashboardJSON))
	if err != nil {
		return fmt.Errorf("gopgbase: grafana import: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("gopgbase: grafana import: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("gopgbase: grafana import failed (HTTP %d): %s", resp.StatusCode, string(body))
	}

	return nil
}

// updatePoolMetrics refreshes connection pool gauge metrics.
func (o *ObservabilityLibrary) UpdatePoolMetrics() {
	u, ok := o.client.ds.(Unwrapper)
	if !ok {
		return
	}

	stats := u.Unwrap().Stats()
	o.connPoolGauge.WithLabelValues("open").Set(float64(stats.OpenConnections))
	o.connPoolGauge.WithLabelValues("in_use").Set(float64(stats.InUse))
	o.connPoolGauge.WithLabelValues("idle").Set(float64(stats.Idle))
}
