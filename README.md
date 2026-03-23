# gopgbase

A unified, security-first Go client for PostgreSQL-compatible databases with constructor-injected adaptors, fluent query building, and production-grade observability.

## Overview

`gopgbase` abstracts multiple PostgreSQL-compatible databases behind a single `DataStore` interface. Inject one or many adaptors at runtime using **Constructor Injection** — the `Client` never creates its own connections.

### Supported Providers

| Provider | Adaptor | Default Port | Default SSL |
|----------|---------|-------------|-------------|
| PostgreSQL (self-hosted) | `NewPostgresAdaptor` | 5432 | `verify-full` |
| AWS RDS for PostgreSQL | `NewPostgresAdaptor` | 5432 | `verify-full` |
| Railway / Render | `NewPostgresAdaptor` | 5432 | `verify-full` |
| Supabase | `NewSupabaseAdaptor` | 5432/6543 | `verify-full` |
| CockroachDB | `NewCockroachAdaptor` | 26257 | `verify-full` |
| Neon | `NewNeonAdaptor` | 5432 | `require` |
| Amazon Redshift | `NewRedshiftAdaptor` | 5439 | `verify-full` |
| TimescaleDB | `NewTimescaleAdaptor` | 5432 | `verify-full` |

## Installation

```bash
go get github.com/goozt/gopgbase
```

## Quick Start

### Single Adaptor (PostgreSQL)

```go
package main

import (
    "context"
    "database/sql"
    "log"

    "github.com/goozt/gopgbase"
    "github.com/goozt/gopgbase/adaptors"
)

func main() {
    ctx := context.Background()

    // Create an adaptor (manages its own connection pool).
    ds, err := adaptors.NewPostgresAdaptor(adaptors.PostgresConfig{
        BaseConfig: adaptors.BaseConfig{
            Host: "localhost", Port: 5432,
            User: "postgres", Password: "secret", DBName: "mydb",
            Insecure: true, // Local dev only! Disables TLS.
        },
    })
    if err != nil {
        log.Fatal(err)
    }
    defer ds.Close()

    // Inject into Client — all DB access goes through DataStore.
    client := gopgbase.NewClient(ds)

    // Transaction with auto commit/rollback.
    err = client.Transaction(ctx, func(tx *sql.Tx) error {
        _, err := tx.ExecContext(ctx, "INSERT INTO users (name) VALUES ($1)", "Alice")
        return err
    })
    if err != nil {
        log.Fatal(err)
    }

    // Convenience helpers.
    count, _ := client.Count(ctx, "users", "active = $1", true)
    log.Printf("Active users: %d", count)
}
```

### URL-Based Configuration (Railway, Render)

```go
ds, err := adaptors.NewPostgresAdaptor(adaptors.PostgresConfig{
    ConnectionURL: os.Getenv("DATABASE_URL"),
})
```

### Supabase Adaptor

```go
ds, err := adaptors.NewSupabaseAdaptor(adaptors.SupabaseConfig{
    ConnectionURL: os.Getenv("SUPABASE_DB_URL"),
})
client := gopgbase.NewClient(ds)

// Use Supabase companion library for RLS, auth, storage.
sbLib, _ := supabase.NewSupabaseLibrary(client, supabase.Config{
    ProjectURL:     os.Getenv("SUPABASE_URL"),
    APIKey:         os.Getenv("SUPABASE_ANON_KEY"),
    ServiceRoleKey: os.Getenv("SUPABASE_SERVICE_ROLE_KEY"),
})
sbLib.EnableRLS(ctx, "profiles", "policy_name", "USING (auth.uid() = user_id)")
```

### Multiple Adaptors (Supabase + Redshift)

```go
// Each adaptor has its own connection pool.
sbDS, _ := adaptors.NewSupabaseAdaptor(supabaseCfg)
rsDS, _ := adaptors.NewRedshiftAdaptor(redshiftCfg)

// Separate clients for different workloads.
userClient := gopgbase.NewClient(sbDS)     // OLTP: user data
analyticsClient := gopgbase.NewClient(rsDS) // OLAP: analytics

count, _ := userClient.Count(ctx, "users", "")
exists, _ := analyticsClient.Exists(ctx, "SELECT 1 FROM daily_metrics WHERE date = $1", today)
```

## Architecture

```
┌─────────────┐     ┌──────────────┐
│   Client    │────▶│  DataStore   │ (interface)
└─────────────┘     └──────────────┘
                           │
        ┌──────────────────┼──────────────────┐
        ▼                  ▼                  ▼
  ┌──────────┐      ┌──────────┐      ┌──────────┐
  │ Postgres │      │ Supabase │      │  Custom  │
  │ Adaptor  │      │ Adaptor  │      │  Mock    │
  └──────────┘      └──────────┘      └──────────┘
       │                  │
       ▼                  ▼
    *sql.DB            *sql.DB
```

### DataStore Interface

```go
type DataStore interface {
    QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
    QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
    ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
    BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error)
    PingContext(ctx context.Context) error
    Close() error
}
```

All database access flows through this interface. Users can implement their own `DataStore` for mocking, custom pools, or alternative drivers — no internal types required.

Concrete adaptors also expose `Unwrap() *sql.DB` for interop with tools like goose.

## Client Features

### QueryBuilder (Fluent DSL)

```go
rows, err := client.QueryBuilder().
    Select("users").
    Columns("id", "name", "email").
    Join("INNER JOIN orders ON users.id = orders.user_id").
    Where("age > ?", 18).          // ? auto-converts to $N
    Where("active = $2", true).    // $N passed as-is (don't mix!)
    GroupBy("name").
    OrderBy("name ASC").
    Limit(10).Offset(20).
    Query(ctx)
```

### Transactions

```go
// Auto commit/rollback with panic recovery.
client.Transaction(ctx, func(tx *sql.Tx) error { ... })

// With isolation level.
client.TransactionWithIsolation(ctx, sql.LevelSerializable, func(tx *sql.Tx) error { ... })

// Read-only transaction.
client.ReadOnlyTransaction(ctx, func(tx *sql.Tx) error { ... })

// Savepoints (nested transactions).
client.Transaction(ctx, func(tx *sql.Tx) error {
    client.Savepoint(ctx, tx, "sp1", func(tx *sql.Tx) error { ... })
    return nil
})

// Batch operations in single transaction.
client.BatchTransaction(ctx, op1, op2, op3)
```

### StructScan

```go
type User struct {
    ID   int    `db:"id"`
    Name string `db:"name"`
}

rows, _ := client.DataStore().QueryContext(ctx, "SELECT id, name FROM users")
for rows.Next() {
    var u User
    client.StructScan(ctx, rows, &u)
}
```

### Bulk Operations

```go
// BulkInsert — parameterized multi-row INSERT.
n, err := client.BulkInsert(ctx, "users", []string{"name", "age"},
    [][]any{{"Alice", 30}, {"Bob", 25}, {"Charlie", 35}})

// BulkCopy — pgx COPY protocol (falls back to BulkInsert for non-pgx).
n, err := client.BulkCopy(ctx, "metrics", []string{"time", "value"}, data)
```

### ForEachRow (Memory-Efficient)

```go
err := client.ForEachRow(ctx, "SELECT id, name FROM users", nil,
    func(row map[string]any) error {
        fmt.Println(row["name"])
        return nil
    })
```

## Security: The `Insecure` Flag

| `Insecure` | Behavior | Use Case |
|------------|----------|----------|
| `false` (default) | TLS enabled, certificates verified (`verify-full` or provider equivalent) | **Production** |
| `true` | TLS disabled (`sslmode=disable`) | **Local development only** |

`Insecure` only affects TLS/SSL settings. It never bypasses SQL parameterization or other safety measures.

**Never use `Insecure: true` in production.**

## Companion Libraries

Optional, provider-specific helper libraries in `libs/`:

| Package | Provider | Key Features |
|---------|----------|-------------|
| `libs/common` | All | Pagination, SoftDelete, AuditTrail, SchemaDiff, Migrations |
| `libs/postgres` | PostgreSQL | Extensions, VacuumAnalyze, IndexAdvisor, ReplicationLag, LockWatcher |
| `libs/timescale` | TimescaleDB | Hypertables, TimeBucket, ContinuousAggs, Compression, LTTB |
| `libs/supabase` | Supabase | RLS, Auth/JWT, EdgeFunctions, Storage, UserManager |
| `libs/redshift` | Redshift | Vacuum, MaterializedViews, WLM, Spectrum |
| `libs/cockroachdb` | CockroachDB | MultiRegion, GlobalTables, DistSQL, Backup, CDC |
| `libs/neon` | Neon | pgvector, VectorIndex, Branching, ConnectionPooler |

### Migrations (via Goose)

```go
//go:embed migrations/*.sql
var MigrationsFS embed.FS

migrator := common.NewMigrateLibrary(client)
migrator.Up(ctx, MigrationsFS)        // Run pending migrations
migrator.Down(ctx, MigrationsFS, 1)   // Rollback 1
migrator.Version(ctx)                 // Current version
migrator.Status(ctx, MigrationsFS)    // Print status
```

CLI users can use `goose` directly:
```bash
go install github.com/pressly/goose/v3/cmd/goose@latest
goose -dir migrations postgres "$DATABASE_URL" up
```

## Observability

```go
obs := client.EnableObservability(ctx)

// Prometheus metrics auto-registered: gopgbase_queries_total, gopgbase_query_duration_seconds, etc.
http.Handle("/metrics", promhttp.Handler())
http.HandleFunc("/healthz", client.HealthCheckHandler)

// Import pre-built Grafana dashboard.
client.ImportGrafanaDashboard("http://grafana:3000", apiKey)

// Connection pool tuning.
client.TunePool(ctx, runtime.NumCPU(), 1000)
```

## Development

```bash
# Install Task (one-time)
go install github.com/go-task/task/v3/cmd/task@latest

# Setup dev tools
task init

# Daily workflow
task test          # Run tests
task lint          # Pre-commit checks
task testcover     # Coverage report
task bench         # Benchmarks
task gen           # Regenerate mocks
task ci            # Full CI pipeline
```

## Project Structure

```
gopgbase/
├── datastore.go          # DataStore interface
├── client.go             # Client, QueryBuilder, StructScan, BulkCopy
├── tx.go                 # Transaction helpers
├── observability.go      # Prometheus, OTEL, Grafana
├── adaptors/
│   ├── config.go         # BaseConfig, pgxDataStore
│   ├── postgres.go       # PostgreSQL, RDS, Railway, Render
│   ├── supabase.go
│   ├── cockroach.go
│   ├── neon.go
│   ├── redshift.go
│   └── timescale.go
├── libs/
│   ├── common/           # Pagination, AuditTrail, Migrations, ExplainAnalyze
│   ├── postgres/         # Extensions, IndexAdvisor, LockWatcher
│   ├── timescale/        # Hypertables, ContinuousAggs, LTTB
│   ├── supabase/         # RLS, Auth, EdgeFunctions, Storage
│   ├── redshift/         # Vacuum, Spectrum, WLM
│   ├── cockroachdb/      # MultiRegion, CDC, Backup
│   └── neon/             # pgvector, Branching
├── examples/
├── testdata/
├── .github/workflows/    # CI + Release
├── Taskfile.yml
└── Makefile
```

## License

See [LICENSE](LICENSE) for details.
