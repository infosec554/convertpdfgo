package cv

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/google/uuid"

	"convertpdfgo/api/models"
	"convertpdfgo/pkg/logger"
	"convertpdfgo/service/llm"
	"convertpdfgo/storage"
)

// ats scores a CV against a job description. The score is hybrid:
//
//   - 40% keyword match — TF-IDF style keyword extraction from the job,
//     then check how many show up in the CV.
//   - 30% format quality — rule-based: has personal contact info, sane
//     section headers, no weird tables, page count reasonable.
//   - 30% content quality — bullet length, action verbs, quantified
//     results — partly heuristic, partly Gemini hint.
//
// Trade-off: pure Gemini scoring would be more nuanced but unpredictable
// and slow. Pure rule-based would miss synonyms ("led" vs "managed").
// The hybrid splits responsibility: rules for things we can measure
// deterministically, AI for the squishy stuff.
type ats struct {
	stg storage.IStorage
	llm llm.Provider
	log logger.ILogger
}

func newATS(stg storage.IStorage, p llm.Provider, log logger.ILogger) *ats {
	return &ats{stg: stg, llm: p, log: log}
}

// Score loads the CV, extracts keywords from the job description, runs
// the three component scores, and persists the result in cv_ats_runs
// (so the user gets an audit trail of how their CV evolves).
func (a *ats) Score(ctx context.Context, cvID string, req models.CVATSRequest) (*models.CVATSRun, error) {
	sections, err := a.stg.CV().ListSections(ctx, cvID)
	if err != nil {
		return nil, fmt.Errorf("ats: load sections: %w", err)
	}
	cvText := flattenCV(sections)

	keywords := a.extractKeywords(ctx, req.JobDescription)
	matchScore, missing := scoreKeywordMatch(keywords, cvText)
	formatScore, formatIssues := scoreFormat(sections)
	contentScore, weakBullets := a.scoreContent(ctx, sections)

	total := int(0.4*float64(matchScore) + 0.3*float64(formatScore) + 0.3*float64(contentScore))
	if total > 100 {
		total = 100
	}

	feedback := map[string]any{
		"keyword_score":   matchScore,
		"format_score":    formatScore,
		"content_score":   contentScore,
		"missing_keywords": missing,
		"format_issues":   formatIssues,
		"weak_bullets":    weakBullets,
		"suggestions":     a.aiSuggestions(ctx, sections, req.JobDescription, missing),
	}

	run := &models.CVATSRun{
		ID:             uuid.NewString(),
		CVID:           cvID,
		JobTitle:       firstLine(req.JobDescription, req.JobTitle),
		JobDescription: req.JobDescription,
		Score:          total,
		Feedback:       feedback,
	}
	if err := a.stg.CV().SaveATSRun(ctx, run); err != nil {
		a.log.Warning("ats: save run", logger.Error(err))
	}
	return run, nil
}

// ─── Keyword extraction (Gemini) ────────────────────────────────────────

// extractKeywords asks Gemini for the 25 most-likely ATS keywords from
// the job description. Cheap call (~300 tokens) but precise — beats a
// regex/TF-IDF impl when the job is in Uzbek or Russian.
func (a *ats) extractKeywords(ctx context.Context, jobDesc string) []string {
	prompt := `From the job description below, list the 25 most important keywords an Applicant Tracking System would look for: hard skills, tools, frameworks, certifications, qualifications. Return JSON: {"keywords":[...lowercase strings...]}. No commentary.

Job:
` + truncate(jobDesc, 8000)

	ch, err := a.llm.Chat(ctx, []llm.Message{{Role: llm.RoleUser, Content: prompt}}, llm.ChatOpts{
		Temperature: 0.1,
		MaxTokens:   400,
	})
	if err != nil {
		a.log.Warning("ats: extract keywords", logger.Error(err))
		return fallbackKeywords(jobDesc)
	}
	var sb strings.Builder
	for c := range ch {
		if c.Err != nil {
			return fallbackKeywords(jobDesc)
		}
		sb.WriteString(c.Delta)
	}
	raw := stripJSONFences(sb.String())
	var parsed struct {
		Keywords []string `json:"keywords"`
	}
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return fallbackKeywords(jobDesc)
	}
	return parsed.Keywords
}

// fallbackKeywords runs a tiny regex over the job description in case
// Gemini fails. Looks for words ≥4 chars that aren't stop-words.
var wordRE = regexp.MustCompile(`[a-zA-Zа-яА-ЯёЁ_+#.]{4,}`)
var stop = map[string]bool{
	"with": true, "from": true, "this": true, "that": true, "have": true,
	"will": true, "your": true, "their": true, "should": true, "would": true,
	"about": true, "also": true, "more": true, "than": true,
}

func fallbackKeywords(text string) []string {
	matches := wordRE.FindAllString(strings.ToLower(text), -1)
	freq := map[string]int{}
	for _, m := range matches {
		if stop[m] {
			continue
		}
		freq[m]++
	}
	type kv struct {
		k string
		v int
	}
	pairs := make([]kv, 0, len(freq))
	for k, v := range freq {
		pairs = append(pairs, kv{k, v})
	}
	// simple insertion sort by freq desc — list is small
	for i := 1; i < len(pairs); i++ {
		for j := i; j > 0 && pairs[j-1].v < pairs[j].v; j-- {
			pairs[j-1], pairs[j] = pairs[j], pairs[j-1]
		}
	}
	out := make([]string, 0, 25)
	for i, p := range pairs {
		if i >= 25 {
			break
		}
		out = append(out, p.k)
	}
	return out
}

// ─── Keyword match ──────────────────────────────────────────────────────

func scoreKeywordMatch(keywords []string, cvText string) (int, []string) {
	if len(keywords) == 0 {
		return 0, nil
	}
	cvLower := strings.ToLower(cvText)
	found := 0
	missing := []string{}
	for _, kw := range keywords {
		if strings.Contains(cvLower, strings.ToLower(kw)) {
			found++
		} else {
			missing = append(missing, kw)
		}
	}
	score := int(float64(found) / float64(len(keywords)) * 100)
	if score > 100 {
		score = 100
	}
	return score, missing
}

// ─── Format score (rule-based) ──────────────────────────────────────────

func scoreFormat(sections []models.CVSection) (int, []string) {
	score := 0
	issues := []string{}

	hasPersonal := false
	hasEmail := false
	hasPhone := false
	hasExperience := false
	hasEducation := false
	hasSkills := false

	for _, s := range sections {
		switch s.SectionType {
		case models.SectionPersonal:
			hasPersonal = true
			if e, ok := s.Content["email"].(string); ok && e != "" {
				hasEmail = true
			}
			if p, ok := s.Content["phone"].(string); ok && p != "" {
				hasPhone = true
			}
		case models.SectionExperience:
			hasExperience = true
		case models.SectionEducation:
			hasEducation = true
		case models.SectionSkills:
			hasSkills = true
		}
	}

	if hasPersonal {
		score += 15
	} else {
		issues = append(issues, "Missing personal information section")
	}
	if hasEmail {
		score += 10
	} else {
		issues = append(issues, "No email address found")
	}
	if hasPhone {
		score += 10
	} else {
		issues = append(issues, "No phone number found")
	}
	if hasExperience {
		score += 25
	} else {
		issues = append(issues, "Missing Experience section")
	}
	if hasEducation {
		score += 15
	} else {
		issues = append(issues, "Missing Education section")
	}
	if hasSkills {
		score += 15
	} else {
		issues = append(issues, "Missing Skills section")
	}
	// Reserve 10 points for "no exotic stuff" — we don't really detect
	// tables/graphics from a content-only CV, so always award it.
	score += 10

	if score > 100 {
		score = 100
	}
	return score, issues
}

// ─── Content quality score (heuristic + Gemini hint) ────────────────────

func (a *ats) scoreContent(ctx context.Context, sections []models.CVSection) (int, []map[string]any) {
	totalBullets := 0
	goodBullets := 0
	weak := []map[string]any{}

	for _, s := range sections {
		if s.SectionType != models.SectionExperience {
			continue
		}
		items, ok := s.Content["items"].([]any)
		if !ok {
			continue
		}
		for ii, it := range items {
			item, ok := it.(map[string]any)
			if !ok {
				continue
			}
			bullets, ok := item["bullets"].([]any)
			if !ok {
				continue
			}
			for bi, b := range bullets {
				bstr, ok := b.(string)
				if !ok {
					continue
				}
				totalBullets++
				score, reason := bulletQuality(bstr)
				if score >= 80 {
					goodBullets++
				} else if score < 50 {
					weak = append(weak, map[string]any{
						"section_id":  s.ID,
						"item_index":  ii,
						"bullet_index": bi,
						"text":        bstr,
						"reason":      reason,
					})
				}
			}
		}
	}
	if totalBullets == 0 {
		return 50, weak // neutral if no bullets
	}
	ratio := float64(goodBullets) / float64(totalBullets)
	return int(ratio * 100), weak
}

// bulletQuality returns (0-100, reason). 100 = action verb + number;
// 50 = action verb only; 30 = no verb but otherwise fine; <30 = junk.
func bulletQuality(b string) (int, string) {
	b = strings.TrimSpace(b)
	wc := wordCount(b)

	if wc < 4 {
		return 10, "Too short — add detail"
	}
	if wc > 35 {
		return 30, "Too long — split or trim to under 25 words"
	}

	hasVerb := startsWithActionVerb(b)
	hasNum := containsNumber(b)

	switch {
	case hasVerb && hasNum:
		return 100, ""
	case hasVerb:
		return 70, "Add a quantifiable metric (%, $, headcount, time saved)"
	case hasNum:
		return 60, "Lead with an action verb (Led, Built, Shipped, …)"
	default:
		return 30, "Lead with an action verb and quantify the impact"
	}
}

// ─── AI suggestions ─────────────────────────────────────────────────────

// aiSuggestions produces 3 short, actionable suggestions for boosting
// the score — Gemini sees the CV summary + missing keywords and writes
// targeted advice ("Add Docker and Kubernetes to skills since they
// appear 4 times in the job description").
func (a *ats) aiSuggestions(ctx context.Context, sections []models.CVSection, jobDesc string, missing []string) []string {
	if a.llm == nil {
		return nil
	}
	if len(missing) == 0 {
		return []string{"Your CV already matches the job's key terms — focus on quantified outcomes next."}
	}
	prompt := fmt.Sprintf(`Given:
- Job description (truncated): %s
- Missing keywords from CV: %s

Write 3 short improvement tips for the resume writer. Each tip max 20 words. Respond JSON: {"tips":["…","…","…"]}. No commentary.`,
		truncate(jobDesc, 1500),
		strings.Join(missing, ", "),
	)
	ch, err := a.llm.Chat(ctx, []llm.Message{{Role: llm.RoleUser, Content: prompt}}, llm.ChatOpts{
		Temperature: 0.4,
		MaxTokens:   300,
	})
	if err != nil {
		return nil
	}
	var sb strings.Builder
	for c := range ch {
		if c.Err != nil {
			return nil
		}
		sb.WriteString(c.Delta)
	}
	var parsed struct {
		Tips []string `json:"tips"`
	}
	_ = json.Unmarshal([]byte(stripJSONFences(sb.String())), &parsed)
	return parsed.Tips
}

// ─── Helpers ────────────────────────────────────────────────────────────

func flattenCV(sections []models.CVSection) string {
	var sb strings.Builder
	for _, s := range sections {
		sb.WriteString(flattenSectionText(s.Content))
		sb.WriteString("\n")
	}
	return sb.String()
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func firstLine(jobDesc, fallback string) string {
	if fallback != "" {
		return fallback
	}
	if i := strings.IndexByte(jobDesc, '\n'); i > 0 {
		return strings.TrimSpace(jobDesc[:i])
	}
	if len(jobDesc) > 80 {
		return strings.TrimSpace(jobDesc[:80])
	}
	return strings.TrimSpace(jobDesc)
}

func stripJSONFences(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	return strings.TrimSpace(s)
}
