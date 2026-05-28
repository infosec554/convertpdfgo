package cv

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"convertpdfgo/api/models"
	"convertpdfgo/pkg/logger"
	"convertpdfgo/service/llm"
	"convertpdfgo/storage"
)

// translator turns every section of a CV into the requested target
// locale in one go. We do all sections in a single LLM call because
// (a) it preserves cross-section context (e.g. "Senior Engineer" in
// experience should match what we put in personal.title), and (b) it
// halves the per-translation cost vs N round-trips.
type translator struct {
	stg storage.IStorage
	llm llm.Provider
	log logger.ILogger
}

func newTranslator(stg storage.IStorage, p llm.Provider, log logger.ILogger) *translator {
	return &translator{stg: stg, llm: p, log: log}
}

// Run rewrites every section in the new locale and saves them back via
// ReplaceSections (single transaction). The session row's locale field
// is updated too so subsequent renders pick the right template strings.
func (t *translator) Run(ctx context.Context, cvID, targetLocale string) error {
	targetLocale = strings.ToLower(strings.TrimSpace(targetLocale))
	if targetLocale == "" {
		return fmt.Errorf("translate: target_locale is required")
	}
	sess, err := t.stg.CV().GetSession(ctx, cvID)
	if err != nil {
		return err
	}
	if sess.Locale == targetLocale {
		// Nothing to do.
		return nil
	}

	sections, err := t.stg.CV().ListSections(ctx, cvID)
	if err != nil {
		return err
	}
	if len(sections) == 0 {
		// Empty CV — just flip the locale.
		sess.Locale = targetLocale
		return t.stg.CV().UpdateSession(ctx, sess)
	}

	// Serialise the section map once so we ship the whole CV in one
	// prompt. We send section_type + content; the model must respect
	// the same shape on output.
	input, err := json.Marshal(sectionsAsMap(sections))
	if err != nil {
		return fmt.Errorf("translate: marshal: %w", err)
	}

	prompt := fmt.Sprintf(`Translate every value in the JSON object below from %s to %s.

Rules:
- Preserve the JSON structure exactly — same keys, same nesting, same array order.
- Translate values only (strings inside the structure); never translate keys.
- Preserve proper nouns (company names, school names, technology names) as-is.
- Dates stay numeric / unchanged.
- Email, phone, URLs stay unchanged.
- Output JSON only. No fences, no commentary.

Input:
%s`, sess.Locale, targetLocale, string(input))

	ch, err := t.llm.Chat(ctx, []llm.Message{{Role: llm.RoleUser, Content: prompt}}, llm.ChatOpts{
		Temperature: 0.1,
		MaxTokens:   8000,
	})
	if err != nil {
		return fmt.Errorf("translate: llm: %w", err)
	}
	var sb strings.Builder
	for c := range ch {
		if c.Err != nil {
			return c.Err
		}
		sb.WriteString(c.Delta)
	}

	raw := stripJSONFences(sb.String())
	var translated map[string]map[string]any
	if err := json.Unmarshal([]byte(raw), &translated); err != nil {
		return fmt.Errorf("translate: decode (%d bytes): %w", len(raw), err)
	}

	// Apply the translated content back onto our sections, preserving
	// IDs, sort order, and section_type so the editor doesn't see a
	// completely different set of rows after the swap.
	out := make([]models.CVSection, 0, len(sections))
	for _, s := range sections {
		newContent, ok := translated[s.SectionType]
		if !ok {
			// Translator dropped this section — keep the original.
			out = append(out, s)
			continue
		}
		out = append(out, models.CVSection{
			ID:          s.ID,
			CVID:        s.CVID,
			SectionType: s.SectionType,
			SortOrder:   s.SortOrder,
			Content:     newContent,
		})
	}
	if err := t.stg.CV().ReplaceSections(ctx, cvID, out); err != nil {
		return fmt.Errorf("translate: persist: %w", err)
	}

	sess.Locale = targetLocale
	return t.stg.CV().UpdateSession(ctx, sess)
}

// sectionsAsMap turns the row-per-section storage shape into a flat
// section_type → content map, which is what we feed Gemini. Two
// sections of the same type would collide — we'd lose one — but the
// schema doesn't actually allow that today (parser emits each type at
// most once and the UI nudges users toward `custom` for extras).
func sectionsAsMap(sections []models.CVSection) map[string]map[string]any {
	out := make(map[string]map[string]any, len(sections))
	for _, s := range sections {
		out[s.SectionType] = s.Content
	}
	return out
}
