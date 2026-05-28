package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"convertpdfgo/api/models"
	"convertpdfgo/pkg/logger"
)

// cvRepo backs the ICVStorage interface — all cv_* tables share a single
// repo type because the relationships between them are tight (every CRUD
// call touches at least two of sessions / sections / ats_runs / history)
// and fanning them out across multiple repo types would mean shuttling
// the pgxpool to each one anyway.
type cvRepo struct {
	db  *pgxpool.Pool
	log logger.ILogger
}

func NewCVRepo(db *pgxpool.Pool, log logger.ILogger) *cvRepo {
	return &cvRepo{db: db, log: log}
}

// ─── Sessions ───────────────────────────────────────────────────────────

func (r *cvRepo) CreateSession(ctx context.Context, s *models.CVSession) error {
	if s.ID == "" {
		s.ID = uuid.NewString()
	}
	if s.TemplateID == "" {
		s.TemplateID = "modern"
	}
	if s.Locale == "" {
		s.Locale = "en"
	}
	if s.Status == "" {
		s.Status = "draft"
	}
	if s.Title == "" {
		s.Title = "Untitled CV"
	}
	now := time.Now()
	s.CreatedAt = now
	s.UpdatedAt = now

	_, err := r.db.Exec(ctx, `
		INSERT INTO cv_sessions
		    (id, user_id, title, mode, source_file_id, template_id, locale, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $9)
	`, s.ID, s.UserID, s.Title, s.Mode, s.SourceFileID, s.TemplateID, s.Locale, s.Status, now)
	if err != nil {
		r.log.Error("cvRepo.CreateSession", logger.Error(err))
	}
	return err
}

func (r *cvRepo) GetSession(ctx context.Context, id string) (*models.CVSession, error) {
	var s models.CVSession
	var parseErr *string
	err := r.db.QueryRow(ctx, `
		SELECT id, user_id, title, mode, source_file_id, template_id, locale,
		       status, parse_error, last_pdf_id, created_at, updated_at
		FROM cv_sessions WHERE id = $1
	`, id).Scan(&s.ID, &s.UserID, &s.Title, &s.Mode, &s.SourceFileID, &s.TemplateID,
		&s.Locale, &s.Status, &parseErr, &s.LastPDFID, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("cv session not found")
		}
		return nil, err
	}
	if parseErr != nil {
		s.ParseError = *parseErr
	}
	return &s, nil
}

func (r *cvRepo) UpdateSession(ctx context.Context, s *models.CVSession) error {
	_, err := r.db.Exec(ctx, `
		UPDATE cv_sessions
		SET title = $1, template_id = $2, locale = $3, updated_at = NOW()
		WHERE id = $4
	`, s.Title, s.TemplateID, s.Locale, s.ID)
	return err
}

func (r *cvRepo) UpdateSessionStatus(ctx context.Context, id, status, parseError string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE cv_sessions
		SET status = $1, parse_error = NULLIF($2, ''), updated_at = NOW()
		WHERE id = $3
	`, status, parseError, id)
	return err
}

func (r *cvRepo) UpdateSessionLastPDF(ctx context.Context, id, fileID string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE cv_sessions SET last_pdf_id = $1, updated_at = NOW() WHERE id = $2
	`, fileID, id)
	return err
}

func (r *cvRepo) ListSessions(ctx context.Context, userID string, limit int) ([]models.CVSession, error) {
	if limit <= 0 || limit > 100 {
		limit = 30
	}
	rows, err := r.db.Query(ctx, `
		SELECT id, user_id, title, mode, source_file_id, template_id, locale,
		       status, COALESCE(parse_error,''), last_pdf_id, created_at, updated_at
		FROM cv_sessions
		WHERE user_id = $1
		ORDER BY updated_at DESC
		LIMIT $2
	`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]models.CVSession, 0, limit)
	for rows.Next() {
		var s models.CVSession
		if err := rows.Scan(&s.ID, &s.UserID, &s.Title, &s.Mode, &s.SourceFileID,
			&s.TemplateID, &s.Locale, &s.Status, &s.ParseError, &s.LastPDFID,
			&s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (r *cvRepo) DeleteSession(ctx context.Context, id string) error {
	_, err := r.db.Exec(ctx, `DELETE FROM cv_sessions WHERE id = $1`, id)
	return err
}

// ─── Sections ───────────────────────────────────────────────────────────

func (r *cvRepo) AddSection(ctx context.Context, s *models.CVSection) error {
	if s.ID == "" {
		s.ID = uuid.NewString()
	}
	contentJSON, err := json.Marshal(s.Content)
	if err != nil {
		return fmt.Errorf("marshal content: %w", err)
	}
	now := time.Now()
	s.CreatedAt = now
	s.UpdatedAt = now
	_, err = r.db.Exec(ctx, `
		INSERT INTO cv_sections (id, cv_id, section_type, sort_order, content, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $6)
	`, s.ID, s.CVID, s.SectionType, s.SortOrder, contentJSON, now)
	return err
}

func (r *cvRepo) UpdateSection(ctx context.Context, id string, content map[string]any) error {
	contentJSON, err := json.Marshal(content)
	if err != nil {
		return fmt.Errorf("marshal content: %w", err)
	}
	_, err = r.db.Exec(ctx, `
		UPDATE cv_sections SET content = $1, updated_at = NOW() WHERE id = $2
	`, contentJSON, id)
	return err
}

func (r *cvRepo) UpdateSectionEmbedding(ctx context.Context, id string, embedding []float32) error {
	_, err := r.db.Exec(ctx, `
		UPDATE cv_sections SET embedding = $1::vector, updated_at = NOW() WHERE id = $2
	`, vectorToLiteral(embedding), id)
	return err
}

func (r *cvRepo) DeleteSection(ctx context.Context, id string) error {
	_, err := r.db.Exec(ctx, `DELETE FROM cv_sections WHERE id = $1`, id)
	return err
}

func (r *cvRepo) ListSections(ctx context.Context, cvID string) ([]models.CVSection, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, cv_id, section_type, sort_order, content, created_at, updated_at
		FROM cv_sections
		WHERE cv_id = $1
		ORDER BY sort_order, created_at
	`, cvID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.CVSection{}
	for rows.Next() {
		var s models.CVSection
		var contentBytes []byte
		if err := rows.Scan(&s.ID, &s.CVID, &s.SectionType, &s.SortOrder, &contentBytes,
			&s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, err
		}
		if len(contentBytes) > 0 {
			_ = json.Unmarshal(contentBytes, &s.Content)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// ReorderSections updates sort_order in a single transaction so the
// editor can drag-drop and persist with one round trip.
func (r *cvRepo) ReorderSections(ctx context.Context, cvID string, order []string) error {
	if len(order) == 0 {
		return nil
	}
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	for i, id := range order {
		if _, err := tx.Exec(ctx, `
			UPDATE cv_sections SET sort_order = $1, updated_at = NOW()
			WHERE id = $2 AND cv_id = $3
		`, i, id, cvID); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// ReplaceSections is the import path — wipe whatever was there and bulk
// insert the parser's output. Runs inside a single transaction so a
// partial parse doesn't leave the session in a torn state.
func (r *cvRepo) ReplaceSections(ctx context.Context, cvID string, sections []models.CVSection) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `DELETE FROM cv_sections WHERE cv_id = $1`, cvID); err != nil {
		return err
	}
	for i, s := range sections {
		contentJSON, err := json.Marshal(s.Content)
		if err != nil {
			return err
		}
		id := s.ID
		if id == "" {
			id = uuid.NewString()
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO cv_sections (id, cv_id, section_type, sort_order, content)
			VALUES ($1, $2, $3, $4, $5)
		`, id, cvID, s.SectionType, i, contentJSON); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// ─── Templates ──────────────────────────────────────────────────────────

func (r *cvRepo) ListTemplates(ctx context.Context) ([]models.CVTemplate, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, name, COALESCE(description,''), is_ats_friendly, COALESCE(preview_url,''),
		       COALESCE(category,''), sort_order
		FROM cv_templates
		WHERE is_public = TRUE
		ORDER BY sort_order, name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.CVTemplate{}
	for rows.Next() {
		var t models.CVTemplate
		if err := rows.Scan(&t.ID, &t.Name, &t.Description, &t.IsATSFriendly,
			&t.PreviewURL, &t.Category, &t.SortOrder); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (r *cvRepo) GetTemplate(ctx context.Context, id string) (*models.CVTemplate, error) {
	var t models.CVTemplate
	err := r.db.QueryRow(ctx, `
		SELECT id, name, COALESCE(description,''), is_ats_friendly, COALESCE(preview_url,''),
		       COALESCE(category,''), sort_order
		FROM cv_templates WHERE id = $1
	`, id).Scan(&t.ID, &t.Name, &t.Description, &t.IsATSFriendly,
		&t.PreviewURL, &t.Category, &t.SortOrder)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("template not found")
		}
		return nil, err
	}
	return &t, nil
}

// ─── ATS ────────────────────────────────────────────────────────────────

func (r *cvRepo) SaveATSRun(ctx context.Context, run *models.CVATSRun) error {
	if run.ID == "" {
		run.ID = uuid.NewString()
	}
	feedbackJSON, err := json.Marshal(run.Feedback)
	if err != nil {
		return err
	}
	if run.CreatedAt.IsZero() {
		run.CreatedAt = time.Now()
	}
	_, err = r.db.Exec(ctx, `
		INSERT INTO cv_ats_runs (id, cv_id, job_title, job_description, score, feedback, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, run.ID, run.CVID, run.JobTitle, run.JobDescription, run.Score, feedbackJSON, run.CreatedAt)
	return err
}

func (r *cvRepo) GetLatestATSScore(ctx context.Context, cvID string) (int, bool, error) {
	var score int
	err := r.db.QueryRow(ctx, `
		SELECT score FROM cv_ats_runs
		WHERE cv_id = $1 ORDER BY created_at DESC LIMIT 1
	`, cvID).Scan(&score)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, false, nil
		}
		return 0, false, err
	}
	return score, true, nil
}

func (r *cvRepo) ListATSRuns(ctx context.Context, cvID string, limit int) ([]models.CVATSRun, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	rows, err := r.db.Query(ctx, `
		SELECT id, cv_id, COALESCE(job_title,''), job_description, score, feedback, created_at
		FROM cv_ats_runs
		WHERE cv_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`, cvID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.CVATSRun{}
	for rows.Next() {
		var run models.CVATSRun
		var fbBytes []byte
		if err := rows.Scan(&run.ID, &run.CVID, &run.JobTitle, &run.JobDescription,
			&run.Score, &fbBytes, &run.CreatedAt); err != nil {
			return nil, err
		}
		if len(fbBytes) > 0 {
			_ = json.Unmarshal(fbBytes, &run.Feedback)
		}
		out = append(out, run)
	}
	return out, rows.Err()
}

// ─── AI history ─────────────────────────────────────────────────────────

func (r *cvRepo) LogAIImprove(ctx context.Context, cvID, sectionID, fieldPath, prompt string, suggestions any, tokensIn, tokensOut int) error {
	sugJSON, err := json.Marshal(suggestions)
	if err != nil {
		return err
	}
	var sectIDArg any
	if sectionID != "" {
		sectIDArg = sectionID
	}
	_, err = r.db.Exec(ctx, `
		INSERT INTO cv_ai_history (id, cv_id, section_id, field_path, prompt, suggestions, tokens_in, tokens_out)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, uuid.NewString(), cvID, sectIDArg, fieldPath, prompt, sugJSON, tokensIn, tokensOut)
	if err != nil {
		r.log.Warning("cvRepo.LogAIImprove", logger.Error(err))
	}
	return err
}

// ─── Voice ──────────────────────────────────────────────────────────────

func (r *cvRepo) SaveVoiceSegment(ctx context.Context, cvID, transcript, parsedSection string, parsedContent map[string]any, locale string) error {
	pcJSON, _ := json.Marshal(parsedContent)
	var sectArg any
	if parsedSection != "" {
		sectArg = parsedSection
	}
	if locale == "" {
		locale = "en"
	}
	_, err := r.db.Exec(ctx, `
		INSERT INTO cv_voice_segments (id, cv_id, transcript, parsed_section, parsed_content, locale)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, uuid.NewString(), cvID, transcript, sectArg, pcJSON, locale)
	return err
}

// ─── Rate limits ────────────────────────────────────────────────────────

// CheckAndBumpAILimit returns true + the new count when the user is still
// under cap; false + current count when they're over. Resets the daily
// window the first time the user calls past midnight.
func (r *cvRepo) CheckAndBumpAILimit(ctx context.Context, userID string, dailyCap int) (bool, int, error) {
	return r.checkAndBump(ctx, userID, "ai_calls_today", dailyCap)
}

func (r *cvRepo) CheckAndBumpATSLimit(ctx context.Context, userID string, dailyCap int) (bool, int, error) {
	return r.checkAndBump(ctx, userID, "ats_runs_today", dailyCap)
}

func (r *cvRepo) checkAndBump(ctx context.Context, userID, col string, dailyCap int) (bool, int, error) {
	// Whitelist the column name to avoid SQL injection — the public
	// methods only pass two values, so a quick lookup is safer than
	// composing the SQL string from a raw arg.
	if col != "ai_calls_today" && col != "ats_runs_today" {
		return false, 0, fmt.Errorf("invalid counter column %q", col)
	}
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return false, 0, err
	}
	defer tx.Rollback(ctx)

	// Ensure a row exists.
	if _, err := tx.Exec(ctx, `
		INSERT INTO cv_rate_limits (user_id) VALUES ($1)
		ON CONFLICT (user_id) DO NOTHING
	`, userID); err != nil {
		return false, 0, err
	}

	// Reset the window if it crosses calendar day.
	if _, err := tx.Exec(ctx, `
		UPDATE cv_rate_limits
		SET ai_calls_today = 0, ats_runs_today = 0, window_start = NOW()
		WHERE user_id = $1 AND window_start::date < NOW()::date
	`, userID); err != nil {
		return false, 0, err
	}

	// Read current count.
	var used int
	q := strings.NewReplacer("__COL__", col).Replace(
		`SELECT __COL__ FROM cv_rate_limits WHERE user_id = $1 FOR UPDATE`,
	)
	if err := tx.QueryRow(ctx, q, userID).Scan(&used); err != nil {
		return false, 0, err
	}
	if used >= dailyCap {
		return false, used, tx.Commit(ctx)
	}

	// Bump.
	bumpSQL := strings.NewReplacer("__COL__", col).Replace(
		`UPDATE cv_rate_limits SET __COL__ = __COL__ + 1 WHERE user_id = $1`,
	)
	if _, err := tx.Exec(ctx, bumpSQL, userID); err != nil {
		return false, 0, err
	}
	return true, used + 1, tx.Commit(ctx)
}
