//go:build ignore

// Example: Using gopgbase with CockroachDB.
//
// Demonstrates multi-region operations and writing user deployment data.
//
// Data flow:
//
//	App → CockroachDB  (deployments, deployment_logs via SQL)
//	CockroachDB → Prometheus  (native /metrics endpoint on :8080 — scraped directly,
//	                            no exporter needed; add to prometheus.yml scrape_configs:
//	                              - job_name: cockroachdb
//	                                static_configs:
//	                                  - targets: ["gopgbase_cockroachdb:8080"]
//	                                metrics_path: /_status/vars)
package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"

	"github.com/goozt/gopgbase"
	"github.com/goozt/gopgbase/adaptors"
	"github.com/goozt/gopgbase/libs/cockroachdb"
)

func main() {
	ctx := context.Background()

	cfg := adaptors.CockroachConfig{
		BaseConfig: adaptors.BaseConfig{
			Host:     os.Getenv("CRDB_HOST"),
			Port:     26257,
			User:     "postgres",
			Password: "secret",
			DBName:   "mydb",
		},
		ClusterID: os.Getenv("CRDB_CLUSTER_ID"),
	}

	ds, err := adaptors.NewCockroachAdaptor(cfg)
	if err != nil {
		log.Fatalf("failed to create cockroach adaptor: %v", err)
	}
	defer ds.Close()

	client := gopgbase.NewClient(ds)

	if err := ds.PingContext(ctx); err != nil {
		log.Fatalf("failed to ping cockroachdb: %v", err)
	}
	fmt.Println("Connected to CockroachDB!")

	// --- Multi-region setup ---
	crdbLib, err := cockroachdb.NewCockroachLibrary(client)
	if err != nil {
		log.Fatalf("failed to create cockroach library: %v", err)
	}

	ops := crdbLib.MultiRegionOps(ctx)
	if err := ops.AddRegion("us-east-1"); err != nil {
		log.Printf("add region: %v", err)
	}
	if err := crdbLib.GlobalTable(ctx, "config"); err != nil {
		log.Printf("global table: %v", err)
	}

	// --- Read user data ---
	totalUsers, err := client.Count(ctx, "users", "")
	if err != nil {
		log.Fatalf("count users: %v", err)
	}
	activeUsers, err := client.Count(ctx, "users", "status = $1", "active")
	if err != nil {
		log.Fatalf("count active users: %v", err)
	}
	fmt.Printf("Users: total=%d, active=%d\n", totalUsers, activeUsers)

	// --- Write a deployment record ---
	// Looks up the first environment and user from fixtures, then records a deployment
	// with a log entry — the same write pattern used by the real PaaS deploy pipeline.
	var envID string
	var userID int64
	err = client.ReadOnlyTransaction(ctx, func(tx *sql.Tx) error {
		return tx.QueryRowContext(ctx, `SELECT id FROM environments LIMIT 1`).Scan(&envID)
	})
	if err != nil {
		log.Fatalf("lookup environment: %v", err)
	}
	err = client.ReadOnlyTransaction(ctx, func(tx *sql.Tx) error {
		return tx.QueryRowContext(ctx, `SELECT id FROM users WHERE status = 'active' LIMIT 1`).Scan(&userID)
	})
	if err != nil {
		log.Fatalf("lookup user: %v", err)
	}

	var deploymentID string
	err = client.Transaction(ctx, func(tx *sql.Tx) error {
		err := tx.QueryRowContext(ctx,
			`INSERT INTO deployments
			 (environment_id, commit_sha, commit_message, branch, status, triggered_by, started_at)
			 VALUES ($1, $2, $3, $4, $5, $6, NOW())
			 RETURNING id`,
			envID,
			"d3adb33fd3adb33fd3adb33fd3adb33fd3adb33f",
			"example: gopgbase cockroach demo",
			"main",
			"building",
			userID,
		).Scan(&deploymentID)
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx,
			`INSERT INTO deployment_logs (deployment_id, level, message, timestamp)
			 VALUES ($1, $2, $3, NOW())`,
			deploymentID, "info", "Build triggered by gopgbase example",
		)
		return err
	})
	if err != nil {
		log.Printf("write deployment: %v", err)
	} else {
		fmt.Printf("Inserted deployment %s + log into CockroachDB\n", deploymentID)
	}

	fmt.Println("CockroachDB example complete!")
}
