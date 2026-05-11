// Static guard against pgx-v5 "OID 25: cannot find encode plan" failures.
//
// The bug class: when SQL concatenates a bind parameter with a string
// literal via `||`, postgres needs both sides to be `text`. pgx-v5 won't
// auto-encode an int into the text-concat context — it errors out as
// "unable to encode N into text format for text (OID 25)". The fix is
// always the same: cast the parameter explicitly (`$1::int` for the
// usual interval arithmetic). Caught + fixed three times in one
// session before this guard existed.
//
// Rule: any `$N` followed (with optional whitespace) by `||` MUST be
// preceded by `::` (a cast). Plain `$N || ...` is the bug.
//
// Allowed shape:    `$1::int || ' minutes'`, `'%' || $1 || '%'`
// Rejected shape:   `$1 || ' minutes'`,      `$1 ||' x'`
package sqlcasts_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// We narrowly look at parameters that appear INSIDE a typed cast like
// `(…)::INTERVAL` or `(…)::DATE`. That's the context where pgx-v5
// can't infer the encoding for a bare int parameter — the actual
// failure mode. `LIKE '%' || $1 || '%'` is fine because the param is
// a Go string and the surrounding context is plain text.
//
// Capture group: the body inside the parentheses preceding `::TYPE`.
var typedCast = regexp.MustCompile(`(?i)\(([^()]*)\)::(?:interval|date|timestamp|timestamptz|time|timetz|numeric|decimal|int|integer|bigint|smallint|float|real|double precision)\b`)

// Match every `$N` in the captured body; we check the tail manually
// for an immediate `::TYPE` cast (Go's regexp has no lookahead).
var anyParam = regexp.MustCompile(`\$\d+`)

func TestParamsCastBeforeStringConcat(t *testing.T) {
	root, err := findRepoRoot()
	if err != nil {
		t.Fatalf("find repo root: %v", err)
	}
	dir := filepath.Join(root, "storage", "postgres")

	var bad []string
	err = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
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
		// Pull only the raw string literals — SQL lives in `...` blocks
		// throughout this repo. Scanning the whole file would
		// false-positive on comments mentioning `$1 || …`.
		for _, lit := range rawStrings(string(b)) {
			for _, m := range typedCast.FindAllStringSubmatchIndex(lit, -1) {
				body := lit[m[2]:m[3]]
				if uncastParamInside(body) {
					rel, _ := filepath.Rel(root, path)
					bad = append(bad, rel+":  …("+strings.TrimSpace(body)+")::…")
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", dir, err)
	}

	if len(bad) > 0 {
		sort.Strings(bad)
		t.Fatalf("uncast parameter in SQL `||` concat — will trigger pgx OID-25:\n  %s\n\n"+
			"Cast the param explicitly: `$N::int || ' minutes'` or `$N::text || …`.",
			strings.Join(bad, "\n  "))
	}
}

// uncastParamInside reports whether `body` contains a `$N` that isn't
// followed by `::TYPE`. Linear scan: cheap, no lookahead needed.
func uncastParamInside(body string) bool {
	for _, m := range anyParam.FindAllStringIndex(body, -1) {
		end := m[1] // position right after the digits
		if end < len(body) && body[end] == ':' && end+1 < len(body) && body[end+1] == ':' {
			continue // has cast
		}
		return true
	}
	return false
}

func rawStrings(src string) []string {
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
			return "", os.ErrNotExist
		}
		d = parent
	}
}
