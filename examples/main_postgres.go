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

	"github.com/goozt/gopgbase"
	"github.com/goozt/gopgbase/adaptors"
)

func main() {
	ctx := context.Background()

	// Option 1: Field-based configuration (self-hosted or RDS).
	cfg := adaptors.PostgresConfig{
		BaseConfig: adaptors.BaseConfig{
			Host:     "localhost",
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

	// Run a transaction.
	err = client.Transaction(ctx, func(tx *sql.Tx) error {
		_, execErr := tx.ExecContext(ctx, "INSERT INTO users (name, email) VALUES ($1, $2)", "Dave", "dave@example.com")
		return execErr
	})
	if err != nil {
		log.Printf("transaction: %v", err)
	}

	// Count rows.
	count, err := client.Count(ctx, "users", "active = $1", true)
	if err != nil {
		log.Printf("count: %v", err)
	} else {
		fmt.Printf("Active users: %d\n", count)
	}

	// Check existence.
	exists, err := client.Exists(ctx, "SELECT 1 FROM users WHERE email = $1", "alice@example.com")
	if err != nil {
		log.Printf("exists: %v", err)
	} else {
		fmt.Printf("User exists: %v\n", exists)
	}

	// QueryBuilder.
	rows, err := client.QueryBuilder().
		Select("users").
		Columns("id", "name", "email").
		Where("age > ?", 18).
		OrderBy("name ASC").
		Limit(10).
		Query(ctx)
	if err != nil {
		log.Printf("query builder: %v", err)
	} else {
		rows.Close()
		fmt.Println("QueryBuilder executed successfully")
	}

	fmt.Println("Postgres example complete!")
}
