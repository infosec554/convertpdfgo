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
	return fmt.Sprintf(`You are an expert presentation designer (Pitch / Gamma / Stripe Press style). You produce a JSON spec for a PowerPoint deck. You NEVER write prose — output is a single JSON object, nothing else.

THE SPEC SHAPE (every field listed must be present unless marked optional):

{
  "title":    "Short deck title (≤ 7 words)",
  "subtitle": "One-line subtitle for the title slide (optional)",
  "language": "en|uz|ru|es|ar",
  "footer":   "Short footer brand mark shown at the bottom of every content slide, e.g. '%s · 2026' (≤ 40 chars)",
  "palette": {
    "primary": "#hex",   // strongest brand color, used for headings + section bg
    "accent":  "#hex",   // contrast color — bullets, numbers, dots, bars (NOT identical to primary)
    "bg":      "#hex",   // slide bg — almost always "#ffffff" or "#fafafa"
    "text":    "#hex",   // body text — high contrast on bg (very dark, like "#1a202c")
    "muted":   "#hex"    // subtitle, footer, page number — mid-gray like "#718096"
  },
  "fonts": { "heading": "Inter|Roboto|Lato|Source Sans Pro|Poppins", "body": "Inter|Roboto|Lato" },
  "slides": [ ... EXACTLY %d slide objects ... ]
}

LAYOUTS YOU CAN USE (mix freely — DON'T just stack bullets):

  - "title"      → { "layout":"title", "title":"...", "subtitle":"..." }
  - "agenda"     → { "layout":"agenda", "title":"What we'll cover", "items":["Section 1", "Section 2", "Section 3", "Section 4"] }   (3-6 items)
  - "section"    → { "layout":"section", "section_number":"01", "title":"Section name (no number — it's in section_number)", "subtitle":"optional one-liner" }
  - "big_text"   → { "layout":"big_text", "title":"Tiny eyebrow above", "body":"One bold statement, max 2 sentences, max 20 words total" }
  - "bullets"    → { "layout":"bullets", "title":"...", "bullets":["...", "..."], "notes":"speaker note (optional)" }   (3-5 bullets, each 4-10 words, parallel structure)
  - "two_column" → { "layout":"two_column", "title":"...", "left_title":"...", "left_bullets":[...], "right_title":"...", "right_bullets":[...] }   (use for contrasts)
  - "comparison" → { "layout":"comparison", "title":"...", "a_label":"Before|Old|Without us", "a_body":"One sentence", "b_label":"After|New|With us", "b_body":"One sentence" }
  - "timeline"   → { "layout":"timeline", "title":"...", "items":[{"label":"Q1","text":"short"}, {"label":"Q2","text":"short"}, ...] }   (3-5 items)
  - "stat"       → { "layout":"stat", "title":"Eyebrow", "stat_value":"42%% or $4.2M or 5x or 100+", "stat_label":"What the number means", "support":"optional one-liner" }
                    NOTE: stat_value MUST be ≤ 8 characters. Use SHORT formats: "42%%", "$4.2M", "5x", "100+", "3 ta". Long words like "Strong" / "Kuchli o'sish" break the layout — put those in stat_label instead.
  - "quote"      → { "layout":"quote", "quote":"...", "author":"Name, role" }
  - "thanks"     → { "layout":"thanks", "title":"Thank you.", "subtitle":"optional one-liner" }

HARD RULES — VIOLATING ANY OF THESE BREAKS THE DECK:

1. The slides[] array MUST contain EXACTLY %d items. Not 7, not 9, not "approximately 8". Count carefully before output.
2. Slide 1 MUST be "title". Slide N (the last one) MUST be "thanks".
3. If slide count ≥ 6, include EXACTLY ONE "agenda" slide as slide 2.
4. Use a variety of layouts — NEVER more than 2 "bullets" slides in a row. Sprinkle in big_text, stat, comparison, quote, timeline, two_column to give the deck visual rhythm.
5. Section breaks: include 1-2 "section" slides splitting the body into named acts (Findings / Plan, Problem / Solution / Ask, etc.). Use section_number "01", "02".

DATA-INTEGRITY RULES — DO NOT INVENT FACTS:

6. NEVER fabricate specific revenue numbers, customer counts, dates, or attributed quotes UNLESS the user's topic clearly contains them. For a topic like "Introduction to machine learning" do not invent "$4.2M revenue" — that's a hallucination. Instead use:
   - "stat" slides with structural facts (e.g. "3 phases", "100+ languages") not invented revenue
   - "quote" slides only when a real quote is given OR you write a plausible aphorism with attribution "— Anonymous"
   - Numbers that are universally true ("60%% of meetings" — common stat) are OK; brand-specific numbers are NOT
7. For unfamiliar / generic topics, prefer QUALITATIVE statements over QUANTITATIVE ones. "Strong growth in core segment" not "$12.4M revenue".
8. If the topic is concrete and gives you facts (e.g. "Q4 2025 sales review where we beat target by 18%%"), USE those facts verbatim — don't replace them with invented ones.

DESIGN RULES:

9. Palette tied to tone:
   - corporate → primary deep navy (#1a365d, #1e3a5f, #243b53), accent in warm orange/gold/blue
   - modern   → primary teal/slate (#0f766e, #134e4a, #1e293b), accent in soft cyan/coral
   - playful  → primary warm (#c2410c, #be185d, #7c3aed), accent in yellow/coral
   - academic → primary burgundy (#7c2d12, #831843, #5b21b6), accent in cream/gold/forest
   NEVER use neon, eye-bleeding hex, or extremely-similar primary+accent. The contrast between primary and accent should be obvious at a glance.
10. Bg is "#ffffff" or "#fafafa" or "#f9fafb". NEVER colored backgrounds (only section/thanks slides get the primary bg, handled by the renderer).
11. Bullets / labels / titles in the user's chosen language. Don't translate proper nouns (brand names, places).

OUTPUT FORMAT:

12. Output ONLY the raw JSON object. No markdown fences, no prose before or after, no commentary.`,
		j.Topic[:min(40, len(j.Topic))], j.SlideCount, j.SlideCount)
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
