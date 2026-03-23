//go:build ignore

// Example: Using gopgbase with Neon serverless PostgreSQL.
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/goozt/gopgbase"
	"github.com/goozt/gopgbase/adaptors"
	"github.com/goozt/gopgbase/libs/neon"
)

func main() {
	ctx := context.Background()

	cfg := adaptors.NeonConfig{
		ConnectionURL: os.Getenv("NEON_DATABASE_URL"),
	}

	ds, err := adaptors.NewNeonAdaptor(cfg)
	if err != nil {
		log.Fatalf("failed to create neon adaptor: %v", err)
	}
	defer ds.Close()

	client := gopgbase.NewClient(ds)

	if err := ds.PingContext(ctx); err != nil {
		log.Fatalf("failed to ping neon: %v", err)
	}
	fmt.Println("Connected to Neon!")

	// Neon-specific operations.
	neonLib, err := neon.NewNeonLibrary(client)
	if err != nil {
		log.Fatalf("failed to create neon library: %v", err)
	}

	// Enable pgvector for AI/vector operations.
	if err := neonLib.EnablePgVector(ctx); err != nil {
		log.Printf("enable pgvector: %v", err)
	}

	// Check serverless scaling.
	scale, err := neonLib.ServerlessScale(ctx)
	if err != nil {
		log.Printf("serverless scale: %v", err)
	} else {
		fmt.Printf("Neon scaling: %v\n", scale)
	}

	fmt.Println("Neon example complete!")
}
