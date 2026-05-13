// Static schema-vs-code sync check.
//
// This test exists because of the live-bug class that gave WEAKNESSES.md
// its loudest entries: three job tables (add_header_footer_jobs,
// qr_code_jobs, detect_blank_pages_jobs) had complete handlers, services
// and storage repos shipped, but no CREATE TABLE migration. Every call
// 500'd with "relation does not exist" — for months — and no one
// noticed because nothing was exercising those endpoints in CI.
//
// The check is intentionally static (no DB connection):
//
//   - Walk storage/postgres/*.go, pull every raw-string SQL literal,
//     regex-match the table names appearing after INSERT INTO / FROM /
//     JOIN / UPDATE / DELETE FROM.
//   - Walk migrations/postgres/*.up.sql, regex-match every
//     CREATE TABLE [IF NOT EXISTS] target.
//   - Fail if the code-side set has a name the migration-side doesn't.
//
// System tables (pg_*, information_schema) and the golang-migrate
// bookkeeping table are allow-listed; the regex captures bare
// identifiers so dotted refs land on the first segment.
package schema_test

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// Tables we *read* but never *own*. Postgres internals + golang-migrate.
// SQL keywords that follow our verb regex (e.g. `ON CONFLICT … DO UPDATE
// SET …` puts "set" right after "UPDATE") are filtered out here too —
// they're never real tables and pre-filtering keeps the regex simple.
var ignoredTables = map[string]bool{
	"pg_constraint":        true,
	"pg_class":             true,
	"pg_attribute":         true,
	"pg_tables":            true,
	"pg_namespace":         true,
	"pg_indexes":           true,
	"pg_get_constraintdef": true, // function in pg_constraint queries
	"schema_migrations":    true,
	"information_schema":   true,
	// SQL keywords that legitimately follow our verb set.
	"set":    true, // `ON CONFLICT … DO UPDATE SET …`
	"values": true, // `INSERT INTO foo (a,b) VALUES …`
	"select": true, // `INSERT INTO foo SELECT …` (subquery insert)
}

// Capture the table name following the verbs that bind a relation.
// Word boundary on the front avoids matching inside e.g. "INFORMATION_FROM"
// (theoretical). The trailing group stops at the first identifier char.
var sqlTableRef = regexp.MustCompile(
	`(?i)\b(?:INSERT\s+INTO|FROM|JOIN|UPDATE|DELETE\s+FROM)\s+(?:ONLY\s+)?([a-z_][a-z0-9_]*)`,
)

// CREATE TABLE [IF NOT EXISTS] foo  — captures `foo`.
var createTableRef = regexp.MustCompile(
	`(?i)CREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?([a-z_][a-z0-9_]*)`,
)

// CTE alias: `WITH foo AS (`, `, bar AS (`. We pull these out of each
// SQL literal so `FROM foo` against a CTE doesn't false-positive against
// the migration list.
var cteAlias = regexp.MustCompile(`(?i)([a-z_][a-z0-9_]*)\s+AS\s*\(`)

func TestSchemaCodeSync(t *testing.T) {
	root, err := findRepoRoot()
	if err != nil {
		t.Fatalf("find repo root: %v", err)
	}

	codeUses := extractCodeUses(t, filepath.Join(root, "storage", "postgres"))
	migrationDefs := extractMigrationDefs(t, filepath.Join(root, "migrations", "postgres"))

	if len(codeUses) == 0 {
		t.Fatalf("found 0 table references in storage code — extractor is broken")
	}
	if len(migrationDefs) == 0 {
		t.Fatalf("found 0 CREATE TABLE statements in migrations — extractor is broken")
	}

	var missing []string
	for tbl := range codeUses {
		if ignoredTables[tbl] {
			continue
		}
		if !migrationDefs[tbl] {
			missing = append(missing, tbl)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		t.Fatalf(
			"table(s) referenced in storage/postgres but never CREATE'd in migrations/postgres:\n  %s\n\n"+
				"This is the exact failure mode that left 3 endpoints quietly 500'ing in production for months. "+
				"Either add a migration that CREATE TABLEs the missing relation, or remove the storage code that references it.",
			strings.Join(missing, "\n  "),
		)
	}
}

// TestEveryMigrationTableIsUsed flags dead schema — tables that exist in
// migrations but have no storage code touching them. Less critical than
// the reverse direction (won't break prod) but useful for keeping the
// surface area honest. Promoted from t.Log to t.Skip if either set is
// empty.
func TestEveryMigrationTableIsUsed(t *testing.T) {
	root, err := findRepoRoot()
	if err != nil {
		t.Fatalf("find repo root: %v", err)
	}

	codeUses := extractCodeUses(t, filepath.Join(root, "storage", "postgres"))
	migrationDefs := extractMigrationDefs(t, filepath.Join(root, "migrations", "postgres"))

	// Tables defined in migrations but read nowhere in storage code.
	// Empty in a healthy repo — historical dead schemas (add_background_jobs,
	// translate_jobs) were dropped in migration 0016.
	var orphan []string
	for tbl := range migrationDefs {
		if !codeUses[tbl] {
			orphan = append(orphan, tbl)
		}
	}
	if len(orphan) > 0 {
		sort.Strings(orphan)
		t.Logf("migration table(s) not referenced by storage code (dead schema?):\n  %s",
			strings.Join(orphan, "\n  "))
	}
}

// --- internals -----------------------------------------------------------

func extractCodeUses(t *testing.T, dir string) map[string]bool {
	t.Helper()
	out := map[string]bool{}
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		// SQL lives in raw-string (backtick) literals in this codebase —
		// scanning whole file would false-positive on FROM in comments
		// and error messages.
		for _, lit := range rawStringLiterals(string(b)) {
			localCTE := map[string]bool{}
			for _, m := range cteAlias.FindAllStringSubmatch(lit, -1) {
				localCTE[strings.ToLower(m[1])] = true
			}
			for _, m := range sqlTableRef.FindAllStringSubmatch(lit, -1) {
				name := strings.ToLower(m[1])
				if localCTE[name] {
					continue
				}
				out[name] = true
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", dir, err)
	}
	return out
}

func extractMigrationDefs(t *testing.T, dir string) map[string]bool {
	t.Helper()
	out := map[string]bool{}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".up.sql") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatalf("read %s: %v", e.Name(), err)
		}
		// Strip line comments so we don't catch tables mentioned in
		// "-- this used to live in 0001" style notes.
		clean := stripSQLComments(string(b))
		for _, m := range createTableRef.FindAllStringSubmatch(clean, -1) {
			out[strings.ToLower(m[1])] = true
		}
	}
	return out
}

// rawStringLiterals scans Go source for `…` substrings (raw string literals).
// Naïve — doesn't try to skip rune literals or interpreted strings — but
// good enough for this repo, where SQL is consistently backtick'd.
func rawStringLiterals(src string) []string {
	var out []string
	for {
		i := strings.IndexByte(src, '`')
		if i < 0 {
			break
		}
		src = src[i+1:]
		j := strings.IndexByte(src, '`')
		if j < 0 {
			break
		}
		out = append(out, src[:j])
		src = src[j+1:]
	}
	return out
}

// stripSQLComments removes `-- …` line comments from SQL. Block comments
// (`/* … */`) are rare in our migrations and left alone.
func stripSQLComments(s string) string {
	var b strings.Builder
	for _, line := range strings.Split(s, "\n") {
		if i := strings.Index(line, "--"); i >= 0 {
			line = line[:i]
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
	return b.String()
}

func findRepoRoot() (string, error) {
	d, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(d, "go.mod")); err == nil {
			return d, nil
		}
		parent := filepath.Dir(d)
		if parent == d {
			return "", fmt.Errorf("go.mod not found walking up from cwd")
		}
		d = parent
	}
}
