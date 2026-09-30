package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Groq speaks the OpenAI chat-completions shape, so this is a thin
// client over one endpoint rather than a full SDK.
//
// It exists alongside Gemini rather than replacing it: Groq has no
// embedding endpoint, and pdf_chunks — the whole PDF chat — is built on
// vectors. Embed here returns ErrUnsupported, and the registry keeps
// Gemini as the default provider for that reason.
//
// What Groq is for is routing: deciding which tool a sentence means.
// That wants to come back in well under a second, and Gemini's free
// tier allows 20 requests per key per day, which a command bar would
// exhaust before lunch.
const (
	groqEndpoint     = "https://api.groq.com/openai/v1/chat/completions"
	groqDefaultModel = "llama-3.3-70b-versatile"
)

type groqProvider struct {
	key   string
	model string
	http  *http.Client
}

// NewGroq returns a chat-only provider. model may be empty, in which
// case a current general-purpose model is used.
func NewGroq(key, model string) Provider {
	if strings.TrimSpace(model) == "" {
		model = groqDefaultModel
	}
	return &groqProvider{
		key:   key,
		model: model,
		// Routing is interactive: a slow answer is worse than none, and
		// the caller has a fallback path.
		http: &http.Client{Timeout: 20 * time.Second},
	}
}

func (g *groqProvider) Name() string { return "groq" }

func (g *groqProvider) Embed(context.Context, []string) ([][]float32, error) {
	return nil, ErrUnsupported
}

func (g *groqProvider) EmbedDim() int { return 0 }

type groqMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type groqRequest struct {
	Model       string        `json:"model"`
	Messages    []groqMessage `json:"messages"`
	Temperature float32       `json:"temperature,omitempty"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
	// Groq honours OpenAI's json_object mode, which is what keeps the
	// router's answer parseable instead of wrapped in prose.
	ResponseFormat *groqFormat `json:"response_format,omitempty"`
}

type groqFormat struct {
	Type string `json:"type"`
}

type groqResponse struct {
	Choices []struct {
		Message      groqMessage `json:"message"`
		FinishReason string      `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// Chat returns the whole completion as a single chunk. Groq supports
// streaming, but every caller here wants one short structured answer,
// and a one-chunk channel keeps them off the streaming path.
func (g *groqProvider) Chat(ctx context.Context, msgs []Message, opts ChatOpts) (<-chan ChatChunk, error) {
	body := groqRequest{
		Model:       firstNonEmpty(opts.Model, g.model),
		Temperature: opts.Temperature,
		MaxTokens:   opts.MaxTokens,
	}
	if opts.JSONMode {
		body.ResponseFormat = &groqFormat{Type: "json_object"}
	}
	if opts.System != "" {
		body.Messages = append(body.Messages, groqMessage{Role: "system", Content: opts.System})
	}
	for _, m := range msgs {
		// Attachments are dropped: this provider is used for text
		// routing, and silently re-encoding files would cost tokens
		// nobody asked to spend.
		body.Messages = append(body.Messages, groqMessage{
			Role:    string(m.Role),
			Content: m.Content,
		})
	}

	raw, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("groq: encode request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, groqEndpoint, bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("groq: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+g.key)
	req.Header.Set("Content-Type", "application/json")

	res, err := g.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("groq: request: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode == http.StatusTooManyRequests {
		return nil, ErrRateLimited
	}

	var parsed groqResponse
	if err := json.NewDecoder(res.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("groq: decode response (status %d): %w", res.StatusCode, err)
	}
	if parsed.Error != nil {
		return nil, fmt.Errorf("groq: %s", parsed.Error.Message)
	}
	if len(parsed.Choices) == 0 {
		return nil, fmt.Errorf("groq: no choices in response (status %d)", res.StatusCode)
	}

	out := make(chan ChatChunk, 1)
	out <- ChatChunk{
		Delta:        parsed.Choices[0].Message.Content,
		FinishReason: parsed.Choices[0].FinishReason,
		TokensIn:     parsed.Usage.PromptTokens,
		TokensOut:    parsed.Usage.CompletionTokens,
	}
	close(out)
	return out, nil
}

func firstNonEmpty(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return a
	}
	return b
}
