package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ErrRateLimited is returned when Gemini's free-tier quota is exhausted
// (HTTP 429). Handlers should map it to a user-friendly "please wait"
// message rather than a generic 500.
var ErrRateLimited = errors.New("llm: rate limit exceeded")

// geminiBaseURL is the v1beta endpoint that supports streamGenerateContent
// (SSE) and batchEmbedContents. The v1 endpoint exists but only the v1beta
// one exposes the streaming variant we need.
const geminiBaseURL = "https://generativelanguage.googleapis.com/v1beta"

// geminiEmbedDim is what we *ask* gemini-embedding-001 to produce via the
// `outputDimensionality` field. The model's native size is 3072, but
// pdf_chunks.embedding is vector(768) — keeping the smaller dim saves ~4x
// storage and shrinks the HNSW index without much retrieval quality loss
// per Google's documentation. If this changes, the migration must move
// too.
const geminiEmbedDim = 768

// geminiProvider talks to Google's Generative Language API for both chat
// and embeddings. It implements Provider.
type geminiProvider struct {
	apiKey     string
	chatModel  string // e.g. "gemini-2.5-flash"
	embedModel string // e.g. "text-embedding-004"
	http       *http.Client
}

// NewGemini constructs a Gemini-backed Provider. apiKey is required;
// chatModel and embedModel fall back to sensible free-tier defaults if
// empty. We keep the http.Client local rather than sharing the package
// default so individual instances can be configured for tests.
func NewGemini(apiKey, chatModel, embedModel string) Provider {
	if chatModel == "" {
		chatModel = "gemini-2.5-flash"
	}
	if embedModel == "" {
		// gemini-embedding-001 replaced text-embedding-004 on v1beta and
		// is the only family currently accepting embedContent on the
		// free tier.
		embedModel = "gemini-embedding-001"
	}
	return &geminiProvider{
		apiKey:     apiKey,
		chatModel:  chatModel,
		embedModel: embedModel,
		http: &http.Client{
			// 5 min per request matches Gemini's stream lifetime and
			// covers slow embed batches over flaky networks.
			Timeout: 5 * time.Minute,
		},
	}
}

func (g *geminiProvider) Name() string { return "gemini" }
func (g *geminiProvider) EmbedDim() int { return geminiEmbedDim }

// ─── Chat (streaming) ───────────────────────────────────────────────────

// Gemini's streamGenerateContent endpoint is annoyingly *not* SSE — it
// returns a JSON array streamed one element per line (with a leading `[`
// and trailing `]`). We read the body line-by-line and extract each
// element as it lands.

type geminiInlineData struct {
	MIMEType string `json:"mimeType"`
	Data     string `json:"data"` // base64
}

type geminiPart struct {
	Text       string            `json:"text,omitempty"`
	InlineData *geminiInlineData `json:"inlineData,omitempty"`
}

type geminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []geminiPart `json:"parts"`
}

type geminiGenConfig struct {
	Temperature     *float32 `json:"temperature,omitempty"`
	MaxOutputTokens int      `json:"maxOutputTokens,omitempty"`
}

type geminiChatReq struct {
	Contents          []geminiContent  `json:"contents"`
	SystemInstruction *geminiContent   `json:"systemInstruction,omitempty"`
	GenerationConfig  *geminiGenConfig `json:"generationConfig,omitempty"`
}

type geminiUsageMeta struct {
	PromptTokenCount     int `json:"promptTokenCount"`
	CandidatesTokenCount int `json:"candidatesTokenCount"`
}

type geminiCandidate struct {
	Content      geminiContent `json:"content"`
	FinishReason string        `json:"finishReason"`
}

type geminiChatResp struct {
	Candidates    []geminiCandidate `json:"candidates"`
	UsageMetadata *geminiUsageMeta  `json:"usageMetadata,omitempty"`
}

// Chat fans the streamed JSON elements into a ChatChunk channel. On any
// transport error we send a final chunk with Err set and close the
// channel; on success the final chunk carries FinishReason + token counts.
func (g *geminiProvider) Chat(ctx context.Context, msgs []Message, opts ChatOpts) (<-chan ChatChunk, error) {
	model := opts.Model
	if model == "" {
		model = g.chatModel
	}

	body, err := json.Marshal(g.buildChatReq(msgs, opts))
	if err != nil {
		return nil, fmt.Errorf("gemini: marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/models/%s:streamGenerateContent?alt=sse&key=%s",
		geminiBaseURL, model, g.apiKey)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("gemini: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")

	resp, err := g.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gemini: do request: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		errBody, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		// 429 is the most user-actionable failure — surface it via a
		// sentinel so handlers can render "please wait" rather than
		// "Internal Server Error".
		if resp.StatusCode == http.StatusTooManyRequests {
			return nil, fmt.Errorf("%w: %s", ErrRateLimited, strings.TrimSpace(string(errBody)))
		}
		return nil, fmt.Errorf("gemini: chat HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(errBody)))
	}

	out := make(chan ChatChunk, 8)
	go g.consumeStream(resp, out)
	return out, nil
}

func (g *geminiProvider) buildChatReq(msgs []Message, opts ChatOpts) geminiChatReq {
	req := geminiChatReq{
		Contents: make([]geminiContent, 0, len(msgs)),
	}

	// Gemini wants system as a separate `systemInstruction` field.
	if opts.System != "" {
		req.SystemInstruction = &geminiContent{Parts: []geminiPart{{Text: opts.System}}}
	}

	for _, m := range msgs {
		role := "user"
		if m.Role == RoleAssistant {
			role = "model" // Gemini's name for assistant
		}
		// System messages embedded in the array are appended to the
		// system instruction so we don't silently drop them.
		if m.Role == RoleSystem {
			if req.SystemInstruction == nil {
				req.SystemInstruction = &geminiContent{Parts: []geminiPart{{Text: m.Content}}}
			} else {
				req.SystemInstruction.Parts = append(req.SystemInstruction.Parts, geminiPart{Text: m.Content})
			}
			continue
		}

		// Build parts: images first, then text. Gemini's docs recommend
		// this order — vision tokens are processed alongside the text,
		// putting them first gives the model the visual context before
		// the prompt.
		parts := make([]geminiPart, 0, len(m.Attachments)+1)
		for _, att := range m.Attachments {
			if len(att.Data) == 0 || att.MIMEType == "" {
				continue
			}
			parts = append(parts, geminiPart{
				InlineData: &geminiInlineData{
					MIMEType: att.MIMEType,
					Data:     base64.StdEncoding.EncodeToString(att.Data),
				},
			})
		}
		if m.Content != "" {
			parts = append(parts, geminiPart{Text: m.Content})
		}
		if len(parts) == 0 {
			continue // skip empty turns
		}
		req.Contents = append(req.Contents, geminiContent{
			Role:  role,
			Parts: parts,
		})
	}

	gen := &geminiGenConfig{}
	if opts.Temperature > 0 {
		t := opts.Temperature
		gen.Temperature = &t
	}
	if opts.MaxTokens > 0 {
		gen.MaxOutputTokens = opts.MaxTokens
	}
	if gen.Temperature != nil || gen.MaxOutputTokens > 0 {
		req.GenerationConfig = gen
	}

	return req
}

// consumeStream reads the SSE-framed response (alt=sse mode) and pushes
// each delta into out. SSE frames look like:
//
//	data: {"candidates":[{"content":{"parts":[{"text":"Hello"}],...}}],...}
//	data: {"candidates":[...,"finishReason":"STOP"],"usageMetadata":{...}}
//
// We close `out` when the body ends or on error.
func (g *geminiProvider) consumeStream(resp *http.Response, out chan<- ChatChunk) {
	defer resp.Body.Close()
	defer close(out)

	scanner := bufio.NewScanner(resp.Body)
	// Some Gemini responses include long single-line frames — bump the
	// buffer well above the default 64KiB.
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)

	var (
		tokensIn, tokensOut int
		finish              string
	)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 || !bytes.HasPrefix(line, []byte("data:")) {
			continue
		}
		payload := bytes.TrimSpace(line[len("data:"):])
		if len(payload) == 0 {
			continue
		}

		var chunk geminiChatResp
		if err := json.Unmarshal(payload, &chunk); err != nil {
			out <- ChatChunk{Err: fmt.Errorf("gemini: decode stream chunk: %w", err)}
			return
		}

		for _, cand := range chunk.Candidates {
			for _, p := range cand.Content.Parts {
				if p.Text != "" {
					out <- ChatChunk{Delta: p.Text}
				}
			}
			if cand.FinishReason != "" {
				finish = cand.FinishReason
			}
		}
		if chunk.UsageMetadata != nil {
			tokensIn = chunk.UsageMetadata.PromptTokenCount
			tokensOut = chunk.UsageMetadata.CandidatesTokenCount
		}
	}
	if err := scanner.Err(); err != nil {
		out <- ChatChunk{Err: fmt.Errorf("gemini: stream read: %w", err)}
		return
	}

	out <- ChatChunk{
		FinishReason: normalizeFinish(finish),
		TokensIn:     tokensIn,
		TokensOut:    tokensOut,
	}
}

func normalizeFinish(g string) string {
	switch g {
	case "STOP":
		return "stop"
	case "MAX_TOKENS":
		return "length"
	case "":
		return ""
	default:
		return strings.ToLower(g)
	}
}

// ─── Embedding ──────────────────────────────────────────────────────────

// gemini-embedding-001 only exposes the single-content `embedContent`
// endpoint on free tier — there's no synchronous batch endpoint we can
// use without going through the async job API. We parallelise instead
// with a small worker pool so a 1500-chunk PDF doesn't take 25 minutes.
const geminiEmbedConcurrency = 8

type geminiEmbedReq struct {
	Content              geminiContent `json:"content"`
	OutputDimensionality int           `json:"outputDimensionality,omitempty"`
}

type geminiEmbedResp struct {
	Embedding struct {
		Values []float32 `json:"values"`
	} `json:"embedding"`
}

// Embed turns each text into a 768-dim vector via parallel calls to
// embedContent. Order of the returned slice matches the input. Cancels
// early if any single request fails.
func (g *geminiProvider) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	out := make([][]float32, len(texts))

	type job struct{ i int }
	type result struct {
		i   int
		err error
	}

	jobs := make(chan job)
	results := make(chan result, len(texts))

	wctx, cancel := context.WithCancel(ctx)
	defer cancel()

	for w := 0; w < geminiEmbedConcurrency; w++ {
		go func() {
			for j := range jobs {
				v, err := g.embedOne(wctx, texts[j.i])
				if err == nil {
					out[j.i] = v
				}
				results <- result{i: j.i, err: err}
			}
		}()
	}

	go func() {
		defer close(jobs)
		for i := range texts {
			select {
			case <-wctx.Done():
				return
			case jobs <- job{i: i}:
			}
		}
	}()

	for done := 0; done < len(texts); done++ {
		r := <-results
		if r.err != nil {
			cancel() // tell other workers to bail
			return nil, r.err
		}
	}
	return out, nil
}

func (g *geminiProvider) embedOne(ctx context.Context, text string) ([]float32, error) {
	body, err := json.Marshal(geminiEmbedReq{
		Content:              geminiContent{Parts: []geminiPart{{Text: text}}},
		OutputDimensionality: geminiEmbedDim,
	})
	if err != nil {
		return nil, fmt.Errorf("gemini: marshal embed: %w", err)
	}

	url := fmt.Sprintf("%s/models/%s:embedContent?key=%s", geminiBaseURL, g.embedModel, g.apiKey)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("gemini: build embed: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := g.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gemini: embed do: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		errBody, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		if resp.StatusCode == http.StatusTooManyRequests {
			return nil, fmt.Errorf("%w: %s", ErrRateLimited, strings.TrimSpace(string(errBody)))
		}
		return nil, fmt.Errorf("gemini: embed HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(errBody)))
	}

	var decoded geminiEmbedResp
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return nil, fmt.Errorf("gemini: decode embed: %w", err)
	}
	if len(decoded.Embedding.Values) == 0 {
		return nil, errors.New("gemini: empty embedding")
	}
	return decoded.Embedding.Values, nil
}
