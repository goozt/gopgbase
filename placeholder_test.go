package gopgbase

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConvertPlaceholders(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr string
	}{
		// Fast path: no ? at all.
		{
			name:  "no placeholders",
			input: "SELECT * FROM users WHERE id = $1",
			want:  "SELECT * FROM users WHERE id = $1",
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},

		// Basic ? → $N conversion.
		{
			name:  "single question mark",
			input: "SELECT * FROM users WHERE id = ?",
			want:  "SELECT * FROM users WHERE id = $1",
		},
		{
			name:  "multiple question marks",
			input: "SELECT * FROM users WHERE age > ? AND name = ?",
			want:  "SELECT * FROM users WHERE age > $1 AND name = $2",
		},
		{
			name:  "three question marks",
			input: "INSERT INTO t (a, b, c) VALUES (?, ?, ?)",
			want:  "INSERT INTO t (a, b, c) VALUES ($1, $2, $3)",
		},

		// ?? → literal ? (JSONB operator escape).
		{
			name:  "double question mark escape",
			input: "SELECT * FROM t WHERE data ?? 'key'",
			want:  "SELECT * FROM t WHERE data ? 'key'",
		},
		{
			name:  "double question mark with pipe (JSONB ?| operator)",
			input: "SELECT * FROM t WHERE data ??| array['a','b']",
			want:  "SELECT * FROM t WHERE data ?| array['a','b']",
		},
		{
			name:  "double question mark with ampersand (JSONB ?& operator)",
			input: "SELECT * FROM t WHERE data ??& array['a','b']",
			want:  "SELECT * FROM t WHERE data ?& array['a','b']",
		},
		{
			name:  "mixed ?? and ? placeholders",
			input: "SELECT * FROM t WHERE data ?? 'key' AND id = ?",
			want:  "SELECT * FROM t WHERE data ? 'key' AND id = $1",
		},

		// Mixed placeholder error.
		{
			name:    "mixed ? and $N",
			input:   "SELECT * FROM t WHERE a = ? AND b = $2",
			wantErr: "mixed placeholders",
		},
		{
			name:    "mixed $1 and ?",
			input:   "SELECT * FROM t WHERE a = $1 AND b = ?",
			wantErr: "mixed placeholders",
		},

		// Single-quoted strings: ? inside should be ignored.
		{
			name:  "question mark inside single quotes",
			input: "SELECT * FROM t WHERE name = '?' AND id = ?",
			want:  "SELECT * FROM t WHERE name = '?' AND id = $1",
		},
		{
			name:  "escaped single quote",
			input: "SELECT * FROM t WHERE name = 'it''s?' AND id = ?",
			want:  "SELECT * FROM t WHERE name = 'it''s?' AND id = $1",
		},
		{
			name:    "unterminated single quote",
			input:   "SELECT * FROM t WHERE name = 'abc? AND id = ?",
			wantErr: "unterminated single-quoted string",
		},

		// Double-quoted identifiers: ? inside should be ignored.
		{
			name:  "question mark inside double quotes",
			input: `SELECT "col?" FROM t WHERE id = ?`,
			want:  `SELECT "col?" FROM t WHERE id = $1`,
		},
		{
			name:  "escaped double quote",
			input: `SELECT "col""?" FROM t WHERE id = ?`,
			want:  `SELECT "col""?" FROM t WHERE id = $1`,
		},
		{
			name:    "unterminated double quote",
			input:   `SELECT "col? FROM t WHERE id = ?`,
			wantErr: "unterminated double-quoted identifier",
		},

		// E-strings: ? inside should be ignored.
		{
			name:  "question mark inside E-string",
			input: `SELECT * FROM t WHERE name = E'what?' AND id = ?`,
			want:  `SELECT * FROM t WHERE name = E'what?' AND id = $1`,
		},
		{
			name:  "E-string with backslash escape",
			input: `SELECT * FROM t WHERE name = E'what\?' AND id = ?`,
			want:  `SELECT * FROM t WHERE name = E'what\?' AND id = $1`,
		},
		{
			name:  "E-string with escaped quote",
			input: `SELECT * FROM t WHERE name = E'it''s' AND id = ?`,
			want:  `SELECT * FROM t WHERE name = E'it''s' AND id = $1`,
		},
		{
			name:  "lowercase e-string",
			input: `SELECT * FROM t WHERE name = e'what?' AND id = ?`,
			want:  `SELECT * FROM t WHERE name = e'what?' AND id = $1`,
		},
		{
			name:    "unterminated E-string",
			input:   `SELECT * FROM t WHERE name = E'abc? AND id = ?`,
			wantErr: "unterminated E-string",
		},

		// Dollar-quoted strings: ? inside should be ignored.
		{
			name:  "question mark inside dollar quote",
			input: "SELECT * FROM t WHERE body = $$what?$$ AND id = ?",
			want:  "SELECT * FROM t WHERE body = $$what?$$ AND id = $1",
		},
		{
			name:  "question mark inside tagged dollar quote",
			input: "SELECT * FROM t WHERE body = $tag$what?$tag$ AND id = ?",
			want:  "SELECT * FROM t WHERE body = $tag$what?$tag$ AND id = $1",
		},
		{
			name:    "unterminated dollar quote",
			input:   "SELECT * FROM t WHERE body = $$what?",
			wantErr: "unterminated dollar-quote",
		},
		{
			name:    "unterminated tagged dollar quote",
			input:   "SELECT * FROM t WHERE body = $tag$what?",
			wantErr: "unterminated dollar-quote",
		},

		// Block comments: ? inside should be ignored.
		{
			name:  "question mark inside block comment",
			input: "SELECT /* what? */ * FROM t WHERE id = ?",
			want:  "SELECT /* what? */ * FROM t WHERE id = $1",
		},
		{
			name:    "unterminated block comment",
			input:   "SELECT /* oops? FROM t WHERE id = ?",
			wantErr: "unterminated block comment",
		},

		// Line comments: ? inside should be ignored.
		{
			name:  "question mark inside line comment",
			input: "SELECT * -- what?\nFROM t WHERE id = ?",
			want:  "SELECT * -- what?\nFROM t WHERE id = $1",
		},
		{
			name:  "line comment at end of query",
			input: "SELECT * FROM t -- trailing?",
			want:  "SELECT * FROM t -- trailing?",
		},

		// $ that is NOT a dollar-quote or $N.
		{
			name:  "dollar sign not followed by digit or tag",
			input: "SELECT $0 FROM t WHERE id = ?",
			want:  "SELECT $0 FROM t WHERE id = $1",
		},
		{
			name:  "dollar sign at end",
			input: "SELECT * FROM t WHERE cost = ? AND label = $",
			want:  "SELECT * FROM t WHERE cost = $1 AND label = $",
		},

		// Complex real-world queries.
		{
			name:  "insert with returning",
			input: "INSERT INTO users (name, email) VALUES (?, ?) RETURNING id",
			want:  "INSERT INTO users (name, email) VALUES ($1, $2) RETURNING id",
		},
		{
			name:  "update with multiple conditions",
			input: "UPDATE users SET name = ?, age = ? WHERE id = ? AND deleted_at IS NULL",
			want:  "UPDATE users SET name = $1, age = $2 WHERE id = $3 AND deleted_at IS NULL",
		},
		{
			name:  "no question marks just dollar placeholders",
			input: "SELECT * FROM users WHERE id = $1 AND age > $2",
			want:  "SELECT * FROM users WHERE id = $1 AND age > $2",
		},

		// Edge cases.
		{
			name:  "only question marks",
			input: "?",
			want:  "$1",
		},
		{
			name:  "only double question mark",
			input: "??",
			want:  "?",
		},
		{
			name:  "question mark after double question mark",
			input: "?? ?",
			want:  "? $1",
		},
		{
			name:  "multiple double question marks",
			input: "?? ?? ??",
			want:  "? ? ?",
		},
		{
			name:  "triple question mark",
			input: "???",
			want:  "?$1",
		},
		{
			name:  "E not followed by quote",
			input: "SELECT Email FROM t WHERE id = ?",
			want:  "SELECT Email FROM t WHERE id = $1",
		},
		{
			name:  "dollar followed by non-identifier char",
			input: "SELECT $# FROM t WHERE id = ?",
			want:  "SELECT $# FROM t WHERE id = $1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := convertPlaceholders(tt.input)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestParseDollarTag(t *testing.T) {
	tests := []struct {
		name  string
		query string
		want  string
		pos   int
	}{
		{"empty string", "", "", 0},
		{"not dollar", "abc", "", 0},
		{"double dollar", "$$", "$$", 0},
		{"tagged", "$tag$rest", "$tag$", 0},
		{"underscore tag", "$_tag$rest", "$_tag$", 0},
		{"digit after dollar", "$1", "", 0},
		{"tag with digits", "$tag1$rest", "$tag1$", 0},
		{"no closing dollar", "$tag", "", 0},
		{"position in middle", "xx$tag$", "$tag$", 2},
		{"position past end", "abc", "", 5},
		{"non-identifier char in tag", "$tag!$", "", 0},
		{"dollar followed by space", "$ abc", "", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseDollarTag(tt.query, tt.pos)
			assert.Equal(t, tt.want, got)
		})
	}
}

// Benchmark convertPlaceholders to ensure it's efficient.
func BenchmarkConvertPlaceholders_NoPlaceholders(b *testing.B) {
	query := "SELECT * FROM users WHERE id = $1 AND age > $2 ORDER BY name LIMIT 10"
	for b.Loop() {
		_, _ = convertPlaceholders(query)
	}
}

func BenchmarkConvertPlaceholders_WithPlaceholders(b *testing.B) {
	query := "SELECT * FROM users WHERE id = ? AND age > ? AND name = ? ORDER BY ? LIMIT ?"
	for b.Loop() {
		_, _ = convertPlaceholders(query)
	}
}

func BenchmarkConvertPlaceholders_Complex(b *testing.B) {
	query := `SELECT * FROM users WHERE name = 'hello?' AND data ?? 'key' AND id = ? /* comment? */ AND active = ?`
	for b.Loop() {
		_, _ = convertPlaceholders(query)
	}
}
