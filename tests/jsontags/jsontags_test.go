// Static audit: every exported struct field in api/models/ must carry a
// json tag.
//
// Why this exists: MergeJob shipped for months with `db:"id"` style tags
// only — no `json:` — so its responses came out as
// `{"ID": "...", "Status": "..."}` (Go's default PascalCase) while every
// other endpoint emitted snake_case. The integration suite caught it
// once it became a hard assertion; this test would have caught it on
// commit.
//
// Rules
//   - Field is exported (starts upper-case) AND
//   - Field does not have `json:"-"` (explicit skip) →
//     a `json:"..."` tag must be present.
//   - Embedded promoted fields are skipped (they inherit from the embed).
//   - Function / channel / interface field types are skipped (the
//     handful of those we have aren't serialized).
package jsontags_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// Files we don't care about. Add here only if a struct legitimately
// can't carry json tags (none today).
var skipFiles = map[string]bool{
	// e.g., "internal_only.go": true,
}

func TestJSONTagsRequired(t *testing.T) {
	root, err := findRepoRoot()
	if err != nil {
		t.Fatalf("find repo root: %v", err)
	}
	dir := filepath.Join(root, "api", "models")

	var violations []string
	err = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		if skipFiles[filepath.Base(path)] {
			return nil
		}
		violations = append(violations, scanFile(t, path)...)
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", dir, err)
	}

	if len(violations) > 0 {
		sort.Strings(violations)
		t.Fatalf("exported struct field(s) in api/models/ missing a `json:` tag:\n  %s\n\n"+
			"Without json:, encoding/json falls back to the Go field name (PascalCase). "+
			"Frontend clients expect snake_case — drift here is the same class of bug we hit on MergeJob.",
			strings.Join(violations, "\n  "))
	}
}

func scanFile(t *testing.T, path string) []string {
	t.Helper()
	fset := token.NewFileSet()
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	f, err := parser.ParseFile(fset, path, src, parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}

	rel, _ := filepath.Rel(filepath.Dir(filepath.Dir(filepath.Dir(path))), path)
	var out []string
	ast.Inspect(f, func(n ast.Node) bool {
		ts, ok := n.(*ast.TypeSpec)
		if !ok {
			return true
		}
		st, ok := ts.Type.(*ast.StructType)
		if !ok || st.Fields == nil {
			return true
		}
		for _, field := range st.Fields.List {
			// Embedded fields (no Names) are inherited from the embed
			// — they can't carry a tag themselves.
			if len(field.Names) == 0 {
				continue
			}
			if !isSerializableType(field.Type) {
				continue
			}
			for _, name := range field.Names {
				if !name.IsExported() {
					continue
				}
				tag := tagFor(field, "json")
				if tag == "-" {
					continue // explicit skip
				}
				if tag == "" {
					out = append(out, formatViolation(rel, ts.Name.Name, name.Name))
				}
			}
		}
		return true
	})
	return out
}

// tagFor returns the value of a struct tag key, or "" if absent.
func tagFor(field *ast.Field, key string) string {
	if field.Tag == nil {
		return ""
	}
	raw := strings.Trim(field.Tag.Value, "`")
	t := reflect.StructTag(raw).Get(key)
	// Strip the ",omitempty" / ",string" suffixes — we only care that
	// a name is set (even an empty name like `json:",omitempty"` is
	// still missing the key part and counts as a violation).
	if comma := strings.IndexByte(t, ','); comma >= 0 {
		t = t[:comma]
	}
	if t == "" && reflect.StructTag(raw).Get(key) == "-" {
		return "-"
	}
	return t
}

// isSerializableType excludes types that encoding/json silently drops
// (chan, func, complex). Interfaces are kept — they can hold anything,
// including JSON-friendly values.
func isSerializableType(expr ast.Expr) bool {
	switch t := expr.(type) {
	case *ast.ChanType, *ast.FuncType:
		return false
	case *ast.Ident:
		switch t.Name {
		case "complex64", "complex128":
			return false
		}
	}
	return true
}

func formatViolation(file, structName, fieldName string) string {
	return file + ":" + structName + "." + fieldName
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
