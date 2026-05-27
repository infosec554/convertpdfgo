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
		key := os.Getenv("GEMINI_API_KEY")
		if key == "" {
			return nil, fmt.Errorf("llm: GEMINI_API_KEY required for provider=gemini")
		}
		return NewGemini(
			key,
			os.Getenv("GEMINI_CHAT_MODEL"),
			os.Getenv("GEMINI_EMBED_MODEL"),
		), nil
	default:
		return nil, fmt.Errorf("llm: unknown provider %q", name)
	}
}
