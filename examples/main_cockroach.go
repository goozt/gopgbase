//go:build ignore

// Example: Using gopgbase with CockroachDB.
package main

import (
	"context"
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
			User:     os.Getenv("CRDB_USER"),
			Password: os.Getenv("CRDB_PASSWORD"),
			DBName:   os.Getenv("CRDB_DBNAME"),
		},
		ClusterID: os.Getenv("CRDB_CLUSTER_ID"),
	}

	// For local CockroachDB:
	if os.Getenv("CRDB_LOCAL") == "true" {
		cfg.Insecure = true
		cfg.Host = "localhost"
		cfg.User = "root"
		cfg.DBName = "defaultdb"
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

	// Multi-region operations.
	crdbLib, err := cockroachdb.NewCockroachLibrary(client)
	if err != nil {
		log.Fatalf("failed to create cockroach library: %v", err)
	}

	ops := crdbLib.MultiRegionOps(ctx)
	if err := ops.AddRegion("us-east-1"); err != nil {
		log.Printf("add region: %v", err)
	}

	// Global table.
	if err := crdbLib.GlobalTable(ctx, "config"); err != nil {
		log.Printf("global table: %v", err)
	}

	fmt.Println("CockroachDB example complete!")
}
