package cv

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"convertpdfgo/api/models"
	"convertpdfgo/pkg/logger"
	"convertpdfgo/service/llm"
	"convertpdfgo/service/pdfchat"
	"convertpdfgo/storage"
)

// parser owns the import flow: it loads the user's uploaded file,
// extracts plain text, and asks Gemini to return the resume as a
// structured JSON object, which we then split into cv_sections rows.
//
// We reuse the pdfchat text extractor for PDFs (page-aware pdftotext);
// other formats fall back to libreoffice or tesseract via gotenberg /
// the existing OCR pipeline.
type parser struct {
	stg storage.IStorage
	llm llm.Provider
	log logger.ILogger
}

func newParser(stg storage.IStorage, p llm.Provider, log logger.ILogger) *parser {
	return &parser{stg: stg, llm: p, log: log}
}

// Run is invoked by the asynq worker (TaskCVImportParse). It reads the
// CV session, finds the uploaded file, extracts text, calls Gemini in
// JSON mode, and writes the resulting sections to the DB.
//
// On any failure the session is marked status=failed with a short
// error string the UI can render.
func (p *parser) Run(ctx context.Context, cvID string) error {
	sess, err := p.stg.CV().GetSession(ctx, cvID)
	if err != nil {
		return fmt.Errorf("cv import: get session: %w", err)
	}
	if sess.SourceFileID == nil || *sess.SourceFileID == "" {
		return p.markFailed(ctx, cvID, errors.New("import requires a source file"))
	}
	file, err := p.stg.File().GetByID(ctx, *sess.SourceFileID)
	if err != nil {
		return p.markFailed(ctx, cvID, fmt.Errorf("get file: %w", err))
	}

	// 1) Extract plain text from whatever format the user uploaded.
	rawText, err := p.extractText(ctx, file.FilePath)
	if err != nil {
		return p.markFailed(ctx, cvID, fmt.Errorf("extract text: %w", err))
	}
	rawText = strings.TrimSpace(rawText)
	if rawText == "" {
		return p.markFailed(ctx, cvID, errors.New("the uploaded file has no extractable text — try running OCR first"))
	}
	if len(rawText) > 24000 {
		// Gemini context budget; truncate long resumes (rare).
		rawText = rawText[:24000]
	}

	// 2) Ask Gemini for a structured JSON view of the resume.
	parsed, err := p.callGemini(ctx, rawText, sess.Locale)
	if err != nil {
		return p.markFailed(ctx, cvID, fmt.Errorf("ai parse: %w", err))
	}

	// 3) Convert the parsed JSON into ordered cv_sections rows.
	sections := buildSectionsFromParse(cvID, parsed)
	if len(sections) == 0 {
		return p.markFailed(ctx, cvID, errors.New("AI returned no usable sections"))
	}

	if err := p.stg.CV().ReplaceSections(ctx, cvID, sections); err != nil {
		return p.markFailed(ctx, cvID, fmt.Errorf("save sections: %w", err))
	}

	// 4) Mark session ready + set a title from the parsed name if we have one.
	if name, _ := nestedString(parsed, "personal", "name"); name != "" {
		sess.Title = name + " — CV"
		_ = p.stg.CV().UpdateSession(ctx, sess)
	}
	return p.stg.CV().UpdateSessionStatus(ctx, cvID, "ready", "")
}

// extractText routes the file by extension to the right plain-text
// extractor. PDF stays on pdftotext (page-aware), Office docs go
// through libreoffice headless, scans/images go through tesseract.
func (p *parser) extractText(ctx context.Context, path string) (string, error) {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".pdf":
		pages, err := pdfchat.ExtractPages(ctx, path)
		if err != nil {
			return "", err
		}
		var sb strings.Builder
		for _, page := range pages {
			sb.WriteString(page.Text)
			sb.WriteByte('\n')
		}
		return sb.String(), nil
	case ".docx", ".doc", ".rtf", ".odt":
		return libreofficeToText(ctx, path)
	case ".txt", ".md":
		out, err := exec.CommandContext(ctx, "cat", path).Output()
		return string(out), err
	case ".jpg", ".jpeg", ".png", ".tiff", ".webp":
		return tesseractToText(ctx, path)
	}
	return "", fmt.Errorf("unsupported source format %q — upload PDF, Word, or an image", ext)
}

// libreofficeToText runs LibreOffice in headless mode, asks for a .txt
// output next to the original file, then returns the contents.
func libreofficeToText(ctx context.Context, path string) (string, error) {
	dir := filepath.Dir(path)
	cmd := exec.CommandContext(ctx, "libreoffice", "--headless", "--convert-to", "txt", "--outdir", dir, path)
	if b, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("libreoffice: %w (%s)", err, strings.TrimSpace(string(b)))
	}
	base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	out := filepath.Join(dir, base+".txt")
	cat, err := exec.CommandContext(ctx, "cat", out).Output()
	if err != nil {
		return "", err
	}
	return string(cat), nil
}

// tesseractToText shells out to the tesseract binary (already installed
// for the OCR feature). Defaults to English + Uzbek + Russian since
// those cover the bulk of our traffic; for arbitrary languages the user
// should run OCR via the existing /ocr tool first.
func tesseractToText(ctx context.Context, path string) (string, error) {
	cmd := exec.CommandContext(ctx, "tesseract", path, "-", "-l", "eng+rus+uzb_cyrl+uzb")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("tesseract: %w", err)
	}
	return string(out), nil
}

// callGemini runs the JSON-mode parse — system instruction asks for a
// strict schema, response is unmarshalled and returned.
func (p *parser) callGemini(ctx context.Context, rawText, locale string) (map[string]any, error) {
	system := parserSystemPrompt(locale)
	userPrompt := "Resume text:\n---\n" + rawText + "\n---\n\nReturn only the JSON object, no commentary."

	ch, err := p.llm.Chat(ctx, []llm.Message{
		{Role: llm.RoleUser, Content: userPrompt},
	}, llm.ChatOpts{
		System:      system,
		Temperature: 0.1, // structured output → low temp
		MaxTokens:   8192,
	})
	if err != nil {
		return nil, err
	}
	var sb strings.Builder
	for c := range ch {
		if c.Err != nil {
			return nil, c.Err
		}
		sb.WriteString(c.Delta)
	}
	raw := strings.TrimSpace(sb.String())
	// Strip ```json fences if the model added them.
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	raw = strings.TrimSpace(raw)

	var parsed map[string]any
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return nil, fmt.Errorf("decode AI response (%d bytes): %w", len(raw), err)
	}
	return parsed, nil
}

func parserSystemPrompt(locale string) string {
	// The schema is in English — Gemini handles cross-language sources
	// fine, and the section_type values stay stable for the DB.
	return `You are a CV/resume parser. Extract the structured data from the resume text below and return ONLY a valid JSON object with this exact shape (omit any field you cannot find rather than inventing data):

{
  "personal": {
    "name": "",
    "title": "",
    "email": "",
    "phone": "",
    "location": "",
    "linkedin": "",
    "github": "",
    "website": ""
  },
  "summary": "<2-3 sentence professional summary>",
  "experience": [
    { "title": "", "company": "", "period": "", "location": "", "bullets": ["", ""] }
  ],
  "education": [
    { "degree": "", "school": "", "period": "", "gpa": "", "honors": "" }
  ],
  "skills": { "technical": [], "soft": [] },
  "languages": [
    { "name": "", "level": "" }
  ],
  "projects": [
    { "name": "", "description": "", "tech": [], "link": "" }
  ],
  "awards": [{ "title": "", "issuer": "", "year": "" }],
  "certifications": [{ "name": "", "issuer": "", "year": "" }]
}

Rules:
- Preserve the user's wording on bullets — don't paraphrase.
- Empty string for missing fields, not null and not made-up.
- Dates: normalize to "MMM YYYY - MMM YYYY" (or "MMM YYYY - Present").
- Respond in the source language; do not translate.
- Output JSON only, no markdown fences, no commentary, no trailing text.`
}

// buildSectionsFromParse turns the parsed JSON into ordered CV section
// rows. We hard-code the section order so the generated CV reads the
// way humans expect it to.
func buildSectionsFromParse(cvID string, parsed map[string]any) []models.CVSection {
	out := make([]models.CVSection, 0, 10)
	order := 0
	add := func(secType string, content map[string]any) {
		if len(content) == 0 || isEmptyContent(content) {
			return
		}
		out = append(out, models.CVSection{
			CVID:        cvID,
			SectionType: secType,
			SortOrder:   order,
			Content:     content,
		})
		order++
	}

	if v, ok := parsed["personal"].(map[string]any); ok {
		add(models.SectionPersonal, v)
	}
	if v, ok := parsed["summary"].(string); ok && strings.TrimSpace(v) != "" {
		add(models.SectionSummary, map[string]any{"text": v})
	}
	if v, ok := parsed["experience"].([]any); ok && len(v) > 0 {
		add(models.SectionExperience, map[string]any{"items": v})
	}
	if v, ok := parsed["education"].([]any); ok && len(v) > 0 {
		add(models.SectionEducation, map[string]any{"items": v})
	}
	if v, ok := parsed["skills"].(map[string]any); ok {
		add(models.SectionSkills, v)
	}
	if v, ok := parsed["languages"].([]any); ok && len(v) > 0 {
		add(models.SectionLanguages, map[string]any{"items": v})
	}
	if v, ok := parsed["projects"].([]any); ok && len(v) > 0 {
		add(models.SectionProjects, map[string]any{"items": v})
	}
	if v, ok := parsed["awards"].([]any); ok && len(v) > 0 {
		add(models.SectionAwards, map[string]any{"items": v})
	}
	if v, ok := parsed["certifications"].([]any); ok && len(v) > 0 {
		add(models.SectionCertifications, map[string]any{"items": v})
	}
	return out
}

// isEmptyContent returns true when every value is empty/zero — keeps
// us from creating useless sections like a "summary" with text "".
func isEmptyContent(m map[string]any) bool {
	for _, v := range m {
		switch t := v.(type) {
		case string:
			if strings.TrimSpace(t) != "" {
				return false
			}
		case []any:
			if len(t) > 0 {
				return false
			}
		case map[string]any:
			if !isEmptyContent(t) {
				return false
			}
		default:
			if v != nil {
				return false
			}
		}
	}
	return true
}

func nestedString(m map[string]any, keys ...string) (string, bool) {
	cur := any(m)
	for _, k := range keys {
		mp, ok := cur.(map[string]any)
		if !ok {
			return "", false
		}
		cur = mp[k]
	}
	s, ok := cur.(string)
	return strings.TrimSpace(s), ok && s != ""
}

func (p *parser) markFailed(ctx context.Context, cvID string, err error) error {
	msg := err.Error()
	if len(msg) > 500 {
		msg = msg[:500]
	}
	if updateErr := p.stg.CV().UpdateSessionStatus(ctx, cvID, "failed", msg); updateErr != nil {
		p.log.Warning("cv parser: mark failed", logger.Error(updateErr))
	}
	return err
}
