package llm

import (
	"fmt"
	"os"
	"strings"
)

// FromEnv returns a Provider based on environment variables. Defaults to
// Gemini if LLM_PROVIDER is unset — that's what the free tier launch uses.
//
// Required env per provider:
//   - gemini: GEMINI_API_KEY (plus optional GEMINI_CHAT_MODEL,
//     GEMINI_EMBED_MODEL)
//   - groq:   GROQ_API_KEY (plus optional GROQ_CHAT_MODEL)
//
// Gemini stays the default because it embeds; Groq has no embedding
// endpoint, and pdf_chunks is built on vectors. Callers that only need
// chat can ask for Groq explicitly with Chatter().
//
// Adding a new provider here is the *only* place that needs to learn its
// name; everything downstream (handlers, services, indexer) speaks to the
// returned Provider interface.
func FromEnv() (Provider, error) {
	name := strings.ToLower(strings.TrimSpace(os.Getenv("LLM_PROVIDER")))
	if name == "" {
		name = "gemini"
	}

	switch name {
	case "gemini":
		// Multi-key support: GEMINI_API_KEYS (comma-separated) wins; a
		// single GEMINI_API_KEY is appended for backwards compatibility.
		// Each free-tier key is capped at 20 requests/day per model, so
		// rotating across N keys lifts the daily ceiling by Nx without
		// any billing relationship.
		var keys []string
		if multi := strings.TrimSpace(os.Getenv("GEMINI_API_KEYS")); multi != "" {
			for _, k := range strings.Split(multi, ",") {
				if k = strings.TrimSpace(k); k != "" {
					keys = append(keys, k)
				}
			}
		}
		if k := strings.TrimSpace(os.Getenv("GEMINI_API_KEY")); k != "" {
			// Avoid duplicating it if the operator put the same key in both.
			seen := false
			for _, existing := range keys {
				if existing == k {
					seen = true
					break
				}
			}
			if !seen {
				keys = append(keys, k)
			}
		}
		if len(keys) == 0 {
			return nil, fmt.Errorf("llm: GEMINI_API_KEYS or GEMINI_API_KEY required for provider=gemini")
		}
		return NewGeminiMulti(
			keys,
			os.Getenv("GEMINI_CHAT_MODEL"),
			os.Getenv("GEMINI_EMBED_MODEL"),
		), nil
	case "groq":
		key := strings.TrimSpace(os.Getenv("GROQ_API_KEY"))
		if key == "" {
			return nil, fmt.Errorf("llm: GROQ_API_KEY required for provider=groq")
		}
		return NewGroq(key, os.Getenv("GROQ_CHAT_MODEL")), nil
	default:
		return nil, fmt.Errorf("llm: unknown provider %q", name)
	}
}

// Chatter returns the provider best suited to short, interactive
// completions — routing a sentence to a tool, say.
//
// Groq when its key is set: the answer comes back in well under a
// second, and its limits are per-minute. Gemini's free tier allows 20
// requests per key per day, which is fine for a document chat and not
// for anything a visitor types into a box. Falls back to FromEnv when
// GROQ_API_KEY is absent, so nothing needs Groq to work.
func Chatter() (Provider, error) {
	if key := strings.TrimSpace(os.Getenv("GROQ_API_KEY")); key != "" {
		return NewGroq(key, os.Getenv("GROQ_CHAT_MODEL")), nil
	}
	return FromEnv()
}
