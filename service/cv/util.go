package cv

import (
	"context"
	"strings"
	"time"
)

// contextBackgroundWithTimeout is a small helper so the goroutines we
// spawn for embedding don't leak — 30s is way more than the API needs
// even on the slowest free-tier round trip.
func contextBackgroundWithTimeout() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 30*time.Second)
}

// flattenSectionText turns a section's content map into a plain
// concatenated string suitable for embedding. We walk the map in a
// stable order so the same input always produces the same embedding.
func flattenSectionText(content map[string]any) string {
	if content == nil {
		return ""
	}
	var sb strings.Builder
	flattenInto(&sb, content)
	return strings.TrimSpace(sb.String())
}

func flattenInto(sb *strings.Builder, v any) {
	switch t := v.(type) {
	case string:
		if t = strings.TrimSpace(t); t != "" {
			sb.WriteString(t)
			sb.WriteByte(' ')
		}
	case []any:
		for _, item := range t {
			flattenInto(sb, item)
		}
	case map[string]any:
		// Deterministic order — sort by key.
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		// We don't import sort here just to avoid the alloc; a small
		// insertion sort is fine for the 5–15 keys per section.
		for i := 1; i < len(keys); i++ {
			for j := i; j > 0 && keys[j-1] > keys[j]; j-- {
				keys[j-1], keys[j] = keys[j], keys[j-1]
			}
		}
		for _, k := range keys {
			flattenInto(sb, t[k])
		}
	}
}

// extractActionVerbs is used by ATS scoring to flag bullet points that
// don't start with an action verb. Sub-set covers the most common
// English resume action verbs; we don't need every possible verb to
// catch the "I worked at X" anti-pattern.
var commonActionVerbs = map[string]struct{}{
	"led": {}, "built": {}, "shipped": {}, "owned": {}, "managed": {},
	"designed": {}, "implemented": {}, "developed": {}, "delivered": {},
	"drove": {}, "launched": {}, "scaled": {}, "improved": {}, "reduced": {},
	"increased": {}, "created": {}, "architected": {}, "established": {},
	"mentored": {}, "coached": {}, "presented": {}, "negotiated": {},
	"analysed": {}, "analyzed": {}, "optimised": {}, "optimized": {},
	"automated": {}, "migrated": {}, "deployed": {}, "trained": {},
}

func startsWithActionVerb(bullet string) bool {
	w := strings.Fields(strings.ToLower(strings.TrimSpace(bullet)))
	if len(w) == 0 {
		return false
	}
	_, ok := commonActionVerbs[w[0]]
	return ok
}

// containsNumber is a quick "has a quantifiable metric" heuristic used
// by the content-quality score. A bullet like "Increased revenue 20%"
// passes; "Worked on revenue stuff" doesn't.
func containsNumber(s string) bool {
	for _, r := range s {
		if r >= '0' && r <= '9' {
			return true
		}
	}
	return false
}

// wordCount is approximate — counts whitespace-separated tokens.
func wordCount(s string) int {
	return len(strings.Fields(s))
}
