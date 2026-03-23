package common

import (
	"context"
	"fmt"

	gopgbase "github.com/goozt/gopgbase"
)

// AuditEntry represents a single audit log entry.
type AuditEntry struct {
	Operation string `json:"operation" db:"operation"`
	TableName string `json:"table_name" db:"table_name"`
	OldData   string `json:"old_data,omitempty" db:"old_data"`
	NewData   string `json:"new_data,omitempty" db:"new_data"`
	ChangedAt string `json:"changed_at" db:"changed_at"`
	ChangedBy string `json:"changed_by,omitempty" db:"changed_by"`
	ID        int64  `json:"id" db:"id"`
}

// SetupAuditTrail creates the audit infrastructure (table, function, trigger)
// for the given source table. This is a standalone function for users who
// don't want to use CommonLibrary.
func SetupAuditTrail(ctx context.Context, client *gopgbase.Client, table, auditTable string) error {
	lib, err := NewCommonLibrary(client)
	if err != nil {
		return err
	}
	return lib.AuditTrail(ctx, table, auditTable)
}

// GetAuditLog retrieves audit entries for a given table, ordered by most recent first.
func GetAuditLog(ctx context.Context, client *gopgbase.Client, auditTable string, limit int) ([]AuditEntry, error) {
	if limit <= 0 {
		limit = 100
	}

	query := fmt.Sprintf(
		"SELECT id, operation, table_name, old_data::text, new_data::text, changed_at::text, COALESCE(changed_by, '') FROM %s ORDER BY changed_at DESC LIMIT $1",
		quoteIdentifier(auditTable),
	)

	rows, err := client.DataStore().QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("gopgbase/common: get audit log: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var entries []AuditEntry
	for rows.Next() {
		var e AuditEntry
		if err := rows.Scan(&e.ID, &e.Operation, &e.TableName, &e.OldData, &e.NewData, &e.ChangedAt, &e.ChangedBy); err != nil {
			return nil, fmt.Errorf("gopgbase/common: get audit log scan: %w", err)
		}
		entries = append(entries, e)
	}

	return entries, rows.Err()
}
