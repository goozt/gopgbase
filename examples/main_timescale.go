//go:build ignore

// Example: Using gopgbase with TimescaleDB.
//
// Demonstrates writing server metrics, app metrics, and HTTP request logs
// directly to TimescaleDB hypertables, and exposing Go runtime metrics for
// Prometheus scraping on :2112/metrics.
//
// Data flow:
//
//	App → TimescaleDB  (server_metrics, app_metrics, http_request_logs via SQL)
//	App → :2112/metrics  (Prometheus scrapes Go runtime counters here)
package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"math/rand/v2"
	"net/http"
	"os"
	"runtime"
	"time"

	"github.com/goozt/gopgbase"
	"github.com/goozt/gopgbase/adaptors"
	"github.com/goozt/gopgbase/libs/timescale"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// writeServerMetrics inserts Go runtime stats into server_metrics every interval.
// In production replace the simulated cpu/disk values with gopsutil readings.
func writeServerMetrics(ctx context.Context, client *gopgbase.Client, serviceID int, region, host string, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	var mem runtime.MemStats
	for {
		select {
		case <-ticker.C:
			runtime.ReadMemStats(&mem)
			err := client.Transaction(ctx, func(tx *sql.Tx) error {
				_, err := tx.ExecContext(ctx,
					`INSERT INTO server_metrics
					 (time, service_id, host, region, cpu_percent, memory_percent, memory_bytes,
					  disk_percent, disk_read_bps, disk_write_bps, net_rx_bps, net_tx_bps,
					  load_avg_1m, load_avg_5m, open_connections, goroutines)
					 VALUES (NOW(), $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)`,
					serviceID, host, region,
					rand.Float64()*100,                      // cpu_percent   — simulated
					float64(mem.Alloc)/float64(mem.Sys)*100, // memory_percent
					int64(mem.Alloc),                        // memory_bytes
					rand.Float64()*80,                       // disk_percent  — simulated
					int64(rand.Float64()*200000),            // disk_read_bps
					int64(rand.Float64()*150000),            // disk_write_bps
					int64(rand.Float64()*500000),            // net_rx_bps
					int64(rand.Float64()*400000),            // net_tx_bps
					rand.Float64()*3,                        // load_avg_1m
					rand.Float64()*2.5,                      // load_avg_5m
					rand.IntN(100),                          // open_connections
					runtime.NumGoroutine(),                  // goroutines
				)
				return err
			})
			if err != nil {
				log.Printf("write server_metrics: %v", err)
			}
		case <-ctx.Done():
			return
		}
	}
}

// writeAppMetrics inserts application-level counters into app_metrics every interval.
func writeAppMetrics(ctx context.Context, client *gopgbase.Client, serviceID int, region string, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			err := client.Transaction(ctx, func(tx *sql.Tx) error {
				_, err := tx.ExecContext(ctx,
					`INSERT INTO app_metrics
					 (time, service_id, region, requests_per_sec, errors_per_sec,
					  p50_latency_ms, p95_latency_ms, p99_latency_ms,
					  active_users, active_websockets, cache_hit_rate,
					  db_pool_active, db_pool_idle, queue_depth)
					 VALUES (NOW(), $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`,
					serviceID, region,
					100+rand.Float64()*400,   // requests_per_sec
					rand.Float64()*5,         // errors_per_sec
					5+rand.Float64()*15,      // p50_latency_ms
					20+rand.Float64()*80,     // p95_latency_ms
					50+rand.Float64()*200,    // p99_latency_ms
					rand.IntN(500),           // active_users
					rand.IntN(100),           // active_websockets
					0.80+rand.Float64()*0.19, // cache_hit_rate
					rand.IntN(20),            // db_pool_active
					rand.IntN(10),            // db_pool_idle
					rand.IntN(50),            // queue_depth
				)
				return err
			})
			if err != nil {
				log.Printf("write app_metrics: %v", err)
			}
		case <-ctx.Done():
			return
		}
	}
}

// statusCapture wraps ResponseWriter to capture the HTTP status code.
type statusCapture struct {
	http.ResponseWriter
	status int
}

func (sc *statusCapture) WriteHeader(code int) {
	sc.status = code
	sc.ResponseWriter.WriteHeader(code)
}

// httpLogger wraps a handler and records each request into http_request_logs.
func httpLogger(client *gopgbase.Client, serviceID int, region string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sc := &statusCapture{ResponseWriter: w, status: 200}
		next.ServeHTTP(sc, r)
		latencyMs := float64(time.Since(start).Microseconds()) / 1000

		ctx := r.Context()
		_ = client.Transaction(ctx, func(tx *sql.Tx) error {
			_, err := tx.ExecContext(ctx,
				`INSERT INTO http_request_logs
				 (time, service_id, region, method, path, status_code, latency_ms,
				  request_bytes, response_bytes, ip_address, user_agent)
				 VALUES (NOW(), $1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
				serviceID, region,
				r.Method, r.URL.Path, sc.status, latencyMs,
				int(r.ContentLength), 0,
				r.RemoteAddr, r.UserAgent(),
			)
			return err
		})
	})
}

func main() {
	ctx := context.Background()

	tsCfg := adaptors.TimescaleConfig{
		BaseConfig: adaptors.BaseConfig{
			Host:     os.Getenv("TIMESCALE_HOST"),
			Port:     5432,
			User:     os.Getenv("TIMESCALE_USER"),
			Password: os.Getenv("TIMESCALE_PASSWORD"),
			DBName:   os.Getenv("TIMESCALE_DBNAME"),
			Insecure: os.Getenv("TIMESCALE_INSECURE") == "true",
		},
	}

	tsDS, err := adaptors.NewTimescaleAdaptor(tsCfg)
	if err != nil {
		log.Fatalf("failed to create timescale adaptor: %v", err)
	}
	defer tsDS.Close()

	tsClient := gopgbase.NewClient(tsDS)

	if err := tsDS.PingContext(ctx); err != nil {
		log.Fatalf("failed to ping timescaledb: %v", err)
	}
	fmt.Println("Connected to TimescaleDB!")

	if err := tsClient.TunePool(ctx, runtime.NumCPU(), 1000); err != nil {
		log.Printf("tune pool: %v", err)
	}

	tsClient.EnableObservability(ctx)

	// --- TimescaleDB hypertable setup ---
	tsLib, err := timescale.NewTimescaleLibrary(tsClient)
	if err != nil {
		log.Fatalf("failed to create timescale library: %v", err)
	}
	if err := tsLib.CreateHypertable(ctx, "metrics", "time", "device_id", true); err != nil {
		log.Printf("create hypertable: %v", err)
	}
	if err := tsLib.AddCompressionPolicy(ctx, "metrics", "7 days"); err != nil {
		log.Printf("compression policy: %v", err)
	}
	if err := tsLib.AddRetentionPolicy(ctx, "metrics", "90 days"); err != nil {
		log.Printf("retention policy: %v", err)
	}

	// --- Resolve service identity ---
	// Uses service_id=1 (first row in monitored_services from fixtures).
	// In production, register/look up the service by name at startup.
	const (
		serviceID = 1
		region    = "ap-south-1"
	)
	host, _ := os.Hostname()

	// --- Start writers → TimescaleDB ---
	go writeServerMetrics(ctx, tsClient, serviceID, region, host, 5*time.Second)
	go writeAppMetrics(ctx, tsClient, serviceID, region, 15*time.Second)
	fmt.Println("Writing server_metrics every 5s and app_metrics every 15s to TimescaleDB...")

	// --- HTTP server ---
	// /metrics  → Prometheus scrapes Go runtime counters here.
	//             Add to prometheus.yml scrape_configs:
	//               - job_name: gopgbase-app
	//                 static_configs:
	//                   - targets: ["host.docker.internal:2112"]
	// /healthz  → liveness probe
	// Port 2112 avoids collision with the Prometheus container mapped to host :9090.
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/healthz", tsClient.HealthCheckHandler)

	fmt.Println("Prometheus metrics: http://localhost:2112/metrics")
	if err := http.ListenAndServe(":2112", httpLogger(tsClient, serviceID, region, mux)); err != nil {
		log.Fatalf("server: %v", err)
	}
}
