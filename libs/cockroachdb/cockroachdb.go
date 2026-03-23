// Package cockroachdb provides CockroachDB-specific operations including
// multi-region management, distributed SQL, backup/restore, and CDC.
package cockroachdb

import (
	"context"
	"fmt"
	"strings"

	gopgbase "github.com/goozt/gopgbase"
)

// CockroachLibrary provides CockroachDB-specific operations.
type CockroachLibrary struct {
	client *gopgbase.Client
}

// NewCockroachLibrary creates a new CockroachLibrary backed by the given Client.
func NewCockroachLibrary(client *gopgbase.Client) (*CockroachLibrary, error) {
	if client == nil {
		return nil, fmt.Errorf("gopgbase/cockroachdb: client must not be nil")
	}
	return &CockroachLibrary{client: client}, nil
}

// MultiRegionOps provides multi-region operations.
type MultiRegionOps struct {
	client *gopgbase.Client
	ctx    context.Context
}

// MultiRegionOps returns a MultiRegionOps for managing multi-region settings.
func (l *CockroachLibrary) MultiRegionOps(ctx context.Context) *MultiRegionOps {
	return &MultiRegionOps{client: l.client, ctx: ctx}
}

// AddRegion adds a region to the database.
func (m *MultiRegionOps) AddRegion(region string) error {
	query := fmt.Sprintf("ALTER DATABASE current_database() ADD REGION %s", quoteIdentifier(region))
	_, err := m.client.DataStore().ExecContext(m.ctx, query)
	if err != nil {
		return fmt.Errorf("gopgbase/cockroachdb: add region %s: %w", region, err)
	}
	return nil
}

// SetPrimaryRegion sets the primary region for the database.
func (m *MultiRegionOps) SetPrimaryRegion(region string) error {
	query := fmt.Sprintf("ALTER DATABASE current_database() SET PRIMARY REGION %s", quoteIdentifier(region))
	_, err := m.client.DataStore().ExecContext(m.ctx, query)
	if err != nil {
		return fmt.Errorf("gopgbase/cockroachdb: set primary region %s: %w", region, err)
	}
	return nil
}

// DropRegion removes a region from the database.
func (m *MultiRegionOps) DropRegion(region string) error {
	query := fmt.Sprintf("ALTER DATABASE current_database() DROP REGION %s", quoteIdentifier(region))
	_, err := m.client.DataStore().ExecContext(m.ctx, query)
	if err != nil {
		return fmt.Errorf("gopgbase/cockroachdb: drop region %s: %w", region, err)
	}
	return nil
}

// ListRegions returns the configured regions for the current database.
func (m *MultiRegionOps) ListRegions() ([]string, error) {
	rows, err := m.client.DataStore().QueryContext(m.ctx,
		"SELECT region FROM [SHOW REGIONS FROM DATABASE]")
	if err != nil {
		return nil, fmt.Errorf("gopgbase/cockroachdb: list regions: %w", err)
	}
	defer rows.Close()

	var regions []string
	for rows.Next() {
		var r string
		if err := rows.Scan(&r); err != nil {
			return nil, err
		}
		regions = append(regions, r)
	}
	return regions, rows.Err()
}

// RegionalTable sets a table's locality to REGIONAL BY ROW.
func (l *CockroachLibrary) RegionalTable(ctx context.Context, table, region string) error {
	query := fmt.Sprintf("ALTER TABLE %s SET LOCALITY REGIONAL BY ROW", quoteIdentifier(table))
	if region != "" {
		query = fmt.Sprintf("ALTER TABLE %s SET LOCALITY REGIONAL IN %s",
			quoteIdentifier(table), quoteIdentifier(region))
	}
	_, err := l.client.DataStore().ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("gopgbase/cockroachdb: regional table: %w", err)
	}
	return nil
}

// GlobalTable sets a table's locality to GLOBAL (replicated to all regions).
func (l *CockroachLibrary) GlobalTable(ctx context.Context, table string) error {
	query := fmt.Sprintf("ALTER TABLE %s SET LOCALITY GLOBAL", quoteIdentifier(table))
	_, err := l.client.DataStore().ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("gopgbase/cockroachdb: global table: %w", err)
	}
	return nil
}

// DistSQL forces distributed SQL execution for the current session.
func (l *CockroachLibrary) DistSQL(ctx context.Context, enable bool) error {
	value := "off"
	if enable {
		value = "on"
	}
	query := fmt.Sprintf("SET distsql = %s", value)
	_, err := l.client.DataStore().ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("gopgbase/cockroachdb: distsql: %w", err)
	}
	return nil
}

// EnterpriseBackup initiates a backup of the specified table or database.
// destination is a cloud storage URL (e.g., "s3://bucket/backup").
func (l *CockroachLibrary) EnterpriseBackup(ctx context.Context, target, destination string) error {
	query := fmt.Sprintf("BACKUP %s TO '%s'",
		quoteIdentifier(target),
		strings.ReplaceAll(destination, "'", "''"))
	_, err := l.client.DataStore().ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("gopgbase/cockroachdb: enterprise backup: %w", err)
	}
	return nil
}

// BackupRestore restores a table or database from a backup.
func (l *CockroachLibrary) BackupRestore(ctx context.Context, target, source string) error {
	query := fmt.Sprintf("RESTORE %s FROM '%s'",
		quoteIdentifier(target),
		strings.ReplaceAll(source, "'", "''"))
	_, err := l.client.DataStore().ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("gopgbase/cockroachdb: backup restore: %w", err)
	}
	return nil
}

// ChangeDataCapture creates a changefeed on the given table to a sink.
//
// sink is a destination URL (e.g., "kafka://broker:9092" or "webhook-https://...").
func (l *CockroachLibrary) ChangeDataCapture(ctx context.Context, table, sink string) error {
	query := fmt.Sprintf(
		"CREATE CHANGEFEED FOR %s INTO '%s' WITH updated, resolved",
		quoteIdentifier(table),
		strings.ReplaceAll(sink, "'", "''"),
	)
	_, err := l.client.DataStore().ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("gopgbase/cockroachdb: change data capture: %w", err)
	}
	return nil
}

func quoteIdentifier(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}
