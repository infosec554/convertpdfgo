package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"

	"convertpdfgo/api/models"
	"convertpdfgo/pkg/logger"
	"convertpdfgo/service/llm"
	"convertpdfgo/storage"
)

// XLSXBuilderService creates AI-generated Excel workbooks from a topic.
// Same shape as PPTXBuilderService — Gemini returns a JSON sheet spec,
// scripts/build_xlsx.py renders it via xlsxwriter with charts and
// formulas.
type XLSXBuilderService interface {
	Create(ctx context.Context, req models.XLSXBuilderRequest, userID *string) (string, error)
	Process(ctx context.Context, jobID string) error
	GetByID(ctx context.Context, id string) (*models.XLSXBuilderJob, error)
}

type xlsxBuilderService struct {
	stg storage.IStorage
	log logger.ILogger
	llm llm.Provider
}

func NewXLSXBuilderService(stg storage.IStorage, log logger.ILogger) XLSXBuilderService {
	p, err := llm.FromEnv()
	if err != nil {
		log.Warning("xlsx_builder: LLM provider not configured", logger.Error(err))
		return &xlsxBuilderService{stg: stg, log: log}
	}
	return &xlsxBuilderService{stg: stg, log: log, llm: p}
}

var ErrXLSXBuilderDisabled = errors.New("xlsx_builder: feature disabled (LLM provider not configured)")

func (s *xlsxBuilderService) Create(ctx context.Context, req models.XLSXBuilderRequest, userID *string) (string, error) {
	if req.SheetCount < 1 || req.SheetCount > 5 {
		req.SheetCount = 2
	}
	if req.Tone == "" {
		req.Tone = "corporate"
	}
	if req.Language == "" {
		req.Language = "en"
	}
	job := &models.XLSXBuilderJob{
		ID:           uuid.NewString(),
		UserID:       userID,
		Topic:        strings.TrimSpace(req.Topic),
		SheetCount:   req.SheetCount,
		Tone:         req.Tone,
		Language:     req.Language,
		Audience:     strings.TrimSpace(req.Audience),
		IncludeChart: req.IncludeChart,
		Status:       "pending",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	if err := s.stg.XLSXBuilder().Create(ctx, job); err != nil {
		return "", err
	}
	return job.ID, nil
}

func (s *xlsxBuilderService) Process(ctx context.Context, jobID string) error {
	job, err := s.stg.XLSXBuilder().GetByID(ctx, jobID)
	if err != nil {
		return fmt.Errorf("get job: %w", err)
	}
	job.Status = "running"
	_ = s.stg.XLSXBuilder().Update(ctx, job)

	spec, err := s.generateSpec(ctx, job)
	if err != nil {
		s.markFailed(ctx, job, fmt.Sprintf("llm: %v", err))
		return fmt.Errorf("llm spec: %w", err)
	}

	outputFileID := uuid.NewString()
	outDir := "storage/xlsx_builder"
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		s.markFailed(ctx, job, fmt.Sprintf("mkdir: %v", err))
		return err
	}
	outPath := filepath.Join(outDir, outputFileID+".xlsx")

	specPath := filepath.Join(outDir, outputFileID+".spec.json")
	specBytes, _ := json.MarshalIndent(spec, "", "  ")
	if err := os.WriteFile(specPath, specBytes, 0o644); err != nil {
		s.markFailed(ctx, job, fmt.Sprintf("write spec: %v", err))
		return err
	}
	defer os.Remove(specPath)

	ctxCmd, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctxCmd, "python3", "scripts/build_xlsx.py", specPath, outPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		s.markFailed(ctx, job, fmt.Sprintf("xlsxwriter: %s: %v", strings.TrimSpace(string(out)), err))
		return fmt.Errorf("xlsxwriter: %w (%s)", err, strings.TrimSpace(string(out)))
	}

	info, err := os.Stat(outPath)
	if err != nil {
		s.markFailed(ctx, job, fmt.Sprintf("stat: %v", err))
		return err
	}

	displayName := safeFilename(job.Topic) + ".xlsx"
	if _, err := s.stg.File().Save(ctx, models.File{
		ID:         outputFileID,
		UserID:     job.UserID,
		FileName:   displayName,
		FilePath:   outPath,
		FileType:   "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		FileSize:   info.Size(),
		UploadedAt: time.Now(),
	}); err != nil {
		s.markFailed(ctx, job, fmt.Sprintf("save file: %v", err))
		return err
	}

	job.OutputFileID = &outputFileID
	job.Status = "done"
	return s.stg.XLSXBuilder().Update(ctx, job)
}

func (s *xlsxBuilderService) markFailed(ctx context.Context, job *models.XLSXBuilderJob, errStr string) {
	job.Status = "failed"
	job.Error = errStr
	if err := s.stg.XLSXBuilder().Update(ctx, job); err != nil {
		s.log.Error("xlsx_builder: mark failed", logger.Error(err))
	}
}

func (s *xlsxBuilderService) GetByID(ctx context.Context, id string) (*models.XLSXBuilderJob, error) {
	return s.stg.XLSXBuilder().GetByID(ctx, id)
}

// xlsxSpec mirrors the JSON the python script expects.
type xlsxSpec struct {
	Title    string                   `json:"title"`
	Language string                   `json:"language"`
	Palette  map[string]string        `json:"palette"`
	Sheets   []map[string]any         `json:"sheets"`
}

func (s *xlsxBuilderService) generateSpec(ctx context.Context, job *models.XLSXBuilderJob) (*xlsxSpec, error) {
	if s.llm == nil {
		return nil, ErrXLSXBuilderDisabled
	}
	sys := buildXLSXSystemPrompt(job)
	user := buildXLSXUserPrompt(job)

	msgs := []llm.Message{{Role: llm.RoleUser, Content: user}}
	stream, err := s.llm.Chat(ctx, msgs, llm.ChatOpts{
		System:      sys,
		Temperature: 0.55,
		// Spreadsheets with realistic varied data + formulas + chart
		// regularly cross 4k tokens for the JSON spec — bump the
		// ceiling so we don't truncate mid-row.
		MaxTokens:   8192,
	})
	if err != nil {
		return nil, err
	}

	var sb strings.Builder
	for ev := range stream {
		if ev.Err != nil {
			return nil, ev.Err
		}
		sb.WriteString(ev.Delta)
	}
	raw := stripCodeFence(strings.TrimSpace(sb.String()))

	var spec xlsxSpec
	if err := json.Unmarshal([]byte(raw), &spec); err != nil {
		return nil, fmt.Errorf("parse spec: %w; got: %s", err, truncForLog(raw, 400))
	}
	if len(spec.Sheets) == 0 {
		return nil, errors.New("empty sheets")
	}
	// Safety defaults for palette
	if spec.Palette == nil {
		spec.Palette = map[string]string{}
	}
	for k, v := range map[string]string{
		"header_bg":   "#1a365d",
		"header_text": "#ffffff",
		"accent":      "#ed8936",
		"zebra":       "#f7fafc",
	} {
		if _, ok := spec.Palette[k]; !ok {
			spec.Palette[k] = v
		}
	}
	return &spec, nil
}

func buildXLSXSystemPrompt(j *models.XLSXBuilderJob) string {
	chartHint := "INCLUDE one chart on the first sheet (column or line based on data shape)."
	if !j.IncludeChart {
		chartHint = "Do NOT include any chart."
	}
	return `You are an expert spreadsheet designer who produces a JSON spec for an Excel workbook. You NEVER write prose — output is a single JSON object, nothing else.

The spec MUST match this exact shape:

{
  "title": "...",                                  // workbook title
  "language": "en|uz|ru|es|ar",                    // match the user's language
  "palette": {                                     // pick a professional palette tied to topic & tone
    "header_bg":   "#hex", // header row background (strong)
    "header_text": "#ffffff or near",
    "accent":      "#hex", // totals row + chart accent
    "zebra":       "#hex"  // alternate row tint (very light)
  },
  "sheets": [
    {
      "name": "Sheet name (max 31 chars)",
      "title": "Visible title above the table",
      "headers": ["Column A", "Column B", ...],
      "rows": [
        ["label", number, number, number, "=SUM(B4:D4)"],  // each row is an array; numbers as numbers, formulas as "=..."
        ...
      ],
      "totals_row": ["TOTAL", "=SUM(B4:B6)", ...],         // OPTIONAL — same length as headers
      "number_format": "$#,##0",                            // OPTIONAL — e.g. "$#,##0", "0.00%", "#,##0", ""
      "freeze_panes": "B4",                                 // OPTIONAL — Excel ref of the top-left unfrozen cell
      "column_widths": [22, 12, 12, 12, 14],                // OPTIONAL — length matches headers
      "chart": {                                            // OPTIONAL — see chartHint rule
        "type":  "column|line|bar|pie",
        "title": "Chart title",
        "categories_col":      1,
        "value_cols":          [2, 3, 4],
        "categories_first_row": 4,
        "categories_last_row":  7,
        "position": "H4"
      }
    }
  ]
}

Rules:
1. Generate realistic, specific data — never write all 100s or 1000s. Vary across rows so charts are interesting.
2. Tone→palette: corporate→navy headers; modern→teal/slate; playful→coral/amber; academic→burgundy/cream.
3. Numbers go as JSON numbers, not strings. Formulas as strings starting with "=". Be careful with row indices — they are 1-indexed in Excel, and the data starts on row 4 (row 1 = title, row 3 = headers, row 4+ = data).
4. Include a totals_row whenever the sheet has summable columns.
5. Pick number_format that matches the data: currency for revenue ("$#,##0"), percent for rates ("0.00%"), plain for counts ("#,##0").
6. ` + chartHint + `
7. All sheet/header/title labels in the user's language. Numbers stay universal (don't translate digits).
8. Output ONLY the JSON. No markdown, no commentary.`
}

func buildXLSXUserPrompt(j *models.XLSXBuilderJob) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "Topic: %s\n", j.Topic)
	fmt.Fprintf(&sb, "Sheet count: %d\n", j.SheetCount)
	fmt.Fprintf(&sb, "Tone: %s\n", j.Tone)
	fmt.Fprintf(&sb, "Output language: %s\n", j.Language)
	if j.Audience != "" {
		fmt.Fprintf(&sb, "Audience: %s\n", j.Audience)
	}
	if !j.IncludeChart {
		sb.WriteString("Do NOT include charts.\n")
	}
	sb.WriteString("\nReturn the JSON spec now. Output JSON only.")
	return sb.String()
}
