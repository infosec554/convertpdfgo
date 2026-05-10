# 📄 convertpdfgo — PDF Features Roadmap

> Hozirgi 25 ta PDF operatsiyasi ustiga **15 ta yangi feature** rejasi.
> Toollar: `pdfcpu` (Go library + CLI), `Gotenberg` (HTTP), poppler (`pdftoppm`),
> kelajakda — Tesseract (OCR), Ghostscript (PDF/A), ImageMagick.

---

## 📊 Hozirgi holat

```
✅ Page ops:         merge, split, removepage, extract, compress
✅ Transforms:       rotate, crop, add-page-numbers, header-footer, qr-code
✅ Security:         protect, unlock
✅ Inspect:          inspect, detect-blank
✅ Image:            jpg-to-pdf, pdf-to-jpg
✅ Office:           word↔pdf, excel→pdf, ppt→pdf, html→pdf
✅ Share:            share/:token
                     ──────────────
                     Jami: 25 endpoint
```

---

## 🎯 Bosqichma-bosqich reja

### 🔴 BOSQICH 1 — Tezkor g'alabalar (3-4 soat, 5 ta endpoint)

> Yangi tool kerak emas. Mavjud `pdfcpu` + `Gotenberg` bilan qilamiz.

| # | Endpoint | Service | Underlying | Effort |
|---|----------|---------|------------|--------|
| 1 | `POST /api/pdf/watermark` | `WatermarkService` | pdfcpu `AddWatermarksFile` (text yoki image) | 60 min |
| 2 | `POST /api/pdf/extract-text` | `ExtractTextService` | pdfcpu `ExtractTextFile` (TXT export) | 30 min |
| 3 | `POST /api/pdf/nup` | `NUpService` | pdfcpu `NUpFile` (2/4/6/9-up bir varaqda) | 30 min |
| 4 | `POST /api/pdf/url-to-pdf` | `URLToPDFService` | Gotenberg `/forms/chromium/convert/url` | 30 min |
| 5 | `POST /api/pdf/metadata` | `MetadataService` | pdfcpu `SetTitle`, `SetAuthor`, etc. | 60 min |

**Migration:** `0007_features_phase1.up.sql` — har bir uchun `*_jobs` jadval.

**Pattern:** Hozirgi async pattern (Create → Enqueue → Process). Frontend yo'q deyilganda, har biri:
- POST → 202 + `job_id`
- GET `/.../{id}` → status

**Foyda:** 25 → **30 endpoint**. Asosiy raqib (smallpdf/ilovepdf) bilan teng.

---

### 🟡 BOSQICH 2 — Format kengaytmalari (3-4 soat, 5 ta endpoint)

| # | Endpoint | Tool | Effort |
|---|----------|------|--------|
| 6 | `POST /api/pdf/redact` | pdfcpu `Redact` (text/area qora) | 60 min |
| 7 | `POST /api/pdf/organize` | pdfcpu `Collect` / page reordering | 60 min |
| 8 | `POST /api/pdf/md-to-pdf` | Gotenberg + markdown→html (server'da render) | 45 min |
| 9 | `POST /api/pdf/bookmarks` | pdfcpu `AddBookmarks` (TOC) | 45 min |
| 10 | `POST /api/pdf/booklet` | pdfcpu `Booklet` (4-up imposition) | 30 min |

---

### 🟢 BOSQICH 3 — OCR + Forms + Sign (alohida ish, ~6 soat)

> Yangi tool'lar kerak: Tesseract, openssl certs, optional Ghostscript.

| # | Endpoint | Tool | Effort |
|---|----------|------|--------|
| 11 | `POST /api/pdf/ocr` | **Tesseract** container + ocrmypdf wrapper | 2 soat |
| 12 | `POST /api/pdf/form/fill` | pdfcpu `FillForm` | 1 soat |
| 13 | `POST /api/pdf/form/extract` | pdfcpu `ExportForm` | 30 min |
| 14 | `POST /api/pdf/sign` | pdfcpu (sertifikat majburiy) | 2 soat |
| 15 | `POST /api/pdf/to-pdfa` | Ghostscript (`gs -dPDFA`) | 30 min |

---

## 🛠️ Tool stack

| Tool | Endpoint'lar | Container | Hajmi |
|------|--------------|-----------|-------|
| pdfcpu (Go lib) | watermark, extract-text, nup, metadata, redact, organize, bookmarks, booklet, form, sign | API + worker | 0 (bundled) |
| pdfcpu CLI | rotate, crop, protect, unlock, page-numbers, header-footer | runtime | 30 MB |
| Gotenberg | word↔pdf, excel/ppt/html→pdf, **url-to-pdf**, **md-to-pdf** | docker service | 1.2 GB |
| poppler (`pdftoppm`) | pdf-to-jpg | runtime | 5 MB |
| **Tesseract + ocrmypdf** | OCR (#11) | **yangi container** | **~600 MB** |
| **Ghostscript** | PDF/A (#15) | runtime apt | 50 MB |

---

## 📁 Yaratiladigan fayllar (Bosqich 1)

```
api/handler/
  ├── watermark.go             # POST/GET /api/pdf/watermark
  ├── extract_text.go          # POST/GET /api/pdf/extract-text
  ├── nup.go                   # POST/GET /api/pdf/nup
  ├── url_to_pdf.go            # POST/GET /api/pdf/url-to-pdf
  └── metadata.go              # POST/GET /api/pdf/metadata

service/
  ├── watermark.go             # Create + Process pattern
  ├── extract_text.go
  ├── nup.go
  ├── url_to_pdf.go
  └── metadata.go

api/models/
  ├── watermark.go             # Request + Job structs
  ├── extract_text.go
  ├── nup.go
  ├── url_to_pdf.go
  └── metadata.go

storage/postgres/
  ├── watermark.go             # CRUD on watermark_jobs
  ├── extract_text.go
  ├── nup.go
  ├── url_to_pdf.go
  └── metadata.go

migrations/postgres/
  └── 0007_features_phase1.{up,down}.sql

pkg/queue/tasks.go             # 5 ta yangi TaskType const + payload
pkg/queue/handlers.go          # 5 ta yangi worker handler
pkg/gotenberg/clint.go         # URLToPDF metodi qo'shiladi
api/router.go                  # /api/pdf/* ga 10 ta yangi route (POST+GET)
service/service.go             # 5 ta yangi service constructor
storage/storage.go             # 5 ta yangi storage interface
```

---

## 🏗️ Bosqich 1 — Implementation order

1. **Migration** (5 ta jadval, 5 daqiqa)
2. **Models** (5 ta fayl, 10 daqiqa)
3. **Storage** (5 ta `*Repo`, 30 daqiqa)
4. **Services** (5 ta service, 60 daqiqa)
5. **Queue tasks + handlers** (10 daqiqa)
6. **Gotenberg `URLToPDF` metodi** (15 daqiqa)
7. **HTTP handlers** (5 ta fayl, 30 daqiqa)
8. **Router'ga 10 ta route** (5 daqiqa)
9. **Vet + commit + deploy** (15 daqiqa)
10. **Smoke test (curl)** (10 daqiqa)

**Jami: ~3 soat.**

---

## 🎯 Bosqich 1 — Endpoint tafsilotlari

### 1. Watermark (`POST /api/pdf/watermark`)

```json
{
  "input_file_id": "uuid",
  "type":          "text" | "image",
  "text":          "CONFIDENTIAL",   // type=text bo'lsa
  "image_file_id": "uuid",           // type=image bo'lsa
  "position":      "c|tl|tr|bl|br",
  "rotation":      0,                // -90..90
  "opacity":       0.4,              // 0..1
  "pages":         "all" | "1-3,5"
}
```

### 2. Extract Text (`POST /api/pdf/extract-text`)

```json
{
  "input_file_id": "uuid",
  "format":        "txt" | "json"   // json: per-page
}
```
Output: `txt` fayl `files` jadvalida saqlanadi, `output_file_id` qaytadi.

### 3. N-up (`POST /api/pdf/nup`)

```json
{
  "input_file_id": "uuid",
  "n":             2 | 4 | 6 | 9,    // sahifa soni / varaq
  "orientation":   "portrait" | "landscape",
  "border":        true              // ramka
}
```

### 4. URL → PDF (`POST /api/pdf/url-to-pdf`)

```json
{
  "url":          "https://example.com/article",
  "wait_for":     "networkidle0",   // optional
  "page_size":    "A4",
  "landscape":    false
}
```
Gotenberg `/forms/chromium/convert/url` orqali.

### 5. Metadata (`POST /api/pdf/metadata`)

```json
{
  "input_file_id": "uuid",
  "title":         "My Doc",
  "author":        "Zarif",
  "subject":       "...",
  "keywords":      ["pdf","convert"],
  "creator":       "convertpdfgo"
}
```

---

## 🎯 Sequence diagram — Watermark misolida

```
Frontend         API                    Worker             pdfcpu
  │              │                        │                  │
  ├─POST /watermark (req)                 │                  │
  │              ├─Create job (DB)        │                  │
  │              ├─Enqueue task ──────────┤                  │
  │◄─202 {id}────┤                        │                  │
  │              │                        ├─Process(jobID)   │
  │              │                        ├─GetByID job      │
  │              │                        ├─GetByID file     │
  │              │                        ├─AddWatermarks ──►│
  │              │                        │◄─output.pdf──────┤
  │              │                        ├─Save File row    │
  │              │                        ├─Update job=done  │
  ├─GET /watermark/:id                    │                  │
  │              ├─DB lookup              │                  │
  │◄─200 {output_file_id}                 │                  │
```

---

## 🚀 Deploy reja

Har bir bosqichdan keyin:
1. `git commit -m "feat(pdf): add <feature>"`
2. `git push`
3. Server'ga tarball + `docker compose --profile full up -d --build api worker`
4. Smoke test (curl)
5. Swagger regen (`make generate`)

---

## 📊 Kutilayotgan natija

| Holat | Endpoint soni | Bosqich | Vaqt |
|-------|---------------|---------|------|
| Hozir | 25 | — | — |
| Bosqich 1 | **30** (+5) | Watermark, Text, N-up, URL→PDF, Metadata | 3 soat |
| Bosqich 2 | **35** (+5) | Redact, Organize, MD→PDF, Bookmarks, Booklet | 3 soat |
| Bosqich 3 | **40** (+5) | OCR, Form fill/extract, Sign, PDF/A | 6 soat |

---

*Yaratildi: 2026-05-10*
