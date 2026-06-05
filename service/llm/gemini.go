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
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
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
//
// Multi-key rotation: the free tier caps each key at 20 requests/day.
// To get past that without a billing relationship, we round-robin
// across a pool of keys and, when a key returns 429
// RESOURCE_EXHAUSTED, automatically retry on the next key. The pool
// is set up at construction; live key health (which one is currently
// 429'd) is tracked in the cooldown map below.
type geminiProvider struct {
	apiKeys     []string
	cursor      atomic.Uint64 // monotonic — % len(apiKeys) on each pick
	cooldown    sync.Map      // key (string) → time.Time when it's allowed again

	// chatModels is the fallback pool — the first entry is the default
	// model; later entries are tried in order when the earlier one
	// returns 429 within the same request. parkedModels tracks which
	// (key, model) pairs are still cooling down.
	chatModels   []string
	embedModel   string        // e.g. "text-embedding-004"
	parkedModels sync.Map      // "<key>|<model>" → time.Time when allowed again
	http         *http.Client
}

// NewGemini constructs a Gemini-backed Provider with a single key —
// kept for backwards compatibility with callers that pre-date multi-key.
func NewGemini(apiKey, chatModel, embedModel string) Provider {
	return NewGeminiMulti([]string{apiKey}, chatModel, embedModel)
}

// NewGeminiMulti is the multi-key constructor. Provide one or more keys;
// the provider picks the next live key on each call and falls through
// to the next one on 429.
//
// chatModel can be a comma-separated list — the provider treats each
// entry as a fallback model. The PerDayPerProjectPerModel quota means
// every (key, model) pair has its own 20/day ceiling, so listing
// "gemini-2.5-flash-lite,gemini-2.5-flash,gemini-2.0-flash" turns 1 key
// into 60 requests/day before adding more keys.
func NewGeminiMulti(apiKeys []string, chatModel, embedModel string) Provider {
	// Filter out empties so a stray comma in the env doesn't waste a slot.
	keys := make([]string, 0, len(apiKeys))
	for _, k := range apiKeys {
		k = strings.TrimSpace(k)
		if k != "" {
			keys = append(keys, k)
		}
	}
	if len(keys) == 0 {
		// Shouldn't happen in practice (FromEnv validates) — leave as zero
		// value so callers see clean nil-deref if they bypass FromEnv.
		keys = nil
	}
	// chatModel may be a comma-separated fallback list, e.g.
	// "gemini-2.5-flash-lite,gemini-2.5-flash,gemini-2.0-flash".
	models := make([]string, 0, 4)
	for _, m := range strings.Split(chatModel, ",") {
		m = strings.TrimSpace(m)
		if m != "" {
			models = append(models, m)
		}
	}
	if len(models) == 0 {
		models = []string{"gemini-2.5-flash"}
	}
	if embedModel == "" {
		// gemini-embedding-001 replaced text-embedding-004 on v1beta and
		// is the only family currently accepting embedContent on the
		// free tier.
		embedModel = "gemini-embedding-001"
	}
	return &geminiProvider{
		apiKeys:    keys,
		chatModels: models,
		embedModel: embedModel,
		http: &http.Client{
			// 5 min per request matches Gemini's stream lifetime and
			// covers slow embed batches over flaky networks.
			Timeout: 5 * time.Minute,
		},
	}
}

// pickKey returns the next live key plus the index it came from. It
// skips keys that are still in the cooldown window we set after a 429.
// Returns "" if every key is cooling down (caller surfaces the original
// 429 in that case).
func (g *geminiProvider) pickKey() (string, int) {
	n := len(g.apiKeys)
	if n == 0 {
		return "", -1
	}
	now := time.Now()
	for attempt := 0; attempt < n; attempt++ {
		idx := int(g.cursor.Add(1)-1) % n
		key := g.apiKeys[idx]
		if v, ok := g.cooldown.Load(key); ok {
			if until, _ := v.(time.Time); now.Before(until) {
				continue // still 429'd — try the next
			}
			g.cooldown.Delete(key)
		}
		return key, idx
	}
	return "", -1
}

// parkKey marks a key as 429'd for the given duration. Subsequent
// pickKey calls skip it until the cooldown elapses.
func (g *geminiProvider) parkKey(key string, dur time.Duration) {
	if dur <= 0 {
		dur = 60 * time.Second
	}
	g.cooldown.Store(key, time.Now().Add(dur))
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
//
// Rotation: we iterate over every (key, model) pair until one returns
// 200. opts.Model overrides the model list entirely (e.g. tests pinning
// to a specific model); otherwise we walk g.chatModels in order, and
// for each model walk g.apiKeys via pickKey(). PerDayPerProjectPerModel
// quota means each (key, model) pair has its own 20/day ceiling, so a
// 2-key × 3-model setup gets you 120 requests/day before billing.
func (g *geminiProvider) Chat(ctx context.Context, msgs []Message, opts ChatOpts) (<-chan ChatChunk, error) {
	models := g.chatModels
	if opts.Model != "" {
		models = []string{opts.Model}
	}
	if len(models) == 0 {
		return nil, errors.New("gemini: no chat model configured")
	}

	body, err := json.Marshal(g.buildChatReq(msgs, opts))
	if err != nil {
		return nil, fmt.Errorf("gemini: marshal request: %w", err)
	}

	var lastErr error
	for _, model := range models {
		for tries := 0; tries < max(1, len(g.apiKeys)); tries++ {
			key, _ := g.pickKey()
			if key == "" {
				break // every key cooling — drop to next model
			}
			// Skip (key, model) pair we already saw 429 for this hour.
			pairKey := key + "|" + model
			if v, ok := g.parkedModels.Load(pairKey); ok {
				if until, _ := v.(time.Time); time.Now().Before(until) {
					continue
				}
				g.parkedModels.Delete(pairKey)
			}

			url := fmt.Sprintf("%s/models/%s:streamGenerateContent?alt=sse&key=%s",
				geminiBaseURL, model, key)
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
			if resp.StatusCode == http.StatusOK {
				out := make(chan ChatChunk, 8)
				go g.consumeStream(resp, out)
				return out, nil
			}
			errBody, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
			resp.Body.Close()
			if resp.StatusCode == http.StatusTooManyRequests {
				// Park the (key, model) pair only — the SAME key with a
				// DIFFERENT model still has its own daily quota.
				//
				// Park-duration heuristic: Google's retryDelay hint is
				// the truth source. Short delays (< 5 min) mean we hit
				// an RPM ceiling and the pair recovers naturally — park
				// for exactly that long with a small jitter. Long
				// delays (or no delay at all) mean the daily quota is
				// gone — park for an hour to leave other (key, model)
				// pairs handling traffic without bouncing off this one.
				parkFor := parseGeminiRetryDelay(string(errBody))
				switch {
				case parkFor <= 0:
					parkFor = 60 * time.Minute
				case parkFor < 5*time.Minute:
					parkFor = parkFor + 2*time.Second // RPM hint + jitter
				default:
					// 5-60+ min delay — leave it; that's the server's call.
				}
				g.parkedModels.Store(pairKey, time.Now().Add(parkFor))
				lastErr = fmt.Errorf("%w: %s", ErrRateLimited, strings.TrimSpace(string(errBody)))
				continue // try the next key with the same model
			}
			if resp.StatusCode == http.StatusNotFound {
				// Model retired / unavailable on this key (e.g. preview-only
				// model on an account that hasn't enabled previews). Park
				// the (key, model) pair for 24h and fall through — there's
				// nothing time can fix in the same calendar day.
				g.parkedModels.Store(pairKey, time.Now().Add(24*time.Hour))
				lastErr = fmt.Errorf("gemini: chat HTTP 404: %s", strings.TrimSpace(string(errBody)))
				continue
			}
			// Other errors are not retryable — return immediately.
			return nil, fmt.Errorf("gemini: chat HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(errBody)))
		}
		// All keys exhausted for this model — fall through to next model.
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("%w: every (key, model) pair is cooling down", ErrRateLimited)
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
const geminiEmbedConcurrency = 4

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

	// Retry on 429s using the server-suggested retryDelay where present,
	// otherwise an exponential backoff capped at 30s. Free tier embedding
	// is 100 req/min — even a small PDF can hit it during burst indexing.
	// Each attempt also picks a fresh key from the rotation, so a key
	// that's currently 429'd for chat doesn't drag embeds down.
	const maxAttempts = 5
	backoff := 2 * time.Second
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		key, _ := g.pickKey()
		if key == "" {
			return nil, fmt.Errorf("%w: every key in the pool is cooling down", ErrRateLimited)
		}
		url := fmt.Sprintf("%s/models/%s:embedContent?key=%s", geminiBaseURL, g.embedModel, key)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
		if err != nil {
			return nil, fmt.Errorf("gemini: build embed: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := g.http.Do(req)
		if err != nil {
			return nil, fmt.Errorf("gemini: embed do: %w", err)
		}

		if resp.StatusCode == http.StatusOK {
			var decoded geminiEmbedResp
			if derr := json.NewDecoder(resp.Body).Decode(&decoded); derr != nil {
				resp.Body.Close()
				return nil, fmt.Errorf("gemini: decode embed: %w", derr)
			}
			resp.Body.Close()
			if len(decoded.Embedding.Values) == 0 {
				return nil, errors.New("gemini: empty embedding")
			}
			return decoded.Embedding.Values, nil
		}

		errBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		resp.Body.Close()
		trimmed := strings.TrimSpace(string(errBody))

		if resp.StatusCode == http.StatusTooManyRequests {
			lastErr = fmt.Errorf("%w: %s", ErrRateLimited, trimmed)
			// Park the offending key so the rotation skips it. We
			// honour the server-suggested retryDelay where present,
			// since that's a more accurate reset window than our
			// guess.
			parkFor := parseGeminiRetryDelay(trimmed)
			if parkFor <= 0 {
				parkFor = 60 * time.Second
			}
			g.parkKey(key, parkFor)

			if attempt == maxAttempts {
				return nil, lastErr
			}
			// With multiple keys in rotation, we can skip the sleep
			// and try the next key immediately — pickKey will hand
			// out a different one (or return "" if the whole pool is
			// cold, which short-circuits the loop above).
			if len(g.apiKeys) <= 1 {
				wait := parseGeminiRetryDelay(trimmed)
				if wait <= 0 || wait > 60*time.Second {
					wait = backoff
				}
				wait += time.Duration(attempt*350) * time.Millisecond
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case <-time.After(wait):
				}
				if backoff < 30*time.Second {
					backoff *= 2
				}
			}
			continue
		}

		return nil, fmt.Errorf("gemini: embed HTTP %d: %s", resp.StatusCode, trimmed)
	}
	return nil, lastErr
}

// parseGeminiRetryDelay extracts the retryDelay (e.g. "17s") from a
// Gemini 429 error body and returns it as a time.Duration. Zero on miss.
var geminiRetrySecRE = regexp.MustCompile(`"retryDelay"\s*:\s*"(\d+(?:\.\d+)?)s"`)

func parseGeminiRetryDelay(body string) time.Duration {
	m := geminiRetrySecRE.FindStringSubmatch(body)
	if len(m) < 2 {
		return 0
	}
	f, err := strconv.ParseFloat(m[1], 64)
	if err != nil || f <= 0 {
		return 0
	}
	return time.Duration(f * float64(time.Second))
}
