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

// voiceParser turns a chunk of spoken transcript (delivered by the
// browser's Web Speech API) into one or more structured cv_sections.
//
// The browser stream is naive — user speaks freely about whatever
// section comes to mind, often jumping between experience, skills, and
// summary in a single breath. Gemini's job is to figure out which
// section the speech belongs to and emit a normalised content object
// matching the schema each section type uses.
type voiceParser struct {
	llm llm.Provider
	log logger.ILogger
}

func newVoiceParser(p llm.Provider, log logger.ILogger) *voiceParser {
	return &voiceParser{llm: p, log: log}
}

// Parse runs the transcript through Gemini and persists a raw record
// in cv_voice_segments for debugging / future model fine-tuning.
// Returns the parsed sections so the client can patch the editor in
// real time without waiting for a full session reload.
func (v *voiceParser) Parse(ctx context.Context, stg storage.IStorage, cvID string, req models.CVVoiceRequest) (*models.CVVoiceResponse, error) {
	transcript := strings.TrimSpace(req.Transcript)
	if transcript == "" {
		return nil, errors.New("cv voice: transcript is empty")
	}
	if len(transcript) > 4000 {
		transcript = transcript[:4000]
	}

	locale := req.Locale
	if locale == "" {
		locale = "en"
	}

	prompt := voicePrompt(transcript, req.SectionHint, locale)
	ch, err := v.llm.Chat(ctx, []llm.Message{
		{Role: llm.RoleUser, Content: prompt},
	}, llm.ChatOpts{
		System:      voiceSystem,
		Temperature: 0.2,
		MaxTokens:   1500,
	})
	if err != nil {
		return nil, fmt.Errorf("llm: %w", err)
	}
	var sb strings.Builder
	for c := range ch {
		if c.Err != nil {
			return nil, c.Err
		}
		sb.WriteString(c.Delta)
	}

	parsed := parseVoiceResponse(sb.String())

	// Best-effort persistence — failure here is acceptable, the caller
	// already has the parsed result back via the API response.
	primary := ""
	var primaryContent map[string]any
	if len(parsed) > 0 {
		primary = parsed[0].SectionType
		primaryContent = parsed[0].Content
	}
	_ = stg.CV().SaveVoiceSegment(ctx, cvID, transcript, primary, primaryContent, locale)

	return &models.CVVoiceResponse{ParsedSections: parsed}, nil
}

const voiceSystem = `You are converting a free-form spoken-aloud description of someone's career into structured resume sections.

The speaker may jump between topics in one breath. Detect which CV section each part belongs to. Possible section_type values: "personal", "summary", "experience", "education", "skills", "languages", "projects", "awards", "certifications", "volunteer".

Output strictly:
{ "parsed_sections": [
    { "section_type": "experience", "content": { ... section-specific schema ... }, "confidence": 0.0-1.0 }
] }

Section content schemas:
- personal: { name, title, email, phone, location, linkedin, github, website } (strings)
- summary: { text }
- experience: { items: [ { title, company, period, location, bullets: [] } ] }
- education: { items: [ { degree, school, period, gpa, honors } ] }
- skills: { technical: [], soft: [] }
- languages: { items: [ { name, level } ] }
- projects: { items: [ { name, description, tech: [], link } ] }
- awards: { items: [ { title, issuer, year } ] }
- certifications: { items: [ { name, issuer, year } ] }
- volunteer: { items: [ { role, organisation, period, description } ] }

Rules:
- Preserve the speaker's wording in bullets; clean filler words but don't paraphrase.
- Confidence 0.0–1.0 reflects how sure you are this segment is the section type you chose.
- Output JSON only. No markdown fences. No commentary.
- Respond in the same language as the transcript.`

func voicePrompt(transcript, hint, locale string) string {
	var sb strings.Builder
	sb.WriteString("Transcript:\n")
	sb.WriteString(transcript)
	sb.WriteString("\n")
	if hint != "" {
		sb.WriteString("Section hint from UI (the user picked this section before speaking): ")
		sb.WriteString(hint)
		sb.WriteString("\n")
	}
	sb.WriteString("Locale: ")
	sb.WriteString(locale)
	return sb.String()
}

func parseVoiceResponse(raw string) []models.CVParsedSection {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	raw = strings.TrimSpace(raw)

	var parsed struct {
		ParsedSections []models.CVParsedSection `json:"parsed_sections"`
	}
	if err := json.Unmarshal([]byte(raw), &parsed); err == nil {
		return parsed.ParsedSections
	}
	return nil
}
