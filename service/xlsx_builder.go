package service

import (
	"context"
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

	// Stage 1 — LLM writes Python code that computes the workbook
	// spec. This is the Code-Interpreter pattern: the model handles
	// the natural-language → structure mapping, Python handles the
	// arithmetic. No Excel formulas means no cell-ref mistakes.
	code, err := s.generateCode(ctx, job)
	if err != nil {
		s.markFailed(ctx, job, fmt.Sprintf("llm: %v", err))
		return fmt.Errorf("llm code: %w", err)
	}

	outputFileID := uuid.NewString()
	outDir := "storage/xlsx_builder"
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		s.markFailed(ctx, job, fmt.Sprintf("mkdir: %v", err))
		return err
	}
	outPath := filepath.Join(outDir, outputFileID+".xlsx")

	// Stage 2 — sandbox-execute the Python. The sandbox enforces a
	// timeout + restricted builtins, and emits a result.json that the
	// renderer consumes.
	codePath   := filepath.Join(outDir, outputFileID+".code.py")
	resultPath := filepath.Join(outDir, outputFileID+".spec.json")
	if err := os.WriteFile(codePath, []byte(code), 0o644); err != nil {
		s.markFailed(ctx, job, fmt.Sprintf("write code: %v", err))
		return err
	}
	defer os.Remove(codePath)
	defer os.Remove(resultPath)

	ctxSandbox, cancelSandbox := context.WithTimeout(ctx, 15*time.Second)
	defer cancelSandbox()
	sandboxCmd := exec.CommandContext(ctxSandbox, "python3", "scripts/sandbox_xlsx.py", codePath, resultPath)
	sandboxOut, err := sandboxCmd.CombinedOutput()
	if err != nil {
		s.markFailed(ctx, job, fmt.Sprintf("sandbox: %s: %v", strings.TrimSpace(string(sandboxOut)), err))
		return fmt.Errorf("sandbox: %w (%s)", err, strings.TrimSpace(string(sandboxOut)))
	}

	// Stage 3 — render the resolved spec to a real .xlsx via xlsxwriter.
	ctxCmd, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctxCmd, "python3", "scripts/build_xlsx.py", resultPath, outPath)
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

// generateCode asks the LLM for Python source that, when run in the
// sandbox, defines a top-level `result` dict matching the renderer's
// expected shape. The Code-Interpreter pattern means the model owns
// the structure + labels, Python owns the arithmetic.
func (s *xlsxBuilderService) generateCode(ctx context.Context, job *models.XLSXBuilderJob) (string, error) {
	if s.llm == nil {
		return "", ErrXLSXBuilderDisabled
	}
	sys := buildXLSXSystemPrompt(job)
	user := buildXLSXUserPrompt(job)

	msgs := []llm.Message{{Role: llm.RoleUser, Content: user}}
	stream, err := s.llm.Chat(ctx, msgs, llm.ChatOpts{
		System:      sys,
		Temperature: 0.4, // structure-heavy task — lower variance is fine
		MaxTokens:   6144,
	})
	if err != nil {
		return "", err
	}

	var sb strings.Builder
	for ev := range stream {
		if ev.Err != nil {
			return "", ev.Err
		}
		sb.WriteString(ev.Delta)
	}
	raw := stripPythonFence(strings.TrimSpace(sb.String()))
	if !strings.Contains(raw, "result") {
		return "", fmt.Errorf("llm output missing `result` assignment; got: %s", truncForLog(raw, 400))
	}
	return raw, nil
}

// stripPythonFence drops ```python ... ``` if present.
func stripPythonFence(s string) string {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "```") {
		return s
	}
	if i := strings.IndexByte(s, '\n'); i > 0 {
		s = s[i+1:]
	}
	if j := strings.LastIndex(s, "```"); j > 0 {
		s = s[:j]
	}
	return strings.TrimSpace(s)
}

func buildXLSXSystemPrompt(j *models.XLSXBuilderJob) string {
	chartHint := `Set "chart" on the first sheet — pick "column" / "line" / "bar" / "pie" by data shape (column for cross-row comparisons, line for time series, bar for ranked rows, pie for share-of-whole). Use:
    "chart": {
      "type": "column",
      "title": "...",
      "categories_col": 1,                # 1-indexed
      "value_cols": [2, 3],
      "categories_first_row": 4,
      "categories_last_row":  3 + len(rows),
      "position": "G3"
    }`
	if !j.IncludeChart {
		chartHint = `Set "chart" to None.`
	}
	return `You write Python code that the server runs in a sandbox to produce a workbook spec. You NEVER output prose — output is one Python script, nothing else.

THE ONE THING YOU MUST DO: define a top-level variable named ` + "`result`" + ` that is a dict matching the shape below. Every numeric cell is a real Python int/float computed in code — NEVER an Excel formula string. The renderer trusts the values you put in.

THE SANDBOX:

You may use these pre-imported modules (no import statement needed):
  math, statistics, datetime, json, re

You may NOT:
  - import any other module
  - open files
  - access the network
  - call eval / exec / compile / __import__

Hard timeout: 8 seconds. Hard size cap: 64 KB of code.

THE RESULT SHAPE (build this dict, name it ` + "`result`" + `):

result = {
  "title":    "Workbook title",
  "language": "en | uz | ru | es | ar",
  "palette": {
    "header_bg":   "#hex",
    "header_text": "#ffffff",
    "accent":      "#hex",
    "zebra":       "#hex"
  },
  "sheets": [
    {
      "name":  "Sheet name (max 31 chars)",
      "title": "Visible title above the table",
      "headers": ["Col A", "Col B", ...],
      "rows": [
        ["label", 100, 200, 90],           # pure values — int / float / str only
        ...
      ],
      "totals_row":    ["TOTAL", 1234, None, 5678],   # optional, same length as headers, None where no total fits
      "number_format": "$#,##0" | "#,##0.00" | "0.00%%" | "#,##0",
      "freeze_panes":  "B4",
      "column_widths": [22, 12, 12, 14],
      "chart":         {...} or None
    }
  ]
}

HARD RULES:

1. EVERY numeric cell is a Python int / float you COMPUTE — never an Excel formula string like "=B4*B5". The sandbox does the math; the renderer just writes values.

2. TOPIC-FIRST. Extract numbers and counts from the user's topic and use them as inputs:
   - "3 workers" → exactly 3 data rows
   - "salary 10,000,000" → that number is a variable
   - "reduce by 10%%" → reduction = 0.10, apply it
   - "monthly budget" → 12 data rows (Jan-Dec)

3. Build the workbook in code. Pattern:
       # Inputs from the topic
       daily_users  = 50000
       ops_per_user = 3
       active_hours = 14
       peak_mult    = 15

       # Derived (Python computes — NEVER a formula string)
       daily_ops    = daily_users * ops_per_user
       avg_req_sec  = daily_ops / (active_hours * 3600)
       peak_req_sec = avg_req_sec * peak_mult

       # Assemble the spec
       result = {
         "title":    "Server Resource Calculator",
         "language": "en",
         "palette":  {"header_bg": "#0f766e", "header_text": "#ffffff", "accent": "#22d3ee", "zebra": "#f0fdfa"},
         "sheets": [{
           "name":  "Calculator",
           "title": "Server Resource Calculator",
           "headers": ["Parameter", "Value", "Unit"],
           "rows": [
             ["Daily Users",      daily_users,             "users"],
             ["Ops per User",     ops_per_user,            "ops/user"],
             ["Active Hours",     active_hours,            "hours"],
             ["Peak Multiplier",  peak_mult,               "x"],
             ["Daily Operations", daily_ops,               "ops/day"],
             ["Avg Req/Sec",      round(avg_req_sec, 2),   "req/sec"],
             ["Peak Req/Sec",     round(peak_req_sec, 2),  "req/sec"],
           ],
           "totals_row":    None,
           "number_format": "#,##0.00",
           "freeze_panes":  "B4",
           "column_widths": [22, 16, 12],
           "chart":         None
         }]
       }

4. ` + chartHint + `

5. Tone → palette guidance (use as a starting point, you may adjust):
     corporate → header_bg "#1a365d" or "#1e3a5f", accent "#ed8936"
     modern    → header_bg "#0f766e" or "#1e293b", accent "#22d3ee"
     playful   → header_bg "#c2410c" or "#be185d", accent "#fbbf24"
     academic  → header_bg "#7c2d12" or "#831843", accent "#fbbf24"

6. Headers / labels / titles in the user's language. Numbers stay universal.

7. number_format: pick ONE per sheet based on dominant column type:
     - "#,##0"     → counts, ID numbers
     - "$#,##0"    → currency
     - "0.00%%"    → percentages (cell value 0.10 displays as "10.00%%")
     - "#,##0.00"  → mixed math (calculator-style sheets)

8. Single sheet preferred unless the topic clearly asks for multiple. Do NOT invent a second sheet about a different topic.

9. Output ONLY the Python script. No markdown fences, no commentary, no explanation. The first non-blank character of your output is either an assignment or a comment.`
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
