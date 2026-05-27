// Package llm wraps the LLM and embedding providers behind a single
// interface so /chat (and any future feature that wants to call an LLM) can
// switch providers via env without touching call sites.
package llm

import (
	"context"
	"errors"
)

// Role names match the OpenAI/Anthropic convention so we don't have to
// translate every time we shuttle messages around.
type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

// Message is one turn in a chat conversation. Attachments piggyback on
// the same turn — Gemini accepts both an image part and a text part in
// one user message, so the API surface stays small.
type Message struct {
	Role        Role
	Content     string
	Attachments []Attachment
}

// Attachment is an inline binary the user wants the model to see. Right
// now only images are supported (Gemini multimodal); audio/video could
// land here too once we wire them up.
type Attachment struct {
	MIMEType string // "image/jpeg" | "image/png" | "image/webp" | ...
	Data     []byte // raw bytes — the provider encodes for the wire
}

// ChatOpts tunes a single Chat call. All fields are optional; zero values
// fall back to provider defaults.
type ChatOpts struct {
	// Model lets a caller override the provider's default (e.g. ask Gemini
	// for "gemini-2.5-pro" instead of the configured "gemini-2.5-flash").
	Model string

	// Temperature is the usual 0.0–2.0 randomness knob. Anything ≤0 is
	// treated as "use provider default" so the zero value does the right
	// thing.
	Temperature float32

	// MaxTokens caps the response length. 0 = provider default.
	MaxTokens int

	// System lets the caller supply a system prompt as a separate field
	// instead of stuffing it into Messages — Gemini wants this in a
	// dedicated `systemInstruction` slot, so plumbing it through the
	// interface keeps the call sites tidy.
	System string
}

// ChatChunk is one piece of a streaming Chat response. Delta is the
// incremental text since the previous chunk; FinishReason and the token
// counts are populated on the *final* chunk only.
//
// If Err is non-nil, the stream has failed — consumers should stop reading
// after seeing a chunk with Err.
type ChatChunk struct {
	Delta        string
	FinishReason string // "stop" | "length" | "" (still streaming)
	TokensIn     int
	TokensOut    int
	Err          error
}

// Provider is the interface every LLM backend implements. Chat MUST close
// the returned channel when streaming finishes (success or error) so
// callers can range over it safely.
type Provider interface {
	Name() string

	// Chat streams a completion. The returned channel is closed when the
	// stream ends. If the underlying API call fails before any chunk is
	// produced, Chat returns an error and the channel is never created.
	Chat(ctx context.Context, msgs []Message, opts ChatOpts) (<-chan ChatChunk, error)

	// Embed turns a batch of texts into vectors. Returns one vector per
	// input, in the same order. Provider implementations should batch
	// internally if the upstream API requires multiple calls.
	//
	// The vector dimension is provider-specific (Gemini text-embedding-004
	// returns 768-dim) and must match the `vector(N)` column in
	// pdf_chunks. Migrations need to be updated if the embedding provider
	// switches.
	Embed(ctx context.Context, texts []string) ([][]float32, error)

	// EmbedDim returns the dimension of vectors this provider's Embed
	// produces — useful for sanity-checking the DB schema at startup.
	EmbedDim() int
}

// ErrUnsupported is returned by providers that don't implement a capability
// (e.g. an LLM-only provider being asked to Embed).
var ErrUnsupported = errors.New("llm: operation not supported by this provider")
