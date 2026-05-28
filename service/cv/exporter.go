package cv

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"

	"convertpdfgo/api/models"
	"convertpdfgo/pkg/gotenberg"
	"convertpdfgo/pkg/logger"
	"convertpdfgo/storage"
)

// exporter renders the CV to a downloadable file.
//
// PDF — render the HTML template, ship to Gotenberg's chromium endpoint.
// DOCX — render → PDF (Gotenberg) → libreoffice headless convert to docx.
// TXT — skip rendering entirely; collapse sections into a plaintext
//
//	resume that ATS scanners and email recruiters love.
//
// Each export creates a fresh file row (so /file/<id>/download serves
// it) and updates cv_sessions.last_pdf_id when the format is PDF so the
// editor's preview iframe can point at it.
type exporter struct {
	stg storage.IStorage
	log logger.ILogger
	got gotenberg.Client
	rdr *renderer
}

func newExporter(stg storage.IStorage, log logger.ILogger, got gotenberg.Client, rdr *renderer) *exporter {
	return &exporter{stg: stg, log: log, got: got, rdr: rdr}
}

// Run materialises the CV for the given format. Returns nil on success;
// the asynq worker treats any returned error as task failure.
//
// `format` is one of "pdf", "docx", "txt". When the format is empty we
// default to PDF — that's the case most callers want.
func (e *exporter) Run(ctx context.Context, cvID, format string) error {
	if format == "" {
		format = "pdf"
	}
	sess, err := e.stg.CV().GetSession(ctx, cvID)
	if err != nil {
		return fmt.Errorf("export: get session: %w", err)
	}
	sections, err := e.stg.CV().ListSections(ctx, cvID)
	if err != nil {
		return fmt.Errorf("export: list sections: %w", err)
	}
	if len(sections) == 0 {
		return fmt.Errorf("export: CV has no content")
	}

	stem := filenameStem(sess.Title)
	outID := uuid.NewString()
	outDir := "uploads"
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("export: mkdir: %w", err)
	}

	switch strings.ToLower(format) {
	case "pdf":
		return e.exportPDF(ctx, sess, sections, outID, stem, outDir)
	case "docx":
		return e.exportDOCX(ctx, sess, sections, outID, stem, outDir)
	case "txt":
		return e.exportTXT(ctx, sess, sections, outID, stem, outDir)
	}
	return fmt.Errorf("export: unsupported format %q", format)
}

// ─── PDF ────────────────────────────────────────────────────────────────

func (e *exporter) exportPDF(ctx context.Context, sess *models.CVSession, sections []models.CVSection, outID, stem, outDir string) error {
	html, err := e.rdr.Render(sess.TemplateID, sess.Locale, sections)
	if err != nil {
		return fmt.Errorf("export pdf: render: %w", err)
	}
	outPath := filepath.Join(outDir, outID+".pdf")
	if _, err := e.got.HTMLStringToPDF(ctx, html, outPath); err != nil {
		return fmt.Errorf("export pdf: gotenberg: %w", err)
	}
	if err := e.registerFile(ctx, sess, outID, outPath, stem+".pdf", "application/pdf"); err != nil {
		return err
	}
	// Cache the file_id on the session so the live preview can short-
	// circuit to it instead of re-rendering on every reload.
	if err := e.stg.CV().UpdateSessionLastPDF(ctx, sess.ID, outID); err != nil {
		e.log.Warning("cv exporter: update last_pdf_id", logger.Error(err))
	}
	return nil
}

// ─── DOCX (PDF → DOCX via libreoffice) ──────────────────────────────────

func (e *exporter) exportDOCX(ctx context.Context, sess *models.CVSession, sections []models.CVSection, outID, stem, outDir string) error {
	// First render PDF, then hand it to libreoffice for the .docx conversion.
	// LibreOffice handles PDF→DOCX cleanly enough that we don't need a
	// separate pdf2docx tool.
	html, err := e.rdr.Render(sess.TemplateID, sess.Locale, sections)
	if err != nil {
		return fmt.Errorf("export docx: render: %w", err)
	}
	pdfTmp := filepath.Join(outDir, outID+".pdf.tmp")
	if _, err := e.got.HTMLStringToPDF(ctx, html, pdfTmp); err != nil {
		return fmt.Errorf("export docx: gotenberg: %w", err)
	}
	defer os.Remove(pdfTmp)
	// LibreOffice writes the converted file next to the source by default.
	// Run via gotenberg's PDFToWord which already shells out to libreoffice.
	docxPath := filepath.Join(outDir, outID+".docx")
	if _, err := e.got.PDFToWord(ctx, pdfTmp, docxPath); err != nil {
		return fmt.Errorf("export docx: pdf→word: %w", err)
	}
	return e.registerFile(ctx, sess, outID, docxPath, stem+".docx",
		"application/vnd.openxmlformats-officedocument.wordprocessingml.document")
}

// ─── TXT (plain text resume, ATS-friendly) ──────────────────────────────

func (e *exporter) exportTXT(ctx context.Context, sess *models.CVSession, sections []models.CVSection, outID, stem, outDir string) error {
	txt := renderPlainText(sections)
	outPath := filepath.Join(outDir, outID+".txt")
	if err := os.WriteFile(outPath, []byte(txt), 0o600); err != nil {
		return fmt.Errorf("export txt: write: %w", err)
	}
	return e.registerFile(ctx, sess, outID, outPath, stem+".txt", "text/plain")
}

// renderPlainText flattens the CV into a markdown-flavoured plaintext
// representation. ATS scanners and email recruiters can both read it
// without choking on layout artifacts.
func renderPlainText(sections []models.CVSection) string {
	var sb strings.Builder
	view := buildViewModel("en", sections)

	if view.Personal.Name != "" {
		sb.WriteString(view.Personal.Name)
		sb.WriteByte('\n')
	}
	if view.Personal.Title != "" {
		sb.WriteString(view.Personal.Title)
		sb.WriteByte('\n')
	}
	{
		var contact []string
		for _, v := range []string{view.Personal.Email, view.Personal.Phone, view.Personal.Location, view.Personal.LinkedIn} {
			if v != "" {
				contact = append(contact, v)
			}
		}
		if len(contact) > 0 {
			sb.WriteString(strings.Join(contact, " · "))
			sb.WriteByte('\n')
		}
	}
	sb.WriteByte('\n')

	section := func(name, body string) {
		body = strings.TrimSpace(body)
		if body == "" {
			return
		}
		sb.WriteString("== ")
		sb.WriteString(strings.ToUpper(name))
		sb.WriteString(" ==\n")
		sb.WriteString(body)
		sb.WriteString("\n\n")
	}

	if view.Summary != "" {
		section("Summary", view.Summary)
	}
	if len(view.Experience) > 0 {
		var sub strings.Builder
		for _, e := range view.Experience {
			fmt.Fprintf(&sub, "%s · %s · %s\n", e.Title, e.Company, e.Period)
			for _, b := range e.Bullets {
				fmt.Fprintf(&sub, "  - %s\n", b)
			}
			sub.WriteByte('\n')
		}
		section("Experience", sub.String())
	}
	if len(view.Education) > 0 {
		var sub strings.Builder
		for _, ed := range view.Education {
			fmt.Fprintf(&sub, "%s · %s · %s\n", ed.Degree, ed.School, ed.Period)
		}
		section("Education", sub.String())
	}
	if len(view.Skills.Technical)+len(view.Skills.Soft) > 0 {
		var sub strings.Builder
		if len(view.Skills.Technical) > 0 {
			sub.WriteString("Technical: ")
			sub.WriteString(strings.Join(view.Skills.Technical, ", "))
			sub.WriteByte('\n')
		}
		if len(view.Skills.Soft) > 0 {
			sub.WriteString("Soft: ")
			sub.WriteString(strings.Join(view.Skills.Soft, ", "))
			sub.WriteByte('\n')
		}
		section("Skills", sub.String())
	}
	if len(view.Languages) > 0 {
		var parts []string
		for _, l := range view.Languages {
			if l.Level != "" {
				parts = append(parts, fmt.Sprintf("%s (%s)", l.Name, l.Level))
			} else {
				parts = append(parts, l.Name)
			}
		}
		section("Languages", strings.Join(parts, ", "))
	}
	if len(view.Projects) > 0 {
		var sub strings.Builder
		for _, p := range view.Projects {
			fmt.Fprintf(&sub, "%s — %s\n", p.Name, p.Description)
		}
		section("Projects", sub.String())
	}
	if len(view.Awards) > 0 {
		var sub strings.Builder
		for _, a := range view.Awards {
			fmt.Fprintf(&sub, "%s · %s · %s\n", a.Title, a.Issuer, a.Year)
		}
		section("Awards", sub.String())
	}
	if len(view.Certs) > 0 {
		var sub strings.Builder
		for _, c := range view.Certs {
			fmt.Fprintf(&sub, "%s · %s · %s\n", c.Name, c.Issuer, c.Year)
		}
		section("Certifications", sub.String())
	}

	return strings.TrimRight(sb.String(), "\n") + "\n"
}

// ─── Helpers ────────────────────────────────────────────────────────────

// registerFile creates a files row for the just-rendered output so the
// existing /file/<id>/download endpoint can serve it without any new
// API surface area.
func (e *exporter) registerFile(ctx context.Context, sess *models.CVSession, fileID, path, displayName, mime string) error {
	fi, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("export: stat: %w", err)
	}
	userID := sess.UserID
	_, err = e.stg.File().Save(ctx, models.File{
		ID:         fileID,
		UserID:     &userID,
		FileName:   displayName,
		FilePath:   path,
		FileType:   mime,
		FileSize:   fi.Size(),
		UploadedAt: time.Now(),
	})
	if err != nil {
		_ = os.Remove(path)
		return fmt.Errorf("export: file save: %w", err)
	}
	return nil
}

// filenameStem turns the CV title into a safe filename stem.
func filenameStem(title string) string {
	title = strings.TrimSpace(title)
	if title == "" {
		title = "resume"
	}
	keep := strings.Builder{}
	for _, r := range title {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			keep.WriteRune(r)
		case r == ' ' || r == '-' || r == '_':
			keep.WriteByte('_')
		}
	}
	out := strings.Trim(keep.String(), "_")
	if out == "" {
		return "resume"
	}
	if len(out) > 60 {
		out = out[:60]
	}
	return out
}
