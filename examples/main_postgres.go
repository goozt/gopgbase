//go:build ignore

// Example: Using gopgbase with self-hosted PostgreSQL or AWS RDS.
//
// This demonstrates single-adaptor usage with field-based and URL-based
// configuration, the Insecure flag for local development, and basic
// CRUD operations via the Client.
package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/goozt/gopgbase"
	"github.com/goozt/gopgbase/adaptors"
)

func main() {
	ctx := context.Background()
	dbHost := os.Getenv("DB_HOST")
	if dbHost == "" {
		dbHost = "localhost"
	}

	// Option 1: Field-based configuration (self-hosted or RDS).
	cfg := adaptors.PostgresConfig{
		BaseConfig: adaptors.BaseConfig{
			Host:     dbHost,
			Port:     5432,
			User:     "postgres",
			Password: "secret",
			DBName:   "mydb",
			Insecure: true, // Local development only! Disables TLS.
		},
		ApplicationName: "gopgbase-example",
	}

	// Option 2: URL-based configuration (Railway, Render, etc.).
	if dbURL := os.Getenv("DATABASE_URL"); dbURL != "" {
		cfg = adaptors.PostgresConfig{
			ConnectionURL: dbURL,
		}
	}

	ds, err := adaptors.NewPostgresAdaptor(cfg)
	if err != nil {
		log.Fatalf("failed to create postgres adaptor: %v", err)
	}
	defer ds.Close()

	// Inject the DataStore into a Client.
	client := gopgbase.NewClient(ds)

	// Ping the database.
	if err := ds.PingContext(ctx); err != nil {
		log.Fatalf("failed to ping database: %v", err)
	}
	fmt.Println("Connected to PostgreSQL!")

	// Count total admins and active MFA admins.
	totalAdmins, err := client.Count(ctx, "admins", "")
	if err != nil {
		log.Fatalf("count admins: %v", err)
	}
	mfaAdmins, err := client.Count(ctx, "admins", "mfa_enabled = $1", true)
	if err != nil {
		log.Fatalf("count mfa admins: %v", err)
	}
	fmt.Printf("Admins: total=%d, mfa_enabled=%d\n", totalAdmins, mfaAdmins)

	// Run a transaction to insert a record in admins.
	newAdminEmail := fmt.Sprintf("dev-%d@gopgbase.local", time.Now().Unix())
	err = client.Transaction(ctx, func(tx *sql.Tx) error {
		_, execErr := tx.ExecContext(ctx, "INSERT INTO admins (email, name, password_hash, role, mfa_enabled) VALUES ($1, $2, $3, $4, $5)", newAdminEmail, "Dev Example", "$2a$12$dummyhash", "admin", false)
		return execErr
	})
	if err != nil {
		log.Fatalf("transaction insert admin: %v", err)
	}
	fmt.Printf("Inserted new admin: %s\n", newAdminEmail)

	// Check existence with Exists helper.
	exists, err := client.Exists(ctx, "SELECT 1 FROM admins WHERE email = $1", newAdminEmail)
	if err != nil {
		log.Fatalf("exists check: %v", err)
	}
	fmt.Printf("Inserted admin exists: %v\n", exists)

	// Query using QueryBuilder and the plans table.
	rows, err := client.QueryBuilder().
		Select("plans").
		Columns("id", "slug", "name", "price_cents").
		Where("active = $1", true).
		OrderBy("id ASC").
		Limit(5).
		Query(ctx)
	if err != nil {
		log.Fatalf("query builder: %v", err)
	}
	defer rows.Close()
	fmt.Println("Active plans:")
	for rows.Next() {
		var id int
		var slug, name string
		var priceCents int
		if scanErr := rows.Scan(&id, &slug, &name, &priceCents); scanErr != nil {
			log.Fatalf("scan plan row: %v", scanErr)
		}
		fmt.Printf("  - %d %s (%s) %d cents\n", id, slug, name, priceCents)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		log.Fatalf("rows error: %v", rowsErr)
	}

	fmt.Println("Postgres example complete!")
}
