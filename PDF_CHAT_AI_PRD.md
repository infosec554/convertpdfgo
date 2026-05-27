# PDF Chat AI — Technical PRD

**Loyiha**: convertpdfgo.com `/chat` tool — to'liq AI-powered PDF chat
**Versiya**: v1.0 (MVP launch)
**LLM**: Groq (Llama 3.3 70B Versatile) — free tier
**Embedding**: Google Gemini `text-embedding-004` — free tier (1500/kun)
**Vector DB**: PostgreSQL + pgvector (mavjud postgres instance)
**Sana**: 2026-05-27
**Author**: Zarif (infosec554)

---

## 1. Executive Summary

### Muammo
Sayt'da `/chat` sahifa **mockup sifatida bor** — upload tugmasi, savol input'i ko'rinadi, lekin **hech narsa ishlamaydi**. Backend'da chat-related kod yo'q. Convertpdfgo.com PDF tool'lariga raqobatbardosh bo'lishi uchun ChatPDF/AskYourPDF darajasidagi feature kerak.

### Yechim
RAG (Retrieval-Augmented Generation) arxitekturasi:
1. User PDF yuklaydi → text page-by-page chiqarib olinadi
2. Text chunk'larga bo'linadi (500-1000 belgi, 100 belgi overlap)
3. Har chunk Gemini bilan embedding'ga aylantiriladi
4. pgvector'da saqlanadi
5. User savol berganda → savol embedding → eng yaqin 5-7 chunk topiladi → Groq Llama 70B'ga yuboriladi → javob streaming bilan qaytadi + page citation'lar

### Asosiy texnologik tanlovlar
| Komponent | Tanlangan | Sabab |
|---|---|---|
| **LLM** | Groq Llama 3.3 70B | Tekin (14K TPM), juda tez (300+ tok/s), sifat ~80% Claude |
| **Embedding** | Gemini text-embedding-004 | Tekin (1500/kun), 768-dim, sifatli |
| **Vector DB** | pgvector (Postgres) | Allaqachon postgres bor — yangi service kerak emas |
| **PDF extraction** | pdfcpu | Allaqachon backend'da bor (inspect tool ishlatadi) |
| **Streaming** | Server-Sent Events (SSE) | WebSocket'dan oson, HTTP/2 bilan ishlaydi |
| **Frontend** | Mavjud Next.js mockup'ni wire qilish | Mockup yaxshi, faqat ulashish kerak |

### Migratsiya rejasi
Code'da `LLMProvider` interface ishlatamiz. 1 oydan keyin paid'ga o'tish: `.env`'da bitta o'zgaruvchini almashtiramiz (`LLM_PROVIDER=groq` → `claude`). Kod tegmaydi.

---

## 2. User Flow

```
┌─────────────────────────────────────────────────────────────┐
│  1. User /chat sahifaga keladi                              │
│     └─ "Upload PDF" tugmasini bosadi                        │
├─────────────────────────────────────────────────────────────┤
│  2. PDF yuklanadi                                           │
│     └─ POST /file/upload  → file_id                         │
│     └─ POST /api/chat/sessions {file_id} → session_id       │
│         (backend async indexing boshlaydi)                  │
│     └─ Frontend "Indexing PDF..." progress bar              │
├─────────────────────────────────────────────────────────────┤
│  3. Backend indexing (async, ~5-15s 50-page PDF uchun)      │
│     ├─ pdfcpu: PDF → page-by-page text                      │
│     ├─ Chunker: text → 500-1000 char chunk'lar              │
│     ├─ Gemini API: har chunk → 768-dim vector               │
│     ├─ pgvector: chunk + vector saqlash                     │
│     └─ Done — session status = "ready"                      │
├─────────────────────────────────────────────────────────────┤
│  4. Avto-summary va suggested questions                     │
│     └─ Groq: "Bu PDF haqida 2 jumla summary + 5 ta savol"   │
│     └─ Frontend sidebar'ga ko'rsatiladi                     │
├─────────────────────────────────────────────────────────────┤
│  5. User savol beradi                                       │
│     └─ POST /api/chat/sessions/<id>/messages (SSE)          │
│     ├─ Backend: savol → embedding → top-7 chunk retrieval   │
│     ├─ Groq: chunk'lar + savol → streaming javob            │
│     ├─ SSE: token-by-token UI'ga keladi                     │
│     └─ Javobda [page 4] kabi citation'lar                   │
├─────────────────────────────────────────────────────────────┤
│  6. Citation bosilganda                                     │
│     └─ PDF viewer (pdf.js) shu page'ga jump qiladi          │
│     └─ Highlight: javobga eng yaqin paragraf                │
└─────────────────────────────────────────────────────────────┘
```

---

## 3. Architecture

```
                    ┌──────────────────────────────────┐
                    │       Browser (Next.js)          │
                    │  /chat sahifa + pdf.js viewer    │
                    └──────┬───────────────────────────┘
                           │ HTTPS (SSE for streaming)
                           ▼
                    ┌──────────────────────────────────┐
                    │     Caddy (already running)       │
                    └──────┬───────────────────────────┘
                           │
                    ┌──────▼───────────────────────────┐
                    │  convertpdfgo_api (Go/Gin)        │
                    │  /api/chat/* yangi handler'lar    │
                    └──────┬─────────────┬─────────────┘
                           │             │
              ┌────────────┘             └─────────────┐
              ▼                                        ▼
       ┌──────────────┐                       ┌─────────────────┐
       │  Postgres    │                       │ convertpdfgo_   │
       │  + pgvector  │                       │ worker (async)  │
       │              │                       │ indexing tasks  │
       │ - chat_      │                       └────┬────────────┘
       │   sessions   │                            │
       │ - chat_      │                            ▼
       │   messages   │                       ┌─────────────────┐
       │ - pdf_chunks │                       │ External APIs   │
       │   (vectors)  │                       │ ┌─────────────┐ │
       └──────────────┘                       │ │ Groq        │ │
                                              │ │ (LLM)       │ │
                                              │ └─────────────┘ │
                                              │ ┌─────────────┐ │
                                              │ │ Gemini      │ │
                                              │ │ (embedding) │ │
                                              │ └─────────────┘ │
                                              └─────────────────┘
```

---

## 4. Database Schema

### 4.1 pgvector extension
Postgres'ga pgvector qo'shamiz. Migration: `migrations/postgres/0NNN_pdf_chat_ai.up.sql`

```sql
-- Enable pgvector
CREATE EXTENSION IF NOT EXISTS vector;

-- Chat sessiyalar — bir PDF = bir session
CREATE TABLE chat_sessions (
    id          UUID PRIMARY KEY,
    user_id     UUID REFERENCES users(id),       -- NULL → guest user
    file_id     UUID NOT NULL REFERENCES files(id) ON DELETE CASCADE,
    title       TEXT,                            -- Avto-generated PDF title
    summary     TEXT,                            -- Avto TL;DR
    status      VARCHAR(20) NOT NULL DEFAULT 'indexing'
                CHECK (status IN ('indexing','ready','failed')),
    error       TEXT,
    created_at  TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_chat_sessions_user ON chat_sessions(user_id, created_at DESC);
CREATE INDEX idx_chat_sessions_file ON chat_sessions(file_id);

-- Chat messages
CREATE TABLE chat_messages (
    id          UUID PRIMARY KEY,
    session_id  UUID NOT NULL REFERENCES chat_sessions(id) ON DELETE CASCADE,
    role        VARCHAR(10) NOT NULL CHECK (role IN ('user','assistant','system')),
    content     TEXT NOT NULL,
    citations   JSONB,                           -- [{"page":4, "chunk_id":"..."}]
    tokens_in   INT,                             -- cost tracking
    tokens_out  INT,
    created_at  TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_chat_messages_session ON chat_messages(session_id, created_at);

-- PDF chunks bilan embedding'lar
CREATE TABLE pdf_chunks (
    id          UUID PRIMARY KEY,
    session_id  UUID NOT NULL REFERENCES chat_sessions(id) ON DELETE CASCADE,
    page_number INT NOT NULL,                    -- 1-indexed
    chunk_index INT NOT NULL,                    -- 0-indexed within page
    content     TEXT NOT NULL,
    embedding   vector(768),                     -- Gemini text-embedding-004 dim
    token_count INT,
    created_at  TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Vector similarity index (HNSW — eng yaxshi for read-heavy)
CREATE INDEX idx_pdf_chunks_embedding ON pdf_chunks
    USING hnsw (embedding vector_cosine_ops);
CREATE INDEX idx_pdf_chunks_session ON pdf_chunks(session_id, page_number);

-- Suggested questions (avto-generated)
CREATE TABLE chat_suggestions (
    id          UUID PRIMARY KEY,
    session_id  UUID NOT NULL REFERENCES chat_sessions(id) ON DELETE CASCADE,
    question    TEXT NOT NULL,
    sort_order  INT NOT NULL DEFAULT 0
);

-- Rate limiting (guest user'lar uchun)
CREATE TABLE chat_rate_limits (
    ip_or_user  TEXT PRIMARY KEY,                -- IP yoki user_id
    count       INT NOT NULL DEFAULT 0,
    window_start TIMESTAMP NOT NULL DEFAULT NOW()
);
```

### 4.2 Down migration
```sql
DROP TABLE IF EXISTS chat_rate_limits;
DROP TABLE IF EXISTS chat_suggestions;
DROP TABLE IF EXISTS pdf_chunks;
DROP TABLE IF EXISTS chat_messages;
DROP TABLE IF EXISTS chat_sessions;
DROP EXTENSION IF EXISTS vector;
```

---

## 5. API Endpoints

Asosiy prefix: `/api/chat/*`

### 5.1 Session create
```
POST /api/chat/sessions
Content-Type: application/json
Authorization: Bearer <jwt>  (optional)

Request:
{
  "file_id": "uuid"
}

Response 201:
{
  "StatusCode": 201,
  "Data": {
    "id": "session-uuid",
    "file_id": "uuid",
    "status": "indexing",
    "created_at": "2026-05-27T18:00:00Z"
  }
}
```

Backend: session yaratiladi, async task'ga `chat:index` topshiriladi (asynq queue), darhol qaytaradi.

### 5.2 Session status (polling — indexing tugaganini bilish)
```
GET /api/chat/sessions/<id>

Response 200:
{
  "StatusCode": 200,
  "Data": {
    "id": "session-uuid",
    "status": "ready",       // "indexing" | "ready" | "failed"
    "title": "Auto-extracted PDF title",
    "summary": "TL;DR in 2-3 sentences...",
    "page_count": 42,
    "chunk_count": 156,
    "suggested_questions": [
      "What is the main argument?",
      "Who are the authors?",
      ...
    ]
  }
}
```

### 5.3 Send message (SSE streaming)
```
POST /api/chat/sessions/<id>/messages
Content-Type: application/json
Accept: text/event-stream

Request:
{
  "content": "What does page 4 say about pricing?"
}

Response (SSE stream):
event: meta
data: {"message_id": "uuid", "retrieved_chunks": [{"page":4, "chunk_id":"..."}]}

event: token
data: {"text": "The"}

event: token
data: {"text": " pricing"}

event: token
data: {"text": " on page 4"}

...

event: citations
data: {"citations": [{"page": 4, "chunk_id": "..."}]}

event: done
data: {"message_id": "uuid", "tokens_in": 1234, "tokens_out": 312}
```

Frontend: `EventSource` yoki `fetch` + ReadableStream bilan o'qiydi.

### 5.4 List messages (chat tarixi)
```
GET /api/chat/sessions/<id>/messages?limit=50

Response 200:
{
  "StatusCode": 200,
  "Data": [
    {
      "id": "uuid",
      "role": "user",
      "content": "What is the main argument?",
      "created_at": "..."
    },
    {
      "id": "uuid",
      "role": "assistant",
      "content": "The main argument is...",
      "citations": [{"page":2}, {"page":5}],
      "created_at": "..."
    }
  ]
}
```

### 5.5 List user sessions (sidebar)
```
GET /api/chat/sessions?limit=20

Response 200:
{
  "StatusCode": 200,
  "Data": [
    {
      "id": "session-uuid",
      "file_id": "uuid",
      "title": "Q3 Financial Report",
      "created_at": "...",
      "message_count": 12
    },
    ...
  ]
}
```

### 5.6 Delete session
```
DELETE /api/chat/sessions/<id>

Response 204
```

CASCADE: pdf_chunks, chat_messages ham o'chiriladi.

---

## 6. LLM Provider Abstraktsiyasi

### 6.1 Interface (Go)
```go
// service/llm/provider.go
package llm

type Message struct {
    Role    string // "system" | "user" | "assistant"
    Content string
}

type ChatOpts struct {
    Temperature float32
    MaxTokens   int
    Stream      bool
}

type ChatChunk struct {
    Text       string
    FinishReason string  // "" | "stop" | "length"
    TokensIn   int       // populated on final chunk
    TokensOut  int
    Err        error
}

type Provider interface {
    Name() string
    Chat(ctx context.Context, msgs []Message, opts ChatOpts) (<-chan ChatChunk, error)
    Embed(ctx context.Context, texts []string) ([][]float32, error)
}
```

### 6.2 Provider implementations
```
service/llm/groq.go       — Groq (Llama 3.3 70B) — DEFAULT
service/llm/claude.go     — Anthropic Claude — 1 oydan keyin
service/llm/gemini.go     — Google Gemini — ham LLM ham embedding
service/llm/openai.go     — OpenAI — backup
service/llm/ollama.go     — Local Ollama — emergency fallback
```

Har biri `Provider` interface'ini implement qiladi.

### 6.3 Hybrid strategy
```go
// service/llm/hybrid.go
// LLM uchun Groq, embedding uchun Gemini ishlatamiz
type hybridProvider struct {
    chatProvider  Provider  // groq
    embedProvider Provider  // gemini
}
```

Sabab: Groq embedding bermaydi, faqat LLM. Gemini ikkalasini ham beradi, lekin LLM uchun Groq tezroq.

### 6.4 Config (.env)
```bash
# Hozir
LLM_CHAT_PROVIDER=groq
LLM_EMBED_PROVIDER=gemini
GROQ_API_KEY=gsk_...
GEMINI_API_KEY=AIza...

# 1 oydan keyin (faqat 2 qator o'zgaradi):
LLM_CHAT_PROVIDER=claude
ANTHROPIC_API_KEY=sk-ant-...
```

---

## 7. Chunking Strategy

### Yondashuv: Semantic chunking with overlap
```
Page boundary'da bo'linadi (har page alohida)
↓
Har page 500-1000 belgili chunk'larga
  ├─ Overlap: 100 belgi (kontekst yo'qolmasligi uchun)
  └─ Bo'linish: gap'larda (`.`, `\n\n`)
↓
Har chunk page_number bilan saqlanadi (citation uchun)
```

**Misol**: 50 pageli PDF
- ~30 chunk per page × 50 = ~1500 chunks
- Har chunk ~700 belgi ≈ 175 tokens
- Jami ~262K tokens
- Embedding: 1500 chunks × 1 call = 1500 Gemini call (free quota: 1500/kun)
- ❗ Bir kunda **1 ta katta PDF** ko'p! Batch embedding (Gemini bir vaqtda 100 ta chunk qabul qiladi) → 15 call per PDF

### Batch embedding optimization
Gemini `embedContent` endpoint **batch mode**'da 100 ta text bir vaqtda qabul qiladi:
- 1500 chunks ÷ 100 = 15 batch call per PDF
- Free quota (1500/kun) ÷ 15 = **100 PDF/kun tekin index qilamiz**

---

## 8. Retrieval & Prompting

### 8.1 Retrieval
User savoli:
```
1. Savol → Gemini embedding → 768-dim vector
2. SQL:
   SELECT id, page_number, content,
          1 - (embedding <=> $1::vector) AS similarity
   FROM pdf_chunks
   WHERE session_id = $2
   ORDER BY embedding <=> $1::vector
   LIMIT 7;
3. Top-7 chunks olamiz (page_number bilan birga)
```

### 8.2 System prompt
```
You are a helpful assistant that answers questions based ONLY on the
provided PDF document chunks. Always cite the page number using
[page N] format when referencing information from the document.

If the answer is not in the provided chunks, say so clearly.
Do not make up information.

Document chunks:
---
[page 1] (chunk 1)
{chunk content}

[page 4] (chunk 23)
{chunk content}

...
---

Answer in the same language as the user's question.
```

### 8.3 User prompt
Faqat user savoli (chat history bilan, agar bor bo'lsa).

### 8.4 Token budget
- System prompt: ~300 tokens
- 7 chunks × 175 tokens = 1225 tokens
- Chat history (last 6 messages): ~1000 tokens
- User question: ~50 tokens
- **Total input**: ~2575 tokens
- Output: 500-1000 tokens (Groq limit'dan past)
- **Per query**: ~3.5K tokens

Groq free tier: 14,400 TPM (token/min) → ~4 query/min maksimal. Real use'da ~1-2 query/min user'dan, OK.

---

## 9. Frontend Changes

### 9.1 Existing mockup'ni o'zgartirish
`/root/converto/apps/web/app/(tools)/chat/page.tsx`:
- Hozir: static mockup
- Yangi: Client component bilan to'liq state, API integratsiya

### 9.2 Yangi komponentlar
```
apps/web/app/(tools)/chat/
├── page.tsx                  — Server component, ToolPageShell wrapper
├── chat-client.tsx            — Main client component (state, layout)
├── components/
│   ├── pdf-viewer.tsx         — pdf.js viewer (left panel)
│   ├── chat-panel.tsx         — Chat messages + input (right panel)
│   ├── message-bubble.tsx     — User/assistant message
│   ├── citation-link.tsx      — [page N] clickable badge
│   ├── session-sidebar.tsx    — Chat history (left rail)
│   ├── upload-zone.tsx        — Initial upload (replaces mockup placeholder)
│   └── suggested-questions.tsx — Avto-generated questions
```

### 9.3 Hook'lar
```
apps/web/lib/chat/
├── use-chat-session.ts        — Session CRUD
├── use-chat-stream.ts         — SSE streaming
├── use-chat-history.ts        — Message list
└── api.ts                     — Chat-specific API wrappers
```

### 9.4 Layout — side-by-side
```
┌──────────────────────────────────────────────────────────┐
│  ToolPageShell header                                    │
├──────────┬──────────────────────────┬────────────────────┤
│ Sidebar  │   PDF Viewer (pdf.js)    │   Chat Panel       │
│ (sessions)│  (highlights on cite)    │                    │
│          │                          │   [messages...]    │
│ session1 │   page 4 ←  ←  ←        │   user: hi         │
│ session2 │   ─────────              │   ai: ...[page 4]  │
│ session3 │   {pdf content}          │                    │
│          │   ─────────              │   [input...]       │
│ +new     │                          │                    │
└──────────┴──────────────────────────┴────────────────────┘
```

### 9.5 PDF.js integration
- `pdfjs-dist` npm package
- Worker thread'da render
- `react-pdf` wrapper'ni ishlatamiz
- Citation click → `scrollIntoView` + temporary highlight overlay

---

## 10. Cost & Quota Analysis

### Free tier limitlar
| Service | Free quota | Bir oycha |
|---|---|---|
| **Groq Llama 3.3 70B** | 14,400 TPM, 30 RPM | 1.2M tokens, ~600K savol |
| **Gemini Embedding** | 1,500 req/kun, batch 100x | ~100 PDF index/kun, ~3K PDF/oy |
| **Postgres** | Mahalliy | Cheksiz |

### Realistik scenariy
- 100 active user, har user kuniga 5 savol → 500 savol/kun
- Har savol ~3.5K tokens → 1.75M tokens/kun
- ❌ Groq free quota 14,400 TPM = 20M/kun, lekin RPM 30 = 43K savol/kun
- 1.75M token uchun: kuniga **8 daqiqa Groq API'ga ulanish**
- ✅ Quota yetadi, lekin **rate limit (30 RPM)** chegara — minutiga 30 savoldan ko'p kelmasligi kerak
- → Frontend'da: bitta user bir vaqtda bir savol, queue qilamiz

### 1 oydan keyin (paid'ga o'tish)
Agar 1000+ user bo'lsa, Claude Sonnet'ga o'tamiz:
- Per session: ~$0.05 (caching bilan)
- 100,000 savol/oy: $5,000
- Bunday traffic kelguniga qadar revenue model ham kerak (Pro plan?)

---

## 11. Implementation Phases (3 hafta)

### **Hafta 1: Backend foundation**
- **D1**: Migration (pgvector + 5 ta table)
  - File: `migrations/postgres/0002_pdf_chat_ai.up.sql`
  - Test: `psql convertpdfgo -c "\d chat_sessions"` ishlaydi
- **D2**: LLM Provider interface
  - File: `service/llm/provider.go`, `service/llm/groq.go`, `service/llm/gemini.go`
  - Test: `go test ./service/llm/...`
- **D3**: PDF text extraction (page-aware)
  - File: `service/pdfchat/extract.go`
  - Test: `pdfcpu extract -mode text` natijasini page-by-page parse qiladi
- **D4**: Chunker + batch embedding pipeline
  - File: `service/pdfchat/index.go`
  - Asynq task: `chat:index_pdf`
- **D5**: Chat session CRUD + indexing trigger
  - Handler: `api/handler/chat_session.go`
  - Endpoints: POST/GET/DELETE /api/chat/sessions
- **D6-7**: Chat messages + SSE streaming
  - Handler: `api/handler/chat_message.go`
  - Retrieval + Groq streaming → SSE response

### **Hafta 2: Frontend integration**
- **D8**: Chat client component skeleton (rewrite mockup'ni)
- **D9**: Upload + session create + status polling
- **D10**: Chat input + SSE streaming receive + render
- **D11**: Citations [page N] rendering + click handlers
- **D12**: pdf.js viewer integration (side-by-side layout)
- **D13**: Suggested questions + summary on load
- **D14**: Session sidebar (history list, switch sessions)

### **Hafta 3: Polish & advanced features**
- **D15**: Voice input (Web Speech API)
- **D16**: Multi-PDF mode (2-3 PDF birga chat)
- **D17**: Export chat (PDF/Markdown)
- **D18**: Public share links (`/chat/share/<token>`)
- **D19**: Rate limiting + cost tracking + error handling
- **D20**: Mobile responsive + accessibility
- **D21**: Production polish + monitoring

---

## 12. Acceptance Criteria (MVP)

PDF Chat AI MVP "tugadi" deyishimiz uchun:

- [ ] User PDF (≤50 page) yuklaydi va 15 soniyada index tayyor
- [ ] User savol berib, 5 soniyada birinchi javob token kelishi boshlaydi (TTFB)
- [ ] Har javobda kamida 1 ta `[page N]` citation bor
- [ ] Citation bosilsa, PDF viewer shu page'ga jump qiladi
- [ ] Avto-summary fayl yuklangach 10 soniyada ko'rinadi
- [ ] 4-6 ta suggested question avto-chiqadi
- [ ] User logout/login qilsa, chat tarixi saqlangan
- [ ] Mobile (375px) layout ishlaydi
- [ ] Boshqa tilda (uz/ru) savol berilsa, javob shu tilda
- [ ] Rate limit: guest 10 savol/kun, signed-in 100 savol/kun
- [ ] Eski PDF (>30 kun) chunk'lari avtomatik o'chiriladi (storage cleanup)
- [ ] 95-percentile savol latency <8s

---

## 13. Risks & Mitigations

| Xavf | Ehtimol | Mitigatsiya |
|---|---|---|
| Groq rate limit (30 RPM) ko'p user'da yetmaydi | O'rta | Queue qilamiz + cache common questions |
| Gemini embedding 1500/kun yetmaydi | Past (100 PDF/kun katta) | Local sentence-transformers fallback |
| PDF text extraction skanerlangan PDF'larda ishlamaydi | Yuqori | OCR fallback (allaqachon ocrmypdf bor) |
| Hallucination — LLM PDF'da yo'q narsa "topadi" | O'rta | System prompt'da "agar yo'q bo'lsa ayt" majburiy + low temperature (0.2) |
| User PDF'i sensitive ma'lumot — tashqi API'ga ketadi | Yuqori | T&C'da aniq ayt, BYOK option keyin qo'shamiz |
| Vector index katta bo'lib disk to'ladi | O'rta | Cleanup job: 30 kundan oshgan chunk'lar o'chiriladi |
| 1 oydan keyin Claude'ga o'tish murakkab bo'ladi | Past | Provider abstraktsiyasi day 1'dan bor |

---

## 14. Success Metrics

Launch'dan keyin 2 hafta ichida:

| Metrik | Target |
|---|---|
| Daily active chat sessions | >50 |
| Avg messages per session | >5 |
| Citation click rate | >40% (foydalanuvchilar PDF'ga qarashlari kerak) |
| Avg user satisfaction (👍/👎) | >70% 👍 |
| Conversion to signup (guest → registered) | >15% |
| Mobile usage | >30% |

Tracking: Prometheus metrics + Grafana dashboard (allaqachon stack'da bor).

---

## 15. Out of Scope (v1.0)

Bularni keyingi versiyalarga qoldiramiz:

- [ ] PDF'larda image/diagram'larni tushunish (vision models)
- [ ] Table extraction va structured query (e.g., "give me row 4 of the table")
- [ ] Multi-language UI (chat o'zi multi-lang ishlaydi, lekin interface tilini o'zgartirish keyinroq)
- [ ] Collaborative chat (multiple users, real-time)
- [ ] Pro tier (paid subscription) — Stripe integration (allaqachon backend'da chat emas, payment uchun base bor)
- [ ] API access (third-party developers o'z app'larida ishlatishi)
- [ ] Annotations on PDF (user highlight'lar, notes)

---

## 16. Open Questions

1. **BYOK qachon qo'shamiz?** v1.0'da yo'q. Keyingi sprintda foydalanuvchi sozlamalarda Groq/Claude/OpenAI key'ini kiritsa, o'sha provider ishlatiladi.
2. **Public share xavfsizmi?** Read-only link, password-protected option. v1.0'da faqat unlisted (link bilan ko'rinadi).
3. **PDF lokal saqlanadimi?** Ha, `/app/uploads/` dyn — allaqachon shunday. 5 daqiqadan keyin auto-delete (boshqa tool'lar kabi)? **YO'Q** — chat'da PDF chunks DB'da, original fayl 7 kun saqlansin (user qaytib kelishi mumkin).
4. **Cost cap qancha?** Free tier'dan o'tib ketmasligi uchun: kuniga 80% quota'ga yetganda yangi session yaratishni bloklash.

---

## 17. Approval & Next Steps

PRD tasdiqlangach:
1. ✅ Bu PRD'ni o'qib chiqing va savollarni `## 16. Open Questions` bo'limida belgilang
2. ✅ Groq API key oling: https://console.groq.com → API Keys → Create
3. ✅ Gemini API key oling: https://aistudio.google.com → Get API key
4. ✅ Ikkalasini server'da `.env`'ga qo'shing
5. ⏭️ **D1 — Migration**'dan boshlaymiz

---

**Document version**: 1.0 · 2026-05-27
**Linked**: [BAHROM-WIRING.md](BAHROM-WIRING.md), [PDF_FEATURES.md](PDF_FEATURES.md), [PDF_ROADMAP.md](PDF_ROADMAP.md)
