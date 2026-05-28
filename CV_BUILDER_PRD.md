# AI Resume / CV Builder — Technical PRD

**Loyiha**: convertpdfgo.com `/cv-builder` tool — AI-powered universal CV builder
**Versiya**: v1.0 (MVP launch)
**LLM**: Gemini 2.5 Flash Lite (mavjud `service/llm` paketi)
**Embedding**: Gemini `gemini-embedding-001` (768-dim, mavjud)
**PDF rendering**: Gotenberg (HTML → PDF, mavjud)
**Vector DB**: PostgreSQL + pgvector (mavjud)
**OCR**: ocrmypdf + Tesseract (mavjud)
**Sana**: 2026-05-28
**Author**: Zarif (infosec554)

---

## 1. Executive Summary

### Muammo
Hozirda online CV builderlar bir nechta jiddiy bo'shliqlarga ega:
- **Til chegaralanishi**: Resume.io, Kickresume, Rezi va boshqalarning aksariyati 5–10 til. O'zbekcha, Arab, Hindcha sifatli emas.
- **Voice input yo'q**: Hech bir leader to'liq voice-first CV builder bermagan.
- **Scanned CV import yo'q**: Eski qog'oz CV (skan/rasm) yangi dizaynga aylantirib bo'lmaydi.
- **AI chat orqali tahrir**: Hech kim "CV'ngdan AI bilan suhbat qil" feature'ini bermagan.
- **Bepul cheklash**: Resume.io oyiga $15, Kickresume $24/oy. Bepul variantlari watermark/PDF'siz.

### Yechim
Convertpdfgo CV Builder = **universal industry feature'lar + 4 ta unique afzallik**:

**Universal (top 5)**:
1. Template gallery (10+ dizayn, ATS-friendly)
2. AI content per field (Gemini Improve button)
3. ATS score (real-time, vakansiya'ga moslab)
4. Multi-format export (PDF / DOCX / TXT)
5. Job-targeted tailoring

**Bizning unique afzalliklarimiz** (mavjud stack'dan kelib chiqgan):
6. **Voice-to-CV** (Web Speech API + Gemini structurization) — telefonda gapirib CV
7. **Scanned paper CV → digital** (Tesseract OCR → Gemini parse → re-design)
8. **20 tilda native** (mavjud i18n + Gemini multi-lingual generation)
9. **Chat with my CV** (mavjud `/chat` infrastructure ulanadi — CV'ng haqida AI'dan so'rash)

### Asosiy texnologik tanlovlar

| Komponent | Tanlangan | Sabab |
|---|---|---|
| **LLM** | Gemini 2.5 Flash Lite | Mavjud, tekin, 1000+ req/kun, multi-lingual |
| **Embedding** | Gemini gemini-embedding-001 | Mavjud, 768-dim, mos jadval allaqachon bor |
| **CV parsing** | Gemini structured output (JSON mode) | Eng aniq, prompt engineering yetarli |
| **OCR (skan CV)** | ocrmypdf + Tesseract | Mavjud, 100+ til qo'llab-quvvatlaydi |
| **Template render** | HTML + Tailwind → Gotenberg → PDF | Mavjud Gotenberg ishlatiladi |
| **Vector search** | pgvector (CV section similarity) | Mavjud schema |
| **Voice input** | Web Speech API (client) + Gemini parse | Hech qanday backend infra qo'shimcha kerak emas |
| **ATS score** | Custom Go service + Gemini hint | Backend rule engine + AI assist |

### Migration roadmap
Kod paid LLM'ga osongina o'tadi (chunki mavjud `llm.Provider` interface ishlatiladi). `LLM_PROVIDER=claude` env almashtirsa bo'ldi.

### Marketing surface
CV Builder homepage'da yangi **CAREER** kategoriyasi sifatida ko'rinadi (AI-POWERED'dan
keyin). 8 ta sub-tool kartasi (`/cv-builder`, `/cv-from-pdf`, `/cv-voice`, `/ats-check`,
`/resume-translate`, `/cover-letter`, `/resume-to-docx`, `/chat-with-cv`) — backend
bittasini ishlatadi, lekin har biri alohida landing page (SEO uchun har kalit so'z
o'z URL'iga ega: "ATS checker", "resume translator", va h.k.). To'liq ro'yxat va
implementation strategy §12.1–12.2'da.

---

## 2. User Flows

### Flow A: Upload existing CV → Edit + Redesign

```
1. User /cv-builder ga keladi
   └─ 3 ta katta CTA card: Upload / Start fresh / Voice
   └─ "Upload existing CV" bosadi

2. Fayl tanlash
   ├─ Format: PDF / DOCX / JPG / PNG (skan)
   ├─ Drag-drop yoki Choose files yoki Dropbox/Drive
   └─ Backend: POST /file/upload → file_id

3. AI parser ishga tushadi (~3-5s)
   ├─ Agar PDF/DOCX → pdftotext / mavjud Word→text
   ├─ Agar rasm/skan → ocrmypdf → text
   ├─ Text → Gemini (JSON mode) → strukturalashtirilgan CV
   │   {personal, summary, experience[], education[], skills[], ...}
   └─ POST /api/cv/sessions {file_id, mode:"import"} → cv_id

4. Editor ochiladi (3-column layout)
   ├─ Chap: bo'limlar ro'yxati + ATS score widget
   ├─ Markaz: faol bo'lim formasi (AI Improve tugmasi har field'da)
   └─ O'ng: jonli HTML PDF preview

5. User tahrir qiladi
   ├─ Har field uchun "✨ Improve with AI" mavjud (Gemini → 3 variant)
   ├─ Section qo'shish/o'chirish
   └─ Template tanlash modali (10 ta gallery)

6. Eksport
   ├─ PDF (default) — Gotenberg HTML → PDF
   ├─ DOCX — convert via Gotenberg
   ├─ TXT — plain markdown export
   └─ Save to Dropbox / Drive (mavjud /api/storage)
```

### Flow B: Start from scratch (wizard)

```
1. "Start from scratch" bosadi
   └─ POST /api/cv/sessions {mode:"scratch"} → empty cv_id

2. Step-by-step wizard
   ├─ Step 1: Personal info (ism, email, telefon, LinkedIn, photo upload)
   ├─ Step 2: Summary (AI taklif: "Quyidagi savollarga javob bering: 1) lavozim 2) yillar 3) ko'nikma" → AI yozadi)
   ├─ Step 3: Experience (har ish: title, company, period, bullets — har bullet uchun AI Improve)
   ├─ Step 4: Education
   ├─ Step 5: Skills (AI taklif: lavozim'ga qarab kerakli skill'larni avtomatik)
   ├─ Step 6: Languages
   └─ Step 7: Final review + template tanlash

3. Editor 3-column layout'da ochiladi (Flow A bilan bir xil)
4. Export — Flow A bilan bir xil
```

### Flow C: Voice-to-CV

```
1. "Talk it through" bosadi
   └─ POST /api/cv/sessions {mode:"voice"} → empty cv_id

2. Voice recorder ekran
   ├─ Web Speech API ishga tushadi (browser native)
   ├─ Live transcript ekranda
   ├─ User gapiradi: "I worked at XYZ from 2022 as senior engineer. Built payment system..."
   └─ Har 5–10s buffer Gemini'ga yuboriladi → strukturalashtiriladi

3. AI sturkturalashtiradi
   ├─ Transcript → Gemini JSON mode → section + content
   ├─ Backend: POST /api/cv/<id>/voice {transcript, section_hint?}
   │   → {sections: [{type:"experience", content:{...}}]}
   └─ Editor real-time'da to'ldiriladi

4. Section'lar to'lganidan keyin
   ├─ "⏭ Next section: Education" yoki "⏹ Stop & save"
   └─ Editor 3-column'ga o'tadi (Flow A/B bilan bir xil)

5. Eksport — bir xil
```

### Flow D: Chat with my CV (mavjud /chat'ga ulashish)

```
1. Editor'da "💬 Chat with this CV" tugma
   └─ /chat?cv=<cv_id> ga otadi

2. Mavjud chat infrastructure
   ├─ CV PDF avtomatik render qilinadi
   ├─ Chat session yaratiladi
   └─ User CV haqida AI bilan suhbat qiladi
      "Buni qisqartiring", "Engineering manager rolida tajriba bormi?", va h.k.

3. Suhbat tugagach user editor'ga qaytadi
   └─ Chat'da takliflar bo'lsa, "Apply suggestion" tugmasi bilan kiritadi
```

---

## 3. Architecture

```
                     ┌──────────────────────────────────┐
                     │       Browser (Next.js)          │
                     │  /cv-builder · 3-column editor   │
                     └──────┬───────────────────────────┘
                            │ HTTPS
                            ▼
                     ┌──────────────────────────────────┐
                     │     Caddy (mavjud)                │
                     └──────┬───────────────────────────┘
                            │
                     ┌──────▼───────────────────────────┐
                     │  convertpdfgo_api (Go/Gin)        │
                     │  /api/cv/* yangi handler'lar      │
                     └──────┬─────────────┬─────────────┘
                            │             │
              ┌─────────────┘             └─────────────┐
              ▼                                          ▼
       ┌──────────────┐                       ┌──────────────────┐
       │  Postgres    │                       │  External APIs   │
       │  + pgvector  │                       │ ┌──────────────┐ │
       │              │                       │ │  Gemini      │ │
       │ cv_sessions  │                       │ │  (chat+embed)│ │
       │ cv_sections  │                       │ └──────────────┘ │
       │ cv_templates │                       └──────────────────┘
       │ cv_ats_runs  │                                ▲
       │ cv_chunks    │                                │
       │  (embeddings)│                                │
       └──────────────┘                                │
              ▲                                        │
              │                              ┌─────────┴────────┐
       ┌──────┴───────┐                      │  convertpdfgo_   │
       │ asynq worker │──────────────────────│  worker (asynq)  │
       │              │  cv:parse, cv:export │                  │
       │              │                      │                  │
       └──────────────┘                      └─────┬────────────┘
                                                    │
                                            ┌───────▼──────────┐
                                            │  gotenberg       │
                                            │ HTML→PDF, OCR    │
                                            └──────────────────┘
```

---

## 4. Database Schema

Yangi migration: `migrations/postgres/0018_cv_builder.up.sql`

```sql
-- Asosiy CV session — har user uchun ko'p bo'lishi mumkin
CREATE TABLE cv_sessions (
    id              UUID PRIMARY KEY,
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title           TEXT NOT NULL DEFAULT 'Untitled CV',
    mode            VARCHAR(20) NOT NULL
                    CHECK (mode IN ('scratch', 'import', 'voice')),
    source_file_id  UUID REFERENCES files(id) ON DELETE SET NULL,
                    -- mode='import' bo'lsa, original yuklangan fayl
    template_id     VARCHAR(50) NOT NULL DEFAULT 'modern',
                    -- "modern", "classic", "creative", "minimal", "tech", ...
    locale          VARCHAR(10) NOT NULL DEFAULT 'en',
                    -- CV qaysi tilda chiqadi
    status          VARCHAR(20) NOT NULL DEFAULT 'draft'
                    CHECK (status IN ('draft', 'parsing', 'ready', 'failed')),
    parse_error     TEXT,
    last_pdf_id     UUID REFERENCES files(id) ON DELETE SET NULL,
                    -- oxirgi PDF eksport (cache)
    created_at      TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMP NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_cv_sessions_user ON cv_sessions(user_id, updated_at DESC);

-- Section'lar — har CV uchun bir nechta (Personal, Summary, Experience, ...)
-- Har section'da JSONB content saqlanadi (flex schema; har section turi farqli)
CREATE TABLE cv_sections (
    id              UUID PRIMARY KEY,
    cv_id           UUID NOT NULL REFERENCES cv_sessions(id) ON DELETE CASCADE,
    section_type    VARCHAR(30) NOT NULL,
                    -- "personal" | "summary" | "experience" | "education"
                    -- | "skills" | "languages" | "projects" | "awards"
                    -- | "certifications" | "publications" | "volunteer" | "custom"
    sort_order      INT NOT NULL DEFAULT 0,
    content         JSONB NOT NULL,
                    -- Flexible — section turi bo'yicha schema
                    -- Misol (experience):
                    -- {"items": [{"title":"Senior Eng", "company":"XYZ",
                    --             "period":"2022-Present", "bullets":["..."]}]}
    embedding       vector(768),
                    -- Section text'i embeddinglangan
                    -- ATS keyword match va job-tailoring uchun ishlatiladi
    created_at      TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMP NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_cv_sections_cv ON cv_sections(cv_id, sort_order);
CREATE INDEX idx_cv_sections_embed ON cv_sections
    USING hnsw (embedding vector_cosine_ops);

-- Template'lar registry (static + custom)
-- Asosan code-side enum, lekin admin keyin yangi qo'shsa
CREATE TABLE cv_templates (
    id              VARCHAR(50) PRIMARY KEY,
                    -- slug: "modern", "classic", ...
    name            TEXT NOT NULL,
    description     TEXT,
    is_ats_friendly BOOLEAN NOT NULL DEFAULT TRUE,
    preview_url     TEXT,
                    -- /cv-templates/modern.png
    html_path       TEXT NOT NULL,
                    -- internal: templates/modern.html.tmpl
    category        VARCHAR(20),
                    -- "modern" | "classic" | "creative" | "tech" | "executive"
    sort_order      INT NOT NULL DEFAULT 0,
    is_public       BOOLEAN NOT NULL DEFAULT TRUE,
    created_at      TIMESTAMP NOT NULL DEFAULT NOW()
);

-- ATS optimization runs (har necha vakansiya tavsifiga moslab)
CREATE TABLE cv_ats_runs (
    id              UUID PRIMARY KEY,
    cv_id           UUID NOT NULL REFERENCES cv_sessions(id) ON DELETE CASCADE,
    job_title       TEXT,
    job_description TEXT NOT NULL,
    score           INT NOT NULL CHECK (score BETWEEN 0 AND 100),
    feedback        JSONB NOT NULL,
                    -- {"missing_keywords": ["docker","k8s"],
                    --  "weak_bullets": [...],
                    --  "format_issues": [],
                    --  "suggestions": [...]}
    created_at      TIMESTAMP NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_cv_ats_runs_cv ON cv_ats_runs(cv_id, created_at DESC);

-- AI generation history — bir field uchun necha marta "Improve" chaqirilgani
CREATE TABLE cv_ai_history (
    id              UUID PRIMARY KEY,
    cv_id           UUID NOT NULL REFERENCES cv_sessions(id) ON DELETE CASCADE,
    section_id      UUID REFERENCES cv_sections(id) ON DELETE CASCADE,
    field_path      TEXT NOT NULL,
                    -- "experience.items[0].bullets[1]" kabi JSON path
    prompt          TEXT NOT NULL,
                    -- AI ga yuborilgan asl matn
    suggestions     JSONB NOT NULL,
                    -- [{"text":"...","selected":false}, ...]
    tokens_in       INT,
    tokens_out      INT,
    created_at      TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Voice transcripts (raw audio'lar saqlanmaydi, faqat matn)
CREATE TABLE cv_voice_segments (
    id              UUID PRIMARY KEY,
    cv_id           UUID NOT NULL REFERENCES cv_sessions(id) ON DELETE CASCADE,
    transcript      TEXT NOT NULL,
    parsed_section  VARCHAR(30),
                    -- Qaysi section'ga ajratildi
    parsed_content  JSONB,
                    -- Strukturalashtirilgan natija
    locale          VARCHAR(10) NOT NULL DEFAULT 'en',
    created_at      TIMESTAMP NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_cv_voice_cv ON cv_voice_segments(cv_id, created_at);

-- Rate limiting (CV builder'da AI Improve juda ko'p chaqiriladi)
CREATE TABLE cv_rate_limits (
    user_id         UUID PRIMARY KEY,
    ai_calls_today  INT NOT NULL DEFAULT 0,
    window_start    TIMESTAMP NOT NULL DEFAULT NOW()
);
```

Down migration: barchasini DROP TABLE qiladi (vector extension qoldiriladi).

---

## 5. API Endpoints

Asosiy prefix: `/api/cv/*`

### 5.1 Session CRUD

```
POST   /api/cv/sessions             — yangi CV session
  body: { mode: "scratch"|"import"|"voice", source_file_id?, locale, template_id? }
  resp: { id, status, mode, ... }  (201)

GET    /api/cv/sessions             — user CV'lari ro'yxati
  resp: [{id, title, template_id, updated_at, status}, ...]

GET    /api/cv/sessions/<id>        — to'liq CV
  resp: { ...session, sections: [...], ats_score?: 78 }

PATCH  /api/cv/sessions/<id>        — meta update (title, template, locale)
  body: { title?, template_id?, locale? }

DELETE /api/cv/sessions/<id>
```

### 5.2 Section operations

```
POST   /api/cv/sessions/<id>/sections           — yangi section qo'shish
  body: { section_type, sort_order?, content }

GET    /api/cv/sessions/<id>/sections           — barcha section'lar
PATCH  /api/cv/sessions/<id>/sections/<sid>     — content update
DELETE /api/cv/sessions/<id>/sections/<sid>

POST   /api/cv/sessions/<id>/sections/reorder   — drag-drop tartib
  body: { order: ["section_id1", "section_id2", ...] }
```

### 5.3 AI actions

```
POST   /api/cv/sessions/<id>/improve            — bitta field'ni yaxshilash
  body: {
    field_path: "experience.items[0].bullets[1]",
    current_text: "menejer bo'lib ishladim",
    context?: { job_title, industry, ... }  // optional context
  }
  resp: {
    suggestions: [
      { text: "Led a 5-person engineering team...", style: "metric-driven" },
      { text: "Managed cross-functional team...", style: "scope-focused" },
      { text: "Owned engineering delivery...", style: "ownership-focused" }
    ],
    tokens_used: 250
  }

POST   /api/cv/sessions/<id>/improve-all        — butun CV'ni yaxshilash (premium)
  resp: { task_id }  // asynq task, status polling kerak

POST   /api/cv/sessions/<id>/voice              — voice transcript parsing
  body: { transcript: "...", section_hint?: "experience", locale: "uz" }
  resp: {
    parsed_sections: [
      { section_type: "experience", content: {...}, confidence: 0.92 }
    ]
  }

POST   /api/cv/sessions/<id>/translate          — CV'ni boshqa tilga tarjima
  body: { target_locale: "ru" }
  resp: { task_id }  // asynq, all sections re-translated
```

### 5.4 ATS scoring

```
POST   /api/cv/sessions/<id>/ats-check          — vakansiya bilan moslik tekshirish
  body: { job_title, job_description }
  resp: {
    run_id,
    score: 78,
    missing_keywords: ["docker", "k8s"],
    weak_bullets: [
      { section_id, item_index, text, suggestion: "Add quantifiable metric" }
    ],
    format_issues: [],
    suggestions: ["Add a Projects section", "Quantify the Q3 results"]
  }

GET    /api/cv/sessions/<id>/ats-runs           — ATS run tarixi
```

### 5.5 Template + Export

```
GET    /api/cv/templates                        — barcha template'lar
  resp: [{id, name, preview_url, is_ats_friendly, category}, ...]

POST   /api/cv/sessions/<id>/export             — eksport (PDF/DOCX/TXT)
  body: { format: "pdf"|"docx"|"txt", template_id? }
  resp: { task_id }  // asynq
  → GET /api/cv/sessions/<id>/export-status?task_id=... → {status, file_id?}

GET    /api/cv/sessions/<id>/preview            — jonli HTML preview
  resp: text/html (rendered template + content)
```

### 5.6 Import (parsing uploaded CV)

```
POST   /api/cv/import                           — fayl → CV
  body: { file_id, locale?: "auto" }
  resp: { cv_id, task_id }  // asynq task starts
  → polling: GET /api/cv/sessions/<id>  // status: parsing → ready
```

### 5.7 Chat integration

```
POST   /api/cv/sessions/<id>/chat-link          — mavjud /chat'da CV bilan suhbat
  resp: { chat_session_id }
  → User /chat?session=<chat_session_id> ga otadi
```

---

## 6. Go Service Architecture

### 6.1 Service interfaces

```go
// service/cv/cv.go
package cv

import (
    "context"
    "convertpdfgo/api/models"
)

type Service interface {
    // Sessions
    CreateSession(ctx context.Context, userID string, req CreateRequest) (*models.CVSession, error)
    GetSession(ctx context.Context, id string) (*models.CVSessionDetail, error)
    ListSessions(ctx context.Context, userID string, limit int) ([]models.CVSession, error)
    UpdateSession(ctx context.Context, id string, patch UpdatePatch) error
    DeleteSession(ctx context.Context, id string) error

    // Sections (CRUD)
    AddSection(ctx context.Context, cvID string, sec models.CVSection) error
    UpdateSection(ctx context.Context, sectionID string, content map[string]any) error
    DeleteSection(ctx context.Context, sectionID string) error
    ReorderSections(ctx context.Context, cvID string, order []string) error

    // AI
    ImproveField(ctx context.Context, cvID, fieldPath, currentText string, ctxData map[string]any) ([]Suggestion, error)
    ImproveAll(ctx context.Context, cvID string) (taskID string, err error)
    TranslateCV(ctx context.Context, cvID, targetLocale string) (taskID string, err error)

    // Voice
    ParseVoice(ctx context.Context, cvID, transcript, sectionHint, locale string) ([]ParsedSection, error)

    // ATS
    RunATSCheck(ctx context.Context, cvID, jobTitle, jobDescription string) (*models.ATSRun, error)

    // Import (asynq-backed)
    ImportCV(ctx context.Context, userID, fileID, locale string) (cvID, taskID string, err error)
    ProcessImport(ctx context.Context, cvID string) error  // worker entry

    // Export (asynq-backed)
    EnqueueExport(ctx context.Context, cvID string, format string, templateID string) (taskID string, err error)
    ProcessExport(ctx context.Context, cvID, format, templateID string) (fileID string, err error)

    // Render
    RenderPreviewHTML(ctx context.Context, cvID string) (string, error)
}
```

### 6.2 Sub-services / helpers

```
service/cv/
├── service.go         — Service interface + main implementation
├── parser.go          — Uploaded file → structured CV (Gemini JSON mode)
├── improver.go        — AI Improve field — 3 ta variant
├── voice.go           — Transcript → section parse
├── ats.go             — ATS score algorithm + Gemini hints
├── translator.go      — CV → boshqa tilga
├── exporter.go        — HTML → PDF / DOCX / TXT
├── renderer.go        — HTML template + content → final HTML
└── templates/         — HTML.tmpl files for each template
    ├── modern.html.tmpl
    ├── classic.html.tmpl
    ├── creative.html.tmpl
    ├── minimal.html.tmpl
    ├── tech.html.tmpl
    ├── executive.html.tmpl
    ├── academic.html.tmpl
    └── ...
```

### 6.3 Models (api/models/cv.go)

```go
type CVSession struct {
    ID           string    `json:"id"`
    UserID       string    `json:"user_id"`
    Title        string    `json:"title"`
    Mode         string    `json:"mode"`
    SourceFileID *string   `json:"source_file_id,omitempty"`
    TemplateID   string    `json:"template_id"`
    Locale       string    `json:"locale"`
    Status       string    `json:"status"`
    ParseError   string    `json:"parse_error,omitempty"`
    LastPDFID    *string   `json:"last_pdf_id,omitempty"`
    CreatedAt    time.Time `json:"created_at"`
    UpdatedAt    time.Time `json:"updated_at"`
}

type CVSection struct {
    ID          string         `json:"id"`
    CVID        string         `json:"cv_id"`
    SectionType string         `json:"section_type"`
    SortOrder   int            `json:"sort_order"`
    Content     map[string]any `json:"content"`
    UpdatedAt   time.Time      `json:"updated_at"`
}

type CVSessionDetail struct {
    CVSession
    Sections []CVSection `json:"sections"`
    ATSScore *int        `json:"ats_score,omitempty"`
}

type Suggestion struct {
    Text  string `json:"text"`
    Style string `json:"style"`  // "metric-driven" | "scope-focused" | "ownership-focused"
}

type ATSRun struct {
    ID              string         `json:"id"`
    CVID            string         `json:"cv_id"`
    JobTitle        string         `json:"job_title"`
    JobDescription  string         `json:"job_description"`
    Score           int            `json:"score"`
    Feedback        map[string]any `json:"feedback"`
    CreatedAt       time.Time      `json:"created_at"`
}
```

---

## 7. CV Parser (Upload → Structured Data)

### 7.1 Pipeline

```
Uploaded file (PDF / DOCX / JPG / PNG)
   ↓
File type detect (mime sniffing)
   ↓
[Branch] PDF       → pdftotext (mavjud service/pdfchat/extract.go)
         DOCX      → libreoffice --convert-to txt
         JPG/PNG   → ocrmypdf → pdftotext → text  (mavjud OCR pipeline)
         Scanned   → tesseract direct
   ↓
Raw text (taxminan 500-5000 char)
   ↓
Gemini JSON mode (structured output)
   System prompt: "You are a CV parser. Extract..."
   Output schema: { personal, summary, experience[], education[], ... }
   ↓
Structured CV JSON
   ↓
DB: cv_sections rows yaratish
   ↓
Embedding: har section uchun Gemini embed_content
   ↓
status='ready'
```

### 7.2 Gemini parser prompt (system instruction)

```
You are a CV/resume parser. Your job is to extract structured data from
raw CV text and return it as JSON. Follow this schema strictly:

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
  "summary": "<2-3 sentences professional summary>",
  "experience": [
    {
      "title": "Senior Engineer",
      "company": "XYZ Corp",
      "period": "Jan 2022 - Present",
      "location": "Remote",
      "bullets": ["...", "..."]
    }
  ],
  "education": [
    {
      "degree": "BSc Computer Science",
      "school": "Tashkent University of IT",
      "period": "2018 - 2022",
      "gpa": "3.8/4.0",
      "honors": ""
    }
  ],
  "skills": {
    "technical": ["Go", "TypeScript", "Postgres"],
    "soft": ["Leadership", "Communication"]
  },
  "languages": [
    { "name": "English", "level": "Fluent" },
    { "name": "Uzbek", "level": "Native" }
  ],
  "projects": [...],
  "awards": [...],
  "certifications": [...]
}

Rules:
- Use the user's exact wording for bullets; don't paraphrase.
- If a field is unclear, set it to empty string (don't invent).
- Detect the CV's source language; respond in same language.
- For dates, normalize to "MMM YYYY - MMM YYYY" format.
- Group skills as best you can (technical vs soft).
```

### 7.3 Worker task

Asynq task: `cv:import_parse`

```go
// pkg/queue/tasks.go
const TaskCVImport = "cv:import_parse"

// pkg/queue/handlers.go
mux.HandleFunc(TaskCVImport, h.runByID("cv_import", func(ctx, id string) error {
    return h.services.CV().ProcessImport(ctx, id)
}))
```

---

## 8. Template Rendering

### 8.1 HTML template strategy

Har template — Go `text/template` faylida. Tailwind CSS inline classes ishlatamiz (Gotenberg Chromium yaxshi qabul qiladi).

Misol: `service/cv/templates/modern.html.tmpl`

```html
<!DOCTYPE html>
<html lang="{{.Locale}}">
<head>
  <meta charset="UTF-8">
  <style>{{.TailwindCSS}}</style>
  <title>{{.Personal.Name}} - {{.Personal.Title}}</title>
</head>
<body class="font-sans bg-white p-8 max-w-3xl mx-auto">
  <header class="border-b-2 border-orange-500 pb-4">
    <h1 class="text-3xl font-bold">{{.Personal.Name}}</h1>
    <p class="text-lg text-gray-600">{{.Personal.Title}}</p>
    <div class="flex gap-4 mt-2 text-sm text-gray-500">
      <span>{{.Personal.Email}}</span>
      <span>{{.Personal.Phone}}</span>
      <span>{{.Personal.Location}}</span>
    </div>
  </header>

  {{if .Summary}}
  <section class="mt-6">
    <h2 class="text-xl font-bold border-b mb-2">Summary</h2>
    <p class="text-sm leading-relaxed">{{.Summary}}</p>
  </section>
  {{end}}

  {{if .Experience}}
  <section class="mt-6">
    <h2 class="text-xl font-bold border-b mb-2">Experience</h2>
    {{range .Experience}}
    <div class="mt-3">
      <div class="flex justify-between">
        <strong>{{.Title}}</strong>
        <span class="text-sm text-gray-500">{{.Period}}</span>
      </div>
      <div class="text-sm text-gray-600">{{.Company}} · {{.Location}}</div>
      <ul class="mt-1 list-disc list-inside text-sm">
        {{range .Bullets}}<li>{{.}}</li>{{end}}
      </ul>
    </div>
    {{end}}
  </section>
  {{end}}

  ...
</body>
</html>
```

### 8.2 Template list (MVP — 10 ta)

| ID | Name | Category | ATS | Description |
|---|---|---|---|---|
| `modern` | Modern | modern | ✓ | Orange accent, clean sans-serif (default) |
| `classic` | Classic | classic | ✓ | Times New Roman, double-column header |
| `minimal` | Minimal | classic | ✓ | Black & white, narrow margins |
| `tech` | Tech | modern | ✓ | Monospace headers, GitHub-style |
| `creative` | Creative | creative | ✗ | Color blocks, visual hierarchy |
| `executive` | Executive | classic | ✓ | Serif, large name, formal |
| `academic` | Academic | classic | ✓ | Heavy on publications + research |
| `designer` | Designer | creative | ✗ | Bold typography, asymmetric layout |
| `two-col` | Two Column | modern | ✓ | Skills/contact sidebar + main |
| `one-col` | One Column | modern | ✓ | Single column, maximum text density |

### 8.3 Gotenberg call

```go
// service/cv/exporter.go
func (e *exporter) HTMLToPDF(ctx context.Context, html string) ([]byte, error) {
    return e.gotenberg.HTMLToPDFFromString(ctx, html, gotenberg.HTMLToPDFOpts{
        PageSize: "A4",
        Margin:   "0",  // Template'da margin'lar ichkarida
    })
}
```

Mavjud `pkg/gotenberg/clint.go` ga `HTMLToPDFFromString` qo'shish kerak (hozir faqat fayl yo'lini qabul qiladi).

---

## 9. AI Improve (Gemini)

### 9.1 Improve flow

```
User clicks "✨ Improve with AI" on a bullet
   ↓
POST /api/cv/sessions/<id>/improve
  body: { field_path: "experience.items[0].bullets[1]", current_text: "..." }
   ↓
Backend: load CV context (job title, industry from other sections)
   ↓
Gemini chat with system prompt:
  "You are an expert resume writer. Improve this bullet point in 3 styles:
   1) metric-driven  2) scope-focused  3) ownership-focused
   Keep it under 25 words. Use action verbs. Quantify if possible."
   ↓
3 ta variant qaytadi
   ↓
Save to cv_ai_history
   ↓
Response: [{text, style}, ...]
   ↓
Frontend: pop-up panel shows 3 cards, user picks one
```

### 9.2 Rate limiting

CV builder'da AI Improve har bullet uchun chaqirilishi mumkin (20+ marta bitta sessiya'da). Limit:
- Free tier: 50 AI calls / kun
- Signed-in: 200 / kun
- Premium (kelajak): cheksiz

`cv_rate_limits` jadvali bilan kuzatib boriladi.

---

## 10. Voice Mode

### 10.1 Frontend (Web Speech API)

```typescript
// /cv-builder/components/voice-recorder.tsx
const recognition = new webkitSpeechRecognition();
recognition.continuous = true;
recognition.interimResults = true;
recognition.lang = "uz-UZ"; // user locale

recognition.onresult = (event) => {
  const transcript = event.results[event.results.length - 1][0].transcript;
  setTranscript(transcript);
  // Har 5-10 soniyada buffer'ni backend'ga yuborish
  if (event.results[event.results.length - 1].isFinal) {
    parseVoiceSegment(transcript);
  }
};
```

### 10.2 Backend parse

```
POST /api/cv/sessions/<id>/voice
  body: { transcript, section_hint?, locale }
   ↓
Gemini structured output:
  System prompt: "User is dictating their CV. Extract the section they're
                  describing. Possible sections: personal, summary,
                  experience, education, skills, languages, projects.
                  Return JSON: { section_type, content }"
   ↓
Parsed result → cv_voice_segments table
   ↓
If section_type already exists in cv_sections, append/merge
If new, create new cv_sections row
   ↓
Return parsed_section to client → UI updates editor in real-time
```

### 10.3 Voice locale support

Browser SpeechRecognition supports 30+ tillar:
- uz-UZ (Uzbek)
- en-US (English)
- ru-RU (Russian)
- ar-SA (Arabic)
- tr-TR (Turkish)
- ko-KR, ja-JP, zh-CN, hi-IN, va h.k.

User locale tanlasa, recognition.lang shu bo'yicha sozlanadi.

---

## 11. ATS Score Algorithm

### 11.1 Score components (0-100)

```
Total = 0.4 × keyword_match + 0.3 × format_quality + 0.3 × content_quality

keyword_match (0-100):
  - Job description'dan TF-IDF keywords ajratish
  - CV ichida shu keyword'lar borligini tekshirish
  - Match: 100 × (CV'da topilgan / Job'da bo'lgan) — top 30 keyword

format_quality (0-100):
  ✓ Has Personal section (10)
  ✓ Has email + phone (10)
  ✓ Standard section headers (10)
  ✓ No tables/columns (template ATS-friendly bo'lsa 20)
  ✓ Plain fonts (15)
  ✓ Date format consistent (10)
  ✓ No graphics in headers (15)
  ✓ Page count reasonable (≤2 pages) (10)

content_quality (0-100):
  ✓ Each bullet has action verb (20)
  ✓ Quantified results (numbers, %) (30)
  ✓ Bullet length reasonable (12-25 words) (20)
  ✓ No passive voice (15)
  ✓ Skills section non-empty (15)
```

### 11.2 Implementation

```go
// service/cv/ats.go
func (a *ats) Score(ctx context.Context, cv *models.CVSessionDetail, jobDesc string) (*models.ATSRun, error) {
    // 1. Job description'dan keywords ajratish (Gemini'ga so'rovni yuborish)
    keywords := a.extractKeywords(ctx, jobDesc)
    // → ["docker", "kubernetes", "go", "microservices", ...]

    // 2. CV ichida shu keyword'lar borligini tekshirish
    cvText := flattenCV(cv)
    matchScore := scoreKeywordMatch(keywords, cvText)

    // 3. Format quality (rule-based)
    formatScore := scoreFormat(cv)

    // 4. Content quality (rule-based + Gemini hint)
    contentScore, weakBullets := a.scoreContent(ctx, cv)

    total := int(0.4*float64(matchScore) + 0.3*float64(formatScore) + 0.3*float64(contentScore))

    feedback := map[string]any{
        "missing_keywords": filterMissing(keywords, cvText),
        "weak_bullets": weakBullets,
        "format_issues": findFormatIssues(cv),
        "suggestions": a.aiSuggestions(ctx, cv, jobDesc),
    }

    return &models.ATSRun{
        ID: uuid.NewString(),
        CVID: cv.ID,
        JobTitle: extractJobTitle(jobDesc),
        JobDescription: jobDesc,
        Score: total,
        Feedback: feedback,
        CreatedAt: time.Now(),
    }, nil
}
```

---

## 12. Frontend Layout (Next.js)

### 12.1 Yangi CAREER kategoriyasi

CV Builder AI-POWERED bo'limining ichida emas, **alohida CAREER kategoriya**'da yashaydi —
SEO (har sub-tool alohida sahifa) va UX (user "career" tool'larni ajratib topadi) uchun.

`packages/data/src/categories.ts` (yoki `tools.ts`'da inline):

```typescript
export const categories = [
  { slug: "essentials", name: "Essentials", icon: "Wrench" },
  { slug: "convert",    name: "Convert",    icon: "ArrowLeftRight" },
  { slug: "edit",       name: "Edit",       icon: "PenTool" },
  { slug: "secure",     name: "Secure",     icon: "ShieldCheck" },
  { slug: "ai",         name: "AI-Powered", icon: "Sparkles" },
  { slug: "career",     name: "Career",     icon: "GraduationCap" },  // ← yangi
];
```

`packages/data/src/tools.ts` — 8 ta CAREER tool:

```typescript
// Asosiy CV Builder + 7 ta sub-tool (har biri alohida sahifa, SEO uchun)
{ slug: "cv-builder",        name: "AI Resume Builder",  category: "career", badge: "new",
  description: "Build, redesign, or talk to your CV — in 20 languages." },
{ slug: "cv-from-pdf",       name: "PDF → Resume",        category: "career", badge: "new",
  description: "Scan or upload an existing CV, redesign it with AI." },
{ slug: "cv-voice",          name: "Voice → Resume",      category: "career", badge: "new",
  description: "Talk through your career, AI structures it into a CV." },
{ slug: "ats-check",         name: "ATS Checker",         category: "career",
  description: "Score your CV against any job description." },
{ slug: "resume-translate",  name: "Resume Translator",   category: "career",
  description: "One CV in 20 languages, native quality." },
{ slug: "cover-letter",      name: "Cover Letter",        category: "career",
  description: "AI cover letter from your CV + job description." },
{ slug: "resume-to-docx",    name: "Resume → DOCX",       category: "career",
  description: "Editable Word from your PDF resume." },
{ slug: "chat-with-cv",      name: "Chat with CV",        category: "career",
  description: "Ask AI about your own resume — reuses /chat pipeline." },
```

### 12.2 Backend reality vs marketing surface

Garchi marketing'da **8 ta tool** ko'rinsa ham, backend'da hammasi **bitta `/cv-builder`
infrastrukturani** ishlatadi:

| Marketing tool sahifa | Backend reuse |
|---|---|
| `/cv-builder` | Full implementation (PRD'dagi hamma narsa) |
| `/cv-from-pdf` | `cv-builder?mode=import` — bir xil sahifa, prefilled mode |
| `/cv-voice` | `cv-builder?mode=voice` — bir xil sahifa, prefilled mode |
| `/ats-check` | Faqat ATS panel + upload, mavjud `service/cv/ats.go` reuse |
| `/resume-translate` | Mavjud `service/cv/translator.go` reuse |
| `/cover-letter` | Yangi sub-tool, Sprint 2 |
| `/resume-to-docx` | Mavjud Gotenberg PDF→DOCX reuse |
| `/chat-with-cv` | Mavjud `/chat` infra reuse |

Ya'ni `Sprint 1` da bitta katta `/cv-builder` tool yetadi — qolgan 7 ta sahifa SEO uchun
sub-page'lar bo'lib qoladi (har birida o'sha tool'ning relevant mode'i prefilled).

### 12.2 Component structure

```
apps/web/app/(tools)/cv-builder/
├── page.tsx                          — Server component, ToolPageShell wrapper
├── cv-builder-client.tsx             — Main client (mode router)
└── components/
    ├── landing.tsx                   — 3 CTA card (Upload/Start/Voice)
    ├── editor/
    │   ├── editor-layout.tsx         — 3-column layout
    │   ├── section-sidebar.tsx       — Chap: section list + ATS
    │   ├── section-editor.tsx        — Markaz: faol section form
    │   ├── pdf-preview.tsx           — O'ng: jonli HTML preview
    │   ├── improve-popover.tsx       — "✨ Improve with AI" modal
    │   ├── template-modal.tsx        — Template gallery
    │   └── ats-panel.tsx             — ATS score widget
    ├── voice/
    │   ├── voice-recorder.tsx        — Web Speech API
    │   └── voice-feedback.tsx        — Live transcript + parsed result
    ├── wizard/
    │   ├── wizard-steps.tsx          — Start-from-scratch step UI
    │   └── step-suggestions.tsx      — AI suggestion per step
    └── export-modal.tsx              — PDF/DOCX/TXT export options
```

### 12.3 Hooks

```
apps/web/lib/cv/
├── api.ts                            — CV CRUD + AI + export wrappers
├── types.ts                          — CVSession, CVSection, Suggestion types
├── use-cv-session.ts                 — Session state hook
├── use-improve.ts                    — AI Improve hook (handles 3-variant popover)
├── use-voice.ts                      — Web Speech API + buffer
├── use-ats.ts                        — ATS score polling
└── use-export.ts                     — Export task polling
```

### 12.4 Routes

Mavjud authGate (AI tool sifatida) — `/cv-builder` ham AI bo'limida, login talab qiladi.

---

## 13. Cost & Quota

### 13.1 Per CV cost estimate (Gemini Flash Lite)

| Operatsiya | Tokenlar | Narx (Free / Paid) |
|---|---|---|
| Import parse | ~3000 in, 1500 out | $0 / $0.013 |
| Improve bullet (3 variants) | ~500 in, 300 out | $0 / $0.003 |
| ATS check | ~2500 in, 1000 out | $0 / $0.009 |
| Translate CV | ~5000 in, 5000 out | $0 / $0.030 |
| Voice parse (per segment) | ~300 in, 200 out | $0 / $0.002 |
| Per session avg | 10K in, 5K out | $0 / $0.045 |

Free tier: ~1500 req/kun → ~50 to'liq CV session/kun bemalol.

### 13.2 Rate limits

```
Free + signed-in user:
- 50 AI calls / kun
- 5 ATS checks / kun
- 1 Translate / kun
- 5 voice parse / kun

Premium (kelajak):
- Cheksiz hammasi
- Priority queue
- Custom templates
```

---

## 14. Implementation Phases (3-4 hafta)

### 🟢 Hafta 1: Backend foundation

| Kun | Vazifa |
|---|---|
| D1 | Migration: 6 ta jadval (`cv_sessions`, `cv_sections`, `cv_templates`, `cv_ats_runs`, `cv_ai_history`, `cv_voice_segments`, `cv_rate_limits`) |
| D2 | Models + storage layer (`storage/postgres/cv.go`) |
| D3 | Service skeleton (`service/cv/service.go`) + handlers (`api/handler/cv.go`) |
| D4 | Routes (`api/routes_cv.go`) — to'liq CRUD |
| D5 | Parser (`service/cv/parser.go`) — PDF/DOCX/Image → Gemini → JSON |
| D6 | Asynq task: `cv:import_parse` + worker handler |
| D7 | Smoke test: upload CV → parse → verify sections |

### 🟡 Hafta 2: AI + Templates

| Kun | Vazifa |
|---|---|
| D8 | Improver (`service/cv/improver.go`) — 3 variant generation |
| D9 | ATS scorer (`service/cv/ats.go`) — keyword + format + content |
| D10 | Translator (`service/cv/translator.go`) — all sections |
| D11 | 5 ta HTML template (modern, classic, minimal, tech, creative) |
| D12 | Renderer + Exporter (HTML → Gotenberg → PDF) |
| D13 | DOCX export (Gotenberg PDF→DOCX) + TXT (markdown) |
| D14 | Smoke test: full CV → 5 ta template'da PDF |

### 🔵 Hafta 3: Frontend MVP

| Kun | Vazifa |
|---|---|
| D15 | Tool data + landing page (3 CTA card) |
| D16 | Editor layout (3-column) |
| D17 | Section editor + section sidebar |
| D18 | AI Improve popover (3-variant UI) |
| D19 | Template modal + apply |
| D20 | PDF preview (iframe + blob URL — mavjud pattern) |
| D21 | Export modal + download |

### 🟣 Hafta 4: Voice + Polish + Sprint 2 prep

| Kun | Vazifa |
|---|---|
| D22 | Voice recorder (Web Speech API) |
| D23 | Voice → parse → editor update flow |
| D24 | ATS panel + Run ATS Check modal |
| D25 | Wizard (start-from-scratch step-by-step) |
| D26 | "Chat with my CV" link to existing `/chat` |
| D27 | Mobile responsive + i18n strings |
| D28 | Production polish + rate limiting + monitoring |

### Sprint 2 (keyingi rejada)

- LinkedIn import
- Cover letter generator
- Public share link
- More templates (5+ qo'shimcha)
- Premium tier (Stripe integration)
- API endpoint for developers

---

## 15. Acceptance Criteria (MVP)

CV Builder MVP "tugadi" deyish uchun:

- [ ] User uploaded PDF CV → 5-10 soniyada parsed va editor ochildi
- [ ] Har section type uchun edit form ishlaydi
- [ ] AI Improve har bullet uchun 3 variant qaytaradi (1 soniyadan past)
- [ ] 5 ta template birgalikda ishlaydi, content saqlanadi
- [ ] PDF export tugaganda fayl 5 sek ichida tayyor
- [ ] Voice mode Uzbek tilida ishlaydi (qisqa demo dictation)
- [ ] ATS score 0-100 oralig'ida, vakansiya'ga moslab
- [ ] Multi-language: CV uz/en/ru'da to'liq ishlaydi
- [ ] AuthGate: faqat logged-in user'lar uchun
- [ ] Mobile (375px) layout ishlaydi
- [ ] 95-percentile latency: improve <2s, ATS <5s, export <8s

---

## 16. Risks & Mitigations

| Xavf | Ehtimol | Mitigatsiya |
|---|---|---|
| Gemini parse'da CV'ni noto'g'ri ajratadi | O'rta | "Try again" + manual edit har section'da; structured output (JSON mode) majburiy |
| AI Improve narxi ko'p chaqiruvda oshib ketadi | Yuqori | Rate limit (50/kun free) + cache (xuddi shu input → cached output) |
| Template render Gotenberg'da timeout | Past | HTML kichik, Gotenberg standartda 30s timeout — yetadi |
| ATS algorithm noto'g'ri keywords chiqaradi | O'rta | Manual override — user keyword qo'shishi mumkin |
| Voice recognition Uzbek'da sifat past | Yuqori | Backup: Whisper API (paid) keyin qo'shish, MVP'da iltimos qilib qoldiramiz |
| Cyrillic / Arabic / Hindi templatelarda hijja muammosi | O'rta | Tailwind font stack to'g'rilash, har locale uchun font-family override |
| User CV'ni juda uzun yozsa, embedding price oshadi | Past | Section maksimal hajmi (5K char per bullet group) |
| Premium feature qachon kerak — bilmaymiz | O'rta | MVP free, 2-3 oydan keyin usage'ga qarab premium qo'shish |

---

## 17. Success Metrics

Launch'dan keyin 2 hafta ichida:

| Metrik | Target |
|---|---|
| Daily new CVs created | >30 |
| Conversion: landing → editor | >40% |
| AI Improve clicks per session | >5 |
| PDF export rate | >70% sessions end with export |
| Avg session duration | 12-20 min |
| Repeat usage (≥2 CV per user) | >25% |
| Voice mode usage | >10% sessions (high if Uzbek users) |
| ATS check usage | >40% sessions |
| Template gallery views | >60% sessions |

Tracking: mavjud Prometheus + Grafana dashboard.

---

## 18. Out of Scope (v1.0)

Sprint 2+ ga qoldiramiz:

- [ ] LinkedIn profile API import (avtomatik scrape)
- [ ] Cover letter generator
- [ ] Public share link (`/cv/share/<token>`)
- [ ] Premium tier billing (Stripe)
- [ ] Custom templates (user dizayni)
- [ ] Multiple resume variants per user
- [ ] CV comparison tool ("v2 vs v1 bullet-by-bullet diff")
- [ ] Real-time collaboration (Liveblocks)
- [ ] Mobile apps (iOS/Android)
- [ ] API for third-party developers
- [ ] White-label for HR companies

---

## 19. Open Questions

1. **Voice quality**: Web Speech API Uzbek'da qancha aniqligini test qilish kerak. Agar past bo'lsa, Whisper-as-fallback'ni MVP'ga kiritamizmi?
2. **Storage retention**: CV draft'lar abadiy saqlanadimi yoki 1 yildan keyin avtomatik o'chiriladimi?
3. **Privacy**: CV'larga vector embedding qo'yamiz (semantic search uchun) — bu user'ning shaxsiy ma'lumotlarini "ko'rsatadi"mi? GDPR ko'z bilan tekshirish kerak.
4. **Free tier**: 50 AI calls/kun chegara mantiqiymi? Real launch'dan keyin tuzatamiz.
5. **Default locale**: Browser navigator.language'dan oladi'mi yoki user profil'dan?
6. **Template ownership**: User custom HTML template qila olishi kerakmi? (Sprint 2)
7. **Share link**: Public link'lar SEO friendly bo'lsinmi (`/cv/zarif`) yoki uuid (`/cv/share/abc-123`)?

---

## 20. Approval & Next Steps

PRD tasdiqlangach:

1. ✅ Bu PRD'ni o'qib chiqing va savollarni `## 19. Open Questions`'da belgilang
2. ✅ Quyidagilarni tasdiqlang:
   - 10 ta template ro'yxati (modern / classic / ... / one-col)
   - ATS algoritm formulasi (0.4 keyword + 0.3 format + 0.3 content)
   - Free tier limits (50 AI calls / 5 ATS / 1 Translate / 5 voice / kun)
3. ⏭️ **D1 — Migration**'dan boshlaymiz

---

**Document version**: 1.0 · 2026-05-28
**Linked**:
- [PDF_CHAT_AI_PRD.md](PDF_CHAT_AI_PRD.md) — mavjud chat AI infra (qaytadan ishlatiladi)
- [BAHROM-WIRING.md](BAHROM-WIRING.md) — frontend monorepo
- [PDF_FEATURES.md](PDF_FEATURES.md) — mavjud tool ro'yxati
