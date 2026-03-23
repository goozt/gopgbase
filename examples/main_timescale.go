//go:build ignore

// Example: Using gopgbase with TimescaleDB.
//
// Demonstrates multi-adaptor usage: TimescaleDB for time-series data
// alongside Supabase for user data, each with its own Client.
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"runtime"

	"github.com/goozt/gopgbase"
	"github.com/goozt/gopgbase/adaptors"
	"github.com/goozt/gopgbase/libs/timescale"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	ctx := context.Background()

	// --- TimescaleDB adaptor ---
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

	// Tune connection pool.
	if err := tsClient.TunePool(ctx, runtime.NumCPU(), 1000); err != nil {
		log.Printf("tune pool: %v", err)
	}

	// Enable observability.
	tsClient.EnableObservability(ctx)

	// Grafana dashboard.
	grafanaURL := os.Getenv("GRAFANA_URL")
	if grafanaURL == "" {
		grafanaURL = "http://grafana.local:3000"
	}
	grafanaKey := os.Getenv("GRAFANA_API_KEY")
	if grafanaKey != "" {
		if err := tsClient.ImportGrafanaDashboard(grafanaURL, grafanaKey); err != nil {
			log.Printf("grafana import: %v", err)
		}
	}

	// --- TimescaleDB operations ---
	tsLib, err := timescale.NewTimescaleLibrary(tsClient)
	if err != nil {
		log.Fatalf("failed to create timescale library: %v", err)
	}

	// Create a hypertable.
	if err := tsLib.CreateHypertable(ctx, "metrics", "time", "device_id", true); err != nil {
		log.Printf("create hypertable: %v", err)
	}

	// Add compression policy.
	if err := tsLib.AddCompressionPolicy(ctx, "metrics", "7 days"); err != nil {
		log.Printf("compression policy: %v", err)
	}

	// Add retention policy.
	if err := tsLib.AddRetentionPolicy(ctx, "metrics", "90 days"); err != nil {
		log.Printf("retention policy: %v", err)
	}

	// --- Supabase adaptor (multi-adaptor demo) ---
	if sbURL := os.Getenv("SUPABASE_DB_URL"); sbURL != "" {
		sbDS, err := adaptors.NewSupabaseAdaptor(adaptors.SupabaseConfig{
			ConnectionURL: sbURL,
		})
		if err != nil {
			log.Fatalf("failed to create supabase adaptor: %v", err)
		}
		defer sbDS.Close()

		sbClient := gopgbase.NewClient(sbDS)
		count, err := sbClient.Count(ctx, "users", "")
		if err != nil {
			log.Printf("supabase count: %v", err)
		} else {
			fmt.Printf("Supabase users: %d\n", count)
		}
	}

	// --- HTTP endpoints ---
	http.Handle("/metrics", promhttp.Handler())
	http.HandleFunc("/healthz", tsClient.HealthCheckHandler)

	fmt.Println("Starting server on :9090...")
	go func() {
		if err := http.ListenAndServe(":9090", nil); err != nil {
			log.Fatalf("server: %v", err)
		}
	}()

	fmt.Println("TimescaleDB + multi-adaptor example running!")
	select {} // Block forever
}
