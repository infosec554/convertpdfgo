package cv

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"convertpdfgo/api/models"
	"convertpdfgo/pkg/logger"
	"convertpdfgo/service/llm"
	"convertpdfgo/storage"
)

// improver rewrites a single field (typically a bullet point) into
// three alternative phrasings, each leaning on a different rhetorical
// frame so the user can pick the one closest to what they meant.
//
// Styles match what most resume coaches recommend:
//
//   - metric-driven    — leads with a number ("Cut deploy time by 40%…")
//   - scope-focused    — leads with breadth ("Owned the payments stack…")
//   - ownership-focused — leads with action verb + responsibility
//     ("Drove the migration from monolith to services…")
type improver struct {
	llm llm.Provider
	log logger.ILogger
}

func newImprover(p llm.Provider, log logger.ILogger) *improver {
	return &improver{llm: p, log: log}
}

// Improve returns three rewrites of req.CurrentText. Each call is also
// recorded in cv_ai_history so we can a) charge fairly, b) dedup repeat
// requests for the same text in future.
func (i *improver) Improve(ctx context.Context, stg storage.IStorage, cvID string, req models.CVImproveRequest) (*models.CVImproveResponse, error) {
	cur := strings.TrimSpace(req.CurrentText)
	if cur == "" {
		return nil, errors.New("cv improver: current_text is empty")
	}

	prompt := improvePrompt(cur, req.Context)
	ch, err := i.llm.Chat(ctx, []llm.Message{
		{Role: llm.RoleUser, Content: prompt},
	}, llm.ChatOpts{
		System:      improveSystem,
		Temperature: 0.7, // some variety between options
		MaxTokens:   600,
	})
	if err != nil {
		return nil, fmt.Errorf("llm: %w", err)
	}

	var sb strings.Builder
	var tokensIn, tokensOut int
	for c := range ch {
		if c.Err != nil {
			return nil, c.Err
		}
		sb.WriteString(c.Delta)
		if c.FinishReason != "" {
			tokensIn = c.TokensIn
			tokensOut = c.TokensOut
		}
	}

	suggestions := parseImproveResponse(sb.String())
	if len(suggestions) == 0 {
		return nil, errors.New("cv improver: AI returned no usable suggestions")
	}

	// Best-effort history log; failure here is not user-visible.
	_ = stg.CV().LogAIImprove(ctx, cvID, "", req.FieldPath, cur, suggestions, tokensIn, tokensOut)

	return &models.CVImproveResponse{
		Suggestions: suggestions,
		TokensUsed:  tokensIn + tokensOut,
	}, nil
}

const improveSystem = `You are an expert resume writer. Given one bullet point or sentence from a CV, produce THREE rewrites — each in a different rhetorical style:

1. "metric-driven"     — leads with a quantifiable number (%, dollars, time, headcount).
2. "scope-focused"     — leads with the scope/scale of what was owned ("Owned X across Y teams…").
3. "ownership-focused" — leads with a strong action verb conveying responsibility (Led / Drove / Owned / Architected).

Rules for every rewrite:
- Maximum 25 words. Concise beats clever.
- Start with an action verb (Led, Built, Shipped, Drove, Owned, Designed, etc.). Never "I" or "Responsible for".
- Preserve the underlying achievement — don't invent numbers.
- Respond ONLY in JSON: { "suggestions": [ {"text":"…","style":"metric-driven"}, {"text":"…","style":"scope-focused"}, {"text":"…","style":"ownership-focused"} ] }
- No markdown fences, no commentary, JSON only.`

func improvePrompt(text string, contextHints map[string]any) string {
	var sb strings.Builder
	sb.WriteString("Bullet to rewrite:\n")
	sb.WriteString(text)
	sb.WriteString("\n")
	if len(contextHints) > 0 {
		sb.WriteString("\nContext (optional):\n")
		for k, v := range contextHints {
			fmt.Fprintf(&sb, "- %s: %v\n", k, v)
		}
	}
	return sb.String()
}

func parseImproveResponse(raw string) []models.CVSuggestion {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	raw = strings.TrimSpace(raw)

	var parsed struct {
		Suggestions []models.CVSuggestion `json:"suggestions"`
	}
	if err := json.Unmarshal([]byte(raw), &parsed); err == nil && len(parsed.Suggestions) > 0 {
		return parsed.Suggestions
	}

	// Fallback: try to salvage 3 lines from a malformed response so the
	// user still gets something rather than an empty popover.
	lines := strings.Split(raw, "\n")
	out := make([]models.CVSuggestion, 0, 3)
	styles := []string{"metric-driven", "scope-focused", "ownership-focused"}
	for _, ln := range lines {
		ln = strings.TrimSpace(strings.Trim(ln, "-*•·123. \""))
		if ln == "" {
			continue
		}
		out = append(out, models.CVSuggestion{
			Text:  ln,
			Style: styles[len(out)%len(styles)],
		})
		if len(out) == 3 {
			break
		}
	}
	return out
}
