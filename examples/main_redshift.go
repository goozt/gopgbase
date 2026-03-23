//go:build ignore

// Example: Using gopgbase with Amazon Redshift.
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/goozt/gopgbase"
	"github.com/goozt/gopgbase/adaptors"
	"github.com/goozt/gopgbase/libs/redshift"
)

func main() {
	ctx := context.Background()

	cfg := adaptors.RedshiftConfig{
		BaseConfig: adaptors.BaseConfig{
			Host:     os.Getenv("REDSHIFT_HOST"),
			Port:     5439,
			User:     os.Getenv("REDSHIFT_USER"),
			Password: os.Getenv("REDSHIFT_PASSWORD"),
			DBName:   os.Getenv("REDSHIFT_DBNAME"),
		},
		ClusterIdentifier: os.Getenv("REDSHIFT_CLUSTER_ID"),
		Region:            os.Getenv("AWS_REGION"),
		StatementTimeout:  300000, // 5 minute timeout for OLAP queries
	}

	ds, err := adaptors.NewRedshiftAdaptor(cfg)
	if err != nil {
		log.Fatalf("failed to create redshift adaptor: %v", err)
	}
	defer ds.Close()

	client := gopgbase.NewClient(ds)

	if err := ds.PingContext(ctx); err != nil {
		log.Fatalf("failed to ping redshift: %v", err)
	}
	fmt.Println("Connected to Redshift!")

	// Redshift-specific operations.
	rsLib, err := redshift.NewRedshiftLibrary(client)
	if err != nil {
		log.Fatalf("failed to create redshift library: %v", err)
	}

	// Vacuum and analyze.
	v := rsLib.Vacuum()
	if err := v.Full(ctx, "events"); err != nil {
		log.Printf("vacuum: %v", err)
	}

	// Create a materialized view.
	if err := rsLib.MaterializedView(ctx, "daily_events",
		"SELECT date_trunc('day', event_time) AS day, COUNT(*) FROM events GROUP BY 1"); err != nil {
		log.Printf("materialized view: %v", err)
	}

	fmt.Println("Redshift example complete!")
}
