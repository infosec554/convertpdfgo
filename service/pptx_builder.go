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

// PPTXBuilderService creates AI-generated PowerPoint decks from a topic.
// Pipeline:
//  1. Gemini call returns a slide-outline JSON (palette, fonts, slides)
//  2. We hand the JSON to scripts/build_pptx.py (python-pptx)
//  3. The .pptx is saved as a file, linked back to the job
type PPTXBuilderService interface {
	Create(ctx context.Context, req models.PPTXBuilderRequest, userID *string) (string, error)
	Process(ctx context.Context, jobID string) error
	GetByID(ctx context.Context, id string) (*models.PPTXBuilderJob, error)
}

type pptxBuilderService struct {
	stg storage.IStorage
	log logger.ILogger
	llm llm.Provider
}

// NewPPTXBuilderService boots the builder. If the LLM provider is missing
// (no GEMINI_API_KEY), Process() will return ErrPPTXBuilderDisabled —
// Create() still records the job so the failure is observable.
func NewPPTXBuilderService(stg storage.IStorage, log logger.ILogger) PPTXBuilderService {
	p, err := llm.FromEnv()
	if err != nil {
		log.Warning("pptx_builder: LLM provider not configured — pptx-builder will fail on Process",
			logger.Error(err))
		return &pptxBuilderService{stg: stg, log: log}
	}
	return &pptxBuilderService{stg: stg, log: log, llm: p}
}

var ErrPPTXBuilderDisabled = errors.New("pptx_builder: feature disabled (LLM provider not configured)")

func (s *pptxBuilderService) Create(ctx context.Context, req models.PPTXBuilderRequest, userID *string) (string, error) {
	// Defaults
	if req.SlideCount < 5 || req.SlideCount > 15 {
		req.SlideCount = 8
	}
	if req.Tone == "" {
		req.Tone = "corporate"
	}
	if req.Language == "" {
		req.Language = "en"
	}
	job := &models.PPTXBuilderJob{
		ID:         uuid.NewString(),
		UserID:     userID,
		Topic:      strings.TrimSpace(req.Topic),
		SlideCount: req.SlideCount,
		Tone:       req.Tone,
		Language:   req.Language,
		Audience:   strings.TrimSpace(req.Audience),
		Status:     "pending",
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	if err := s.stg.PPTXBuilder().Create(ctx, job); err != nil {
		return "", err
	}
	return job.ID, nil
}

func (s *pptxBuilderService) Process(ctx context.Context, jobID string) error {
	job, err := s.stg.PPTXBuilder().GetByID(ctx, jobID)
	if err != nil {
		return fmt.Errorf("get job: %w", err)
	}
	job.Status = "running"
	_ = s.stg.PPTXBuilder().Update(ctx, job)

	// 1. Generate the spec via the LLM
	spec, err := s.generateSpec(ctx, job)
	if err != nil {
		s.markFailed(ctx, job, fmt.Sprintf("llm: %v", err))
		return fmt.Errorf("llm spec: %w", err)
	}

	// 2. Render via python-pptx
	outputFileID := uuid.NewString()
	outDir := "storage/pptx_builder"
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		s.markFailed(ctx, job, fmt.Sprintf("mkdir: %v", err))
		return err
	}
	outPath := filepath.Join(outDir, outputFileID+".pptx")

	specPath := filepath.Join(outDir, outputFileID+".spec.json")
	specBytes, _ := json.MarshalIndent(spec, "", "  ")
	if err := os.WriteFile(specPath, specBytes, 0o644); err != nil {
		s.markFailed(ctx, job, fmt.Sprintf("write spec: %v", err))
		return err
	}
	defer os.Remove(specPath)

	ctxCmd, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctxCmd, "python3", "scripts/build_pptx.py", specPath, outPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		s.markFailed(ctx, job, fmt.Sprintf("python-pptx: %s: %v", strings.TrimSpace(string(out)), err))
		return fmt.Errorf("python-pptx: %w (%s)", err, strings.TrimSpace(string(out)))
	}

	info, err := os.Stat(outPath)
	if err != nil {
		s.markFailed(ctx, job, fmt.Sprintf("stat: %v", err))
		return err
	}

	// 3. Save as a file row
	displayName := safeFilename(job.Topic) + ".pptx"
	if _, err := s.stg.File().Save(ctx, models.File{
		ID:         outputFileID,
		UserID:     job.UserID,
		FileName:   displayName,
		FilePath:   outPath,
		FileType:   "application/vnd.openxmlformats-officedocument.presentationml.presentation",
		FileSize:   info.Size(),
		UploadedAt: time.Now(),
	}); err != nil {
		s.markFailed(ctx, job, fmt.Sprintf("save file: %v", err))
		return err
	}

	job.OutputFileID = &outputFileID
	job.Status = "done"
	return s.stg.PPTXBuilder().Update(ctx, job)
}

func (s *pptxBuilderService) markFailed(ctx context.Context, job *models.PPTXBuilderJob, errStr string) {
	job.Status = "failed"
	job.Error = errStr
	if err := s.stg.PPTXBuilder().Update(ctx, job); err != nil {
		s.log.Error("pptx_builder: mark failed", logger.Error(err))
	}
}

func (s *pptxBuilderService) GetByID(ctx context.Context, id string) (*models.PPTXBuilderJob, error) {
	return s.stg.PPTXBuilder().GetByID(ctx, id)
}

// ─── Gemini outline generation ──────────────────────────────────────────

// pptxSpec mirrors the JSON schema the python script expects.
type pptxSpec struct {
	Title    string          `json:"title"`
	Language string          `json:"language"`
	Palette  map[string]string `json:"palette"`
	Fonts    map[string]string `json:"fonts"`
	Slides   []map[string]any  `json:"slides"`
}

func (s *pptxBuilderService) generateSpec(ctx context.Context, job *models.PPTXBuilderJob) (*pptxSpec, error) {
	if s.llm == nil {
		return nil, ErrPPTXBuilderDisabled
	}
	sys := buildPPTXSystemPrompt(job)
	user := buildPPTXUserPrompt(job)

	msgs := []llm.Message{{Role: llm.RoleUser, Content: user}}
	stream, err := s.llm.Chat(ctx, msgs, llm.ChatOpts{
		System:      sys,
		Temperature: 0.65, // a little creativity but mostly structured
		MaxTokens:   4096,
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

	raw := strings.TrimSpace(sb.String())
	raw = stripCodeFence(raw)

	var spec pptxSpec
	if err := json.Unmarshal([]byte(raw), &spec); err != nil {
		return nil, fmt.Errorf("parse spec: %w; got: %s", err, truncForLog(raw, 400))
	}
	if len(spec.Slides) == 0 {
		return nil, errors.New("empty slides")
	}
	// Provide safety defaults if the LLM forgot any palette/font keys
	for k, v := range map[string]string{"primary": "#1a365d", "accent": "#ed8936", "bg": "#ffffff", "text": "#1a202c", "muted": "#718096"} {
		if _, ok := spec.Palette[k]; !ok {
			if spec.Palette == nil {
				spec.Palette = map[string]string{}
			}
			spec.Palette[k] = v
		}
	}
	if spec.Fonts == nil {
		spec.Fonts = map[string]string{}
	}
	if spec.Fonts["heading"] == "" {
		spec.Fonts["heading"] = "Inter"
	}
	if spec.Fonts["body"] == "" {
		spec.Fonts["body"] = "Inter"
	}
	return &spec, nil
}

func buildPPTXSystemPrompt(j *models.PPTXBuilderJob) string {
	return `You are an expert presentation designer who produces a JSON spec for a PowerPoint deck. You NEVER write prose — output is a single JSON object, nothing else.

The spec MUST match this exact shape:

{
  "title": "...",                                  // short deck title
  "language": "en|uz|ru|es|ar",                    // match the user's language
  "palette": {                                     // pick a professional, tasteful palette tied to the topic & tone
    "primary": "#hex",   // strongest brand color
    "accent":  "#hex",   // contrast — used on numbers, bars, bullets
    "bg":      "#ffffff or near-white",            // main slide background
    "text":    "#hex",   // body text — high contrast on bg
    "muted":   "#hex"    // subtitle / footnote color
  },
  "fonts": { "heading": "Inter|Roboto|Lato|Source Sans Pro|Poppins", "body": "Inter|Roboto|Lato|Source Sans Pro" },
  "slides": [ ... ]
}

Slide layouts you may use (and SHOULD vary across the deck to keep visual rhythm):
  - "title"      → { "layout":"title",      "title":"...", "subtitle":"..." }
  - "section"    → { "layout":"section",    "title":"01 — Section name", "subtitle":"optional one-liner" }
  - "bullets"    → { "layout":"bullets",    "title":"...", "bullets":["...", "..."], "notes":"speaker notes (optional)" }
  - "two_column" → { "layout":"two_column", "title":"...", "left_title":"...", "left_bullets":[...], "right_title":"...", "right_bullets":[...] }
  - "quote"      → { "layout":"quote",      "quote":"...", "author":"..." }
  - "stat"       → { "layout":"stat",       "title":"...", "stat_value":"$4.2M", "stat_label":"Q4 revenue", "support":"optional one-liner" }
  - "thanks"     → { "layout":"thanks",     "title":"Thank you", "subtitle":"optional" }

Rules:
1. First slide MUST be "title", last slide MUST be "thanks".
2. Mix layouts. Pure-bullets decks look amateur — use section breaks every 3-4 content slides, drop in a "stat" or "quote" where it has impact, use "two_column" for contrasts.
3. Bullets: 3-5 per slide, each 6-14 words. Punchy, parallel structure. No periods on bullet items unless they're full sentences.
4. Palette: tie to topic/tone. Corporate→deep blue/navy primary. Modern→teal/black. Playful→warm coral/yellow. Academic→burgundy/cream. NEVER use neon or eye-bleeding colors. Bg is white or a near-white (#fafafa, #f9fafb).
5. Speaker notes on bullet slides: one sentence on what to emphasise verbally — NOT a re-read of the bullets.
6. Stat slides: huge specific number ($4.2M, 87%, 12,000 users) — pick numbers that actually land.
7. Output language matches the requested language EXACTLY. Headings, bullets, quotes, everything.
8. Output ONLY the JSON. No markdown, no commentary, no leading/trailing prose.`
}

func buildPPTXUserPrompt(j *models.PPTXBuilderJob) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "Topic: %s\n", j.Topic)
	fmt.Fprintf(&sb, "Slide count target: exactly %d slides (including title + thanks)\n", j.SlideCount)
	fmt.Fprintf(&sb, "Tone: %s\n", j.Tone)
	fmt.Fprintf(&sb, "Output language: %s\n", j.Language)
	if j.Audience != "" {
		fmt.Fprintf(&sb, "Audience: %s\n", j.Audience)
	}
	sb.WriteString("\nReturn the JSON spec now. Output JSON only.")
	return sb.String()
}

// stripCodeFence removes ```json ... ``` if the LLM wraps despite instructions.
func stripCodeFence(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```") {
		// Drop first fence line
		if i := strings.IndexByte(s, '\n'); i > 0 {
			s = s[i+1:]
		}
		if j := strings.LastIndex(s, "```"); j > 0 {
			s = s[:j]
		}
	}
	return strings.TrimSpace(s)
}

func truncForLog(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func safeFilename(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "presentation"
	}
	var out strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			out.WriteRune(r)
		case r == ' ' || r == '-' || r == '_':
			out.WriteByte('-')
		}
	}
	res := out.String()
	if len(res) > 60 {
		res = res[:60]
	}
	if res == "" {
		return "presentation"
	}
	return res
}
