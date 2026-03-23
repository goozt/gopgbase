//go:build ignore

// Example: Using gopgbase with Supabase.
//
// Demonstrates Supabase adaptor initialization, Client usage,
// and Supabase-specific features via the companion library.
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/goozt/gopgbase"
	"github.com/goozt/gopgbase/adaptors"
	"github.com/goozt/gopgbase/libs/supabase"
)

func main() {
	ctx := context.Background()

	// Supabase connection via URL (from dashboard).
	cfg := adaptors.SupabaseConfig{
		ConnectionURL:  os.Getenv("SUPABASE_DB_URL"),
		ProjectRef:     os.Getenv("SUPABASE_PROJECT_REF"),
		APIKey:         os.Getenv("SUPABASE_ANON_KEY"),
		ServiceRoleKey: os.Getenv("SUPABASE_SERVICE_ROLE_KEY"),
	}

	// For local Supabase (supabase start):
	if os.Getenv("SUPABASE_LOCAL") == "true" {
		cfg = adaptors.SupabaseConfig{
			BaseConfig: adaptors.BaseConfig{
				Host:     "localhost",
				Port:     54322,
				User:     "postgres",
				Password: "postgres",
				DBName:   "postgres",
				Insecure: true, // Local dev only!
			},
		}
	}

	ds, err := adaptors.NewSupabaseAdaptor(cfg)
	if err != nil {
		log.Fatalf("failed to create supabase adaptor: %v", err)
	}
	defer ds.Close()

	client := gopgbase.NewClient(ds)

	// Verify connection.
	if err := ds.PingContext(ctx); err != nil {
		log.Fatalf("failed to ping supabase: %v", err)
	}
	fmt.Println("Connected to Supabase!")

	// Use the Supabase companion library for RLS, auth, etc.
	sbLib, err := supabase.NewSupabaseLibrary(client, supabase.Config{
		ProjectURL:     os.Getenv("SUPABASE_URL"),
		APIKey:         os.Getenv("SUPABASE_ANON_KEY"),
		ServiceRoleKey: os.Getenv("SUPABASE_SERVICE_ROLE_KEY"),
	})
	if err != nil {
		log.Fatalf("failed to create supabase library: %v", err)
	}

	// Enable Row Level Security.
	err = sbLib.EnableRLS(ctx, "profiles", "user_profiles_policy",
		"FOR ALL USING (auth.uid() = user_id)")
	if err != nil {
		log.Printf("enable rls: %v", err)
	}

	fmt.Println("Supabase example complete!")
}
