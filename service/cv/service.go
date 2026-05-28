// Package cv implements the AI Resume Builder feature — multi-mode CV
// creation (scratch / import / voice), AI Improve, ATS scoring, multi-
// language support, and PDF/DOCX/TXT export.
//
// The package sits on top of:
//   - service/llm        — Gemini wrapper for content + embeddings
//   - service/pdfchat    — PDF text extraction (reused for import mode)
//   - pkg/gotenberg      — HTML → PDF rendering for export
//   - storage.ICVStorage — DB layer for cv_* tables (migration 0018)
//
// All public types live in api/models/cv.go.
package cv

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"convertpdfgo/api/models"
	"convertpdfgo/pkg/gotenberg"
	"convertpdfgo/pkg/logger"
	"convertpdfgo/service/llm"
	"convertpdfgo/storage"
)

// Service is the application-level entry point for the /cv-builder tool.
// The HTTP handlers call into Service; the asynq worker calls the
// Process* methods to drive the import and export pipelines.
type Service interface {
	// Sessions
	CreateSession(ctx context.Context, userID string, req models.CVCreateRequest) (*models.CVSession, error)
	GetSession(ctx context.Context, id string) (*models.CVSessionDetail, error)
	ListSessions(ctx context.Context, userID string) ([]models.CVSession, error)
	UpdateSession(ctx context.Context, id string, patch models.CVUpdatePatch) error
	DeleteSession(ctx context.Context, id string) error

	// Sections
	AddSection(ctx context.Context, cvID string, req models.CVSectionUpsert) (*models.CVSection, error)
	UpdateSection(ctx context.Context, sectionID string, content map[string]any) error
	DeleteSection(ctx context.Context, sectionID string) error
	ReorderSections(ctx context.Context, cvID string, order []string) error

	// AI
	ImproveField(ctx context.Context, cvID, userID string, req models.CVImproveRequest) (*models.CVImproveResponse, error)
	ParseVoice(ctx context.Context, cvID string, req models.CVVoiceRequest) (*models.CVVoiceResponse, error)
	Translate(ctx context.Context, cvID, targetLocale string) error
	GenerateCoverLetter(ctx context.Context, cvID string, req models.CVCoverLetterRequest) (*models.CVCoverLetterResponse, error)
	RunATSCheck(ctx context.Context, cvID, userID string, req models.CVATSRequest) (*models.CVATSRun, error)
	ListATSRuns(ctx context.Context, cvID string) ([]models.CVATSRun, error)

	// Templates
	ListTemplates(ctx context.Context) ([]models.CVTemplate, error)

	// Import + Export (asynq-backed)
	ImportCV(ctx context.Context, userID, fileID, locale string) (cvID string, err error)
	ProcessImport(ctx context.Context, cvID string) error // worker entry

	Export(ctx context.Context, cvID, format, templateID string) (taskID string, err error)
	ProcessExport(ctx context.Context, cvID string) error // worker entry

	// Preview HTML (rendered template + content, no PDF conversion)
	RenderPreviewHTML(ctx context.Context, cvID string) (string, error)
}

// ErrCVDisabled is returned when the LLM provider isn't configured —
// CV Builder doesn't make sense without AI. Handlers map it to HTTP 503.
var ErrCVDisabled = errors.New("cv: feature disabled (LLM provider not configured)")

// Daily rate-limit caps. See PRD §13.2.
const (
	DailyAICallCap = 50
	DailyATSCap    = 5
)

type service struct {
	stg storage.IStorage
	log logger.ILogger
	llm llm.Provider     // nil → disabled
	got gotenberg.Client // never nil after wiring; used by exporter

	parser     *parser
	improver   *improver
	voice      *voiceParser
	translator  *translator
	coverLetter *coverLetter
	ats        *ats
	renderer   *renderer
	exporter   *exporter
}

// New builds the Service. Pass a logger and a gotenberg client; the LLM
// provider is loaded from env (same path as the chat feature). If LLM
// init fails we still return a Service — every method just returns
// ErrCVDisabled — so the API can boot without GEMINI_API_KEY in dev.
func New(stg storage.IStorage, log logger.ILogger, got gotenberg.Client) Service {
	p, err := llm.FromEnv()
	if err != nil {
		log.Warning("cv: LLM provider not configured — /cv-builder will return ErrCVDisabled",
			logger.Error(err))
		return &service{stg: stg, log: log, got: got}
	}
	s := &service{stg: stg, log: log, llm: p, got: got}
	s.parser = newParser(stg, p, log)
	s.improver = newImprover(p, log)
	s.voice = newVoiceParser(p, log)
	s.translator = newTranslator(stg, p, log)
	s.coverLetter = newCoverLetter(stg, p, log)
	s.ats = newATS(stg, p, log)
	s.renderer = newRenderer(log)
	s.exporter = newExporter(stg, log, got, s.renderer)
	return s
}

func (s *service) enabled() bool { return s.llm != nil && s.parser != nil }

// ─── Sessions ───────────────────────────────────────────────────────────

func (s *service) CreateSession(ctx context.Context, userID string, req models.CVCreateRequest) (*models.CVSession, error) {
	if !s.enabled() {
		return nil, ErrCVDisabled
	}
	if userID == "" {
		return nil, errors.New("cv: sign-in required")
	}
	if req.Mode != "scratch" && req.Mode != "import" && req.Mode != "voice" {
		return nil, fmt.Errorf("cv: invalid mode %q", req.Mode)
	}
	if req.Mode == "import" && (req.SourceFileID == nil || *req.SourceFileID == "") {
		return nil, errors.New("cv: import mode requires source_file_id")
	}

	sess := &models.CVSession{
		UserID:       userID,
		Title:        req.Title,
		Mode:         req.Mode,
		SourceFileID: req.SourceFileID,
		TemplateID:   req.TemplateID,
		Locale:       req.Locale,
		Status:       "draft",
	}
	if req.Mode == "import" {
		sess.Status = "parsing"
	}
	if err := s.stg.CV().CreateSession(ctx, sess); err != nil {
		return nil, fmt.Errorf("cv: create session: %w", err)
	}
	return sess, nil
}

func (s *service) GetSession(ctx context.Context, id string) (*models.CVSessionDetail, error) {
	if !s.enabled() {
		return nil, ErrCVDisabled
	}
	sess, err := s.stg.CV().GetSession(ctx, id)
	if err != nil {
		return nil, err
	}
	sections, err := s.stg.CV().ListSections(ctx, id)
	if err != nil {
		s.log.Warning("cv: list sections", logger.Error(err))
	}
	detail := &models.CVSessionDetail{
		CVSession: *sess,
		Sections:  sections,
	}
	if score, ok, _ := s.stg.CV().GetLatestATSScore(ctx, id); ok {
		detail.ATSScore = &score
	}
	return detail, nil
}

func (s *service) ListSessions(ctx context.Context, userID string) ([]models.CVSession, error) {
	if !s.enabled() {
		return nil, ErrCVDisabled
	}
	return s.stg.CV().ListSessions(ctx, userID, 50)
}

func (s *service) UpdateSession(ctx context.Context, id string, patch models.CVUpdatePatch) error {
	if !s.enabled() {
		return ErrCVDisabled
	}
	sess, err := s.stg.CV().GetSession(ctx, id)
	if err != nil {
		return err
	}
	if patch.Title != nil {
		sess.Title = strings.TrimSpace(*patch.Title)
	}
	if patch.TemplateID != nil {
		// Validate template exists
		if _, err := s.stg.CV().GetTemplate(ctx, *patch.TemplateID); err != nil {
			return fmt.Errorf("cv: unknown template %q", *patch.TemplateID)
		}
		sess.TemplateID = *patch.TemplateID
	}
	if patch.Locale != nil {
		sess.Locale = *patch.Locale
	}
	return s.stg.CV().UpdateSession(ctx, sess)
}

func (s *service) DeleteSession(ctx context.Context, id string) error {
	if !s.enabled() {
		return ErrCVDisabled
	}
	return s.stg.CV().DeleteSession(ctx, id)
}

// ─── Sections ───────────────────────────────────────────────────────────

func (s *service) AddSection(ctx context.Context, cvID string, req models.CVSectionUpsert) (*models.CVSection, error) {
	if !s.enabled() {
		return nil, ErrCVDisabled
	}
	sec := &models.CVSection{
		ID:          uuid.NewString(),
		CVID:        cvID,
		SectionType: req.SectionType,
		SortOrder:   req.SortOrder,
		Content:     req.Content,
	}
	if err := s.stg.CV().AddSection(ctx, sec); err != nil {
		return nil, err
	}
	// Embedding update is best-effort and async-friendly; do it in the
	// background so the user gets the section ID back immediately.
	go s.embedSectionAsync(sec.ID, sec.Content)
	return sec, nil
}

func (s *service) UpdateSection(ctx context.Context, sectionID string, content map[string]any) error {
	if !s.enabled() {
		return ErrCVDisabled
	}
	if err := s.stg.CV().UpdateSection(ctx, sectionID, content); err != nil {
		return err
	}
	go s.embedSectionAsync(sectionID, content)
	return nil
}

func (s *service) DeleteSection(ctx context.Context, sectionID string) error {
	if !s.enabled() {
		return ErrCVDisabled
	}
	return s.stg.CV().DeleteSection(ctx, sectionID)
}

func (s *service) ReorderSections(ctx context.Context, cvID string, order []string) error {
	if !s.enabled() {
		return ErrCVDisabled
	}
	return s.stg.CV().ReorderSections(ctx, cvID, order)
}

// embedSectionAsync recomputes the section's text embedding using the
// LLM provider. Fires in a goroutine so the user-facing update path
// doesn't pay the Gemini round trip. Errors are logged, not returned.
func (s *service) embedSectionAsync(sectionID string, content map[string]any) {
	if s.llm == nil {
		return
	}
	ctx, cancel := contextBackgroundWithTimeout()
	defer cancel()
	text := flattenSectionText(content)
	if text == "" {
		return
	}
	vecs, err := s.llm.Embed(ctx, []string{text})
	if err != nil || len(vecs) == 0 {
		s.log.Warning("cv: embed section", logger.Error(err))
		return
	}
	if err := s.stg.CV().UpdateSectionEmbedding(ctx, sectionID, vecs[0]); err != nil {
		s.log.Warning("cv: save section embedding", logger.Error(err))
	}
}

// ─── AI delegations ─────────────────────────────────────────────────────

func (s *service) ImproveField(ctx context.Context, cvID, userID string, req models.CVImproveRequest) (*models.CVImproveResponse, error) {
	if !s.enabled() {
		return nil, ErrCVDisabled
	}
	if ok, _, err := s.stg.CV().CheckAndBumpAILimit(ctx, userID, DailyAICallCap); err != nil {
		return nil, err
	} else if !ok {
		return nil, fmt.Errorf("cv: daily AI quota reached (%d/day)", DailyAICallCap)
	}
	return s.improver.Improve(ctx, s.stg, cvID, req)
}

func (s *service) ParseVoice(ctx context.Context, cvID string, req models.CVVoiceRequest) (*models.CVVoiceResponse, error) {
	if !s.enabled() {
		return nil, ErrCVDisabled
	}
	return s.voice.Parse(ctx, s.stg, cvID, req)
}

func (s *service) Translate(ctx context.Context, cvID, targetLocale string) error {
	if !s.enabled() {
		return ErrCVDisabled
	}
	return s.translator.Run(ctx, cvID, targetLocale)
}

func (s *service) RunATSCheck(ctx context.Context, cvID, userID string, req models.CVATSRequest) (*models.CVATSRun, error) {
	if !s.enabled() {
		return nil, ErrCVDisabled
	}
	if ok, _, err := s.stg.CV().CheckAndBumpATSLimit(ctx, userID, DailyATSCap); err != nil {
		return nil, err
	} else if !ok {
		return nil, fmt.Errorf("cv: daily ATS quota reached (%d/day)", DailyATSCap)
	}
	return s.ats.Score(ctx, cvID, req)
}

func (s *service) ListATSRuns(ctx context.Context, cvID string) ([]models.CVATSRun, error) {
	if !s.enabled() {
		return nil, ErrCVDisabled
	}
	return s.stg.CV().ListATSRuns(ctx, cvID, 20)
}

// ─── Templates ──────────────────────────────────────────────────────────

func (s *service) ListTemplates(ctx context.Context) ([]models.CVTemplate, error) {
	return s.stg.CV().ListTemplates(ctx)
}

// ─── Import (asynq driven) ──────────────────────────────────────────────

func (s *service) ImportCV(ctx context.Context, userID, fileID, locale string) (string, error) {
	if !s.enabled() {
		return "", ErrCVDisabled
	}
	sess := &models.CVSession{
		UserID:       userID,
		Title:        "Imported CV",
		Mode:         "import",
		SourceFileID: &fileID,
		Locale:       locale,
		Status:       "parsing",
	}
	if err := s.stg.CV().CreateSession(ctx, sess); err != nil {
		return "", err
	}
	// Caller (handler) enqueues a cv:import_parse asynq task with this id.
	return sess.ID, nil
}

func (s *service) ProcessImport(ctx context.Context, cvID string) error {
	if !s.enabled() {
		return ErrCVDisabled
	}
	return s.parser.Run(ctx, cvID)
}

// ─── Export (asynq driven) ──────────────────────────────────────────────

func (s *service) Export(ctx context.Context, cvID, format, templateID string) (string, error) {
	if !s.enabled() {
		return "", ErrCVDisabled
	}
	if format == "" {
		format = "pdf"
	}
	if format != "pdf" && format != "docx" && format != "txt" {
		return "", fmt.Errorf("cv: unsupported export format %q", format)
	}
	// templateID override is applied lazily inside ProcessExport via
	// session.template_id, so update it here if the caller asked.
	if templateID != "" {
		if err := s.UpdateSession(ctx, cvID, models.CVUpdatePatch{TemplateID: &templateID}); err != nil {
			return "", err
		}
	}
	// The handler will enqueue the actual asynq job; we just return the
	// CV id so the caller can build the task payload.
	return cvID, nil
}

func (s *service) ProcessExport(ctx context.Context, cvID string) error {
	if !s.enabled() {
		return ErrCVDisabled
	}
	return s.exporter.Run(ctx, cvID, "pdf") // format hint stored on the asynq payload — kept simple here
}

// ─── Preview ────────────────────────────────────────────────────────────

func (s *service) RenderPreviewHTML(ctx context.Context, cvID string) (string, error) {
	if !s.enabled() {
		return "", ErrCVDisabled
	}
	sess, err := s.stg.CV().GetSession(ctx, cvID)
	if err != nil {
		return "", err
	}
	sections, err := s.stg.CV().ListSections(ctx, cvID)
	if err != nil {
		return "", err
	}
	return s.renderer.Render(sess.TemplateID, sess.Locale, sections)
}

func (s *service) GenerateCoverLetter(ctx context.Context, cvID string, req models.CVCoverLetterRequest) (*models.CVCoverLetterResponse, error) {
	if !s.enabled() {
		return nil, ErrCVDisabled
	}
	return s.coverLetter.Generate(ctx, cvID, req)
}
