package models

import "time"

// ─── CV session ──────────────────────────────────────────────────────────

// CVSession represents one resume the user is building. status moves
// draft → parsing (only when mode=import) → ready, or → failed.
type CVSession struct {
	ID           string    `json:"id"`
	UserID       string    `json:"user_id"`
	Title        string    `json:"title"`
	Mode         string    `json:"mode"` // scratch | import | voice
	SourceFileID *string   `json:"source_file_id,omitempty"`
	TemplateID   string    `json:"template_id"`
	Locale       string    `json:"locale"`
	Status       string    `json:"status"` // draft | parsing | ready | failed
	ParseError   string    `json:"parse_error,omitempty"`
	LastPDFID    *string   `json:"last_pdf_id,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// CVCreateRequest is the body of POST /api/cv/sessions.
type CVCreateRequest struct {
	Mode         string  `json:"mode" binding:"required"` // scratch | import | voice
	SourceFileID *string `json:"source_file_id,omitempty"`
	TemplateID   string  `json:"template_id,omitempty"`
	Locale       string  `json:"locale,omitempty"`
	Title        string  `json:"title,omitempty"`
}

// CVUpdatePatch is PATCH /api/cv/sessions/<id> — partial update for meta.
type CVUpdatePatch struct {
	Title      *string `json:"title,omitempty"`
	TemplateID *string `json:"template_id,omitempty"`
	Locale     *string `json:"locale,omitempty"`
}

// ─── CV sections ─────────────────────────────────────────────────────────

// Allowed section_type values. Anything outside the list still saves
// (the column is VARCHAR(30), not enum) but the renderer ignores
// unknown types so the UI falls back gracefully.
const (
	SectionPersonal       = "personal"
	SectionSummary        = "summary"
	SectionExperience     = "experience"
	SectionEducation      = "education"
	SectionSkills         = "skills"
	SectionLanguages      = "languages"
	SectionProjects       = "projects"
	SectionAwards         = "awards"
	SectionCertifications = "certifications"
	SectionPublications   = "publications"
	SectionVolunteer      = "volunteer"
	SectionCustom         = "custom"
)

// CVSection is one block of the CV (personal, experience, etc.).
// Content is JSONB on the DB side; the shape depends on SectionType.
type CVSection struct {
	ID          string         `json:"id"`
	CVID        string         `json:"cv_id"`
	SectionType string         `json:"section_type"`
	SortOrder   int            `json:"sort_order"`
	Content     map[string]any `json:"content"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

// CVSessionDetail is the response of GET /api/cv/sessions/<id> — bundles
// the session row, its ordered sections, and the latest ATS score (if any).
type CVSessionDetail struct {
	CVSession
	Sections []CVSection `json:"sections"`
	ATSScore *int        `json:"ats_score,omitempty"`
}

// CVSectionUpsert is the body for adding / updating a section.
type CVSectionUpsert struct {
	SectionType string         `json:"section_type" binding:"required"`
	SortOrder   int            `json:"sort_order,omitempty"`
	Content     map[string]any `json:"content" binding:"required"`
}

// CVSectionReorder is the body of /sections/reorder — a flat array of
// section IDs in their new order.
type CVSectionReorder struct {
	Order []string `json:"order" binding:"required"`
}

// ─── Templates ───────────────────────────────────────────────────────────

type CVTemplate struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Description   string `json:"description"`
	IsATSFriendly bool   `json:"is_ats_friendly"`
	PreviewURL    string `json:"preview_url,omitempty"`
	Category      string `json:"category,omitempty"`
	SortOrder     int    `json:"sort_order"`
}

// ─── AI Improve ──────────────────────────────────────────────────────────

// CVImproveRequest asks for 3 alternative phrasings of a field.
type CVImproveRequest struct {
	FieldPath   string         `json:"field_path" binding:"required"`
	CurrentText string         `json:"current_text" binding:"required"`
	Context     map[string]any `json:"context,omitempty"`
}

// CVImproveResponse returns the three suggestions plus token usage.
type CVImproveResponse struct {
	Suggestions []CVSuggestion `json:"suggestions"`
	TokensUsed  int            `json:"tokens_used"`
}

// CVSuggestion is one variant of an improved field.
type CVSuggestion struct {
	Text  string `json:"text"`
	Style string `json:"style"` // metric-driven | scope-focused | ownership-focused
}

// ─── Voice mode ──────────────────────────────────────────────────────────

// CVVoiceRequest carries a chunk of speech transcript for parsing.
type CVVoiceRequest struct {
	Transcript  string `json:"transcript" binding:"required"`
	SectionHint string `json:"section_hint,omitempty"`
	Locale      string `json:"locale,omitempty"`
}

// CVVoiceResponse echoes back what the AI extracted, so the client can
// optimistically render the new content while the row hits the DB.
type CVVoiceResponse struct {
	ParsedSections []CVParsedSection `json:"parsed_sections"`
}

// CVParsedSection is one (section_type, content) pair produced from a
// voice transcript.
type CVParsedSection struct {
	SectionType string         `json:"section_type"`
	Content     map[string]any `json:"content"`
	Confidence  float32        `json:"confidence,omitempty"`
}

// ─── ATS scoring ─────────────────────────────────────────────────────────

// CVATSRequest scores a CV against a specific job posting.
type CVATSRequest struct {
	JobTitle       string `json:"job_title,omitempty"`
	JobDescription string `json:"job_description" binding:"required"`
}

// CVATSRun is one stored scoring run.
type CVATSRun struct {
	ID             string         `json:"id"`
	CVID           string         `json:"cv_id"`
	JobTitle       string         `json:"job_title,omitempty"`
	JobDescription string         `json:"job_description"`
	Score          int            `json:"score"`
	Feedback       map[string]any `json:"feedback"`
	CreatedAt      time.Time      `json:"created_at"`
}

// ─── Translate ───────────────────────────────────────────────────────────

type CVTranslateRequest struct {
	TargetLocale string `json:"target_locale" binding:"required"`
}

// ─── Export ──────────────────────────────────────────────────────────────

// CVExportRequest picks the output format. TemplateID is optional —
// when omitted the session's saved template is used.
type CVExportRequest struct {
	Format     string `json:"format" binding:"required"` // pdf | docx | txt
	TemplateID string `json:"template_id,omitempty"`
}
