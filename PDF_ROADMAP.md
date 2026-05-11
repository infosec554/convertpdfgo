# 📄 convertpdfgo — To'liq Roadmap

> Hozirgi 30 endpoint ustiga qolgan barcha feature'larni Go'da qo'shish rejasi.
> Raqobatchilar (smallpdf/ilovepdf/pdfcandy) bilan tenglashtirish uchun.

---

## 📊 Hozirgi holat (2026-05-10)

```
✅ 30 endpoint, smoke 26/26 PASS prod
✅ 5-min privacy cleaner ON DELETE CASCADE bilan
✅ Async pipeline: Asynq + Redis, 3 priority queue
✅ Tools: pdfcpu (Go lib + CLI v0.11.0), Gotenberg, poppler-utils
✅ DB: postgres-15, 10 ta migration
```

---

## 🎯 Asosiy savol: Hammasini Go'da qila olamizmi?

**Qisqa javob:** Ha — **~92%**.

| Kategoriya | Go feasibility | Tool/Library |
|-----------|----------------|--------------|
| Page operations | ✅ 100% | pdfcpu Go API |
| Image ↔ PDF | ✅ 100% | Go `image` package + pdfcpu/gofpdf |
| Office ↔ PDF | ✅ 95% | Gotenberg (LibreOffice) |
| OCR | ✅ 100% | Tesseract container + `gosseract` Go bindings |
| Form / Sign | ✅ 100% | pdfcpu `FillForm` + `Sign` API |
| Edit Text / Annotate | ⚠️ 70% | pdfcpu annotations (overlay), real text edit qiyin |
| Translate PDF | ✅ 90% | extract-text + DeepL/Google API + reflow |
| Compare PDF | ✅ 80% | Render → image diff, yoki text diff |
| AutoCAD (DWG/Blueprint) | ❌ 30% | Maxsus kutubxona kerak (pure Go yo'q) |
| AI (Illustrator) | ❌ 20% | Proprietary format, Adobe SDK kerak |
| DJVU | ⚠️ 60% | DjVuLibre orqali, Go binding bor |

**Go-da hal qilolmaymiz**: faqat **3 ta** niche feature (DWG, AutoCAD, AI). Qolganlari hammasi mumkin.

---

## 📦 Implementation tartib (numerated)

> Har bir item: yangi tool kerakmi, qancha vaqt, qaysi technique.

### 🔴 BOSQICH 2 — Quick wins (jami ~6 soat, 12 endpoint)

> Yangi tool kerak emas, mavjud stack bilan ishlaymiz. DB schemalarning bir qismi mavjud.

| # | Feature | Endpoint | Underlying | Effort |
|---|---------|----------|------------|--------|
| 31 | **Reorder Pages** | `POST /api/pdf/organize` | pdfcpu `CollectFile` (jadval `organize_jobs` allaqachon mavjud) | 30 min |
| 32 | **Add Pages to PDF** | `POST /api/pdf/add-pages` | pdfcpu `MergeFile` (insert pozitsiya bilan) | 45 min |
| 33 | **Cut Out PDF** | `POST /api/pdf/cut-out` | extract'ning teskarisi (range'larni o'chirish) | 30 min |
| 34 | **PDF to PNG** | `POST /api/pdf/pdf-to-png` | poppler `pdftoppm -png` (pdf-to-jpg pattern) | 30 min |
| 35 | **PNG to PDF** | `POST /api/pdf/png-to-pdf` | Go `image/png` + pdfcpu (jpg-to-pdf pattern) | 30 min |
| 36 | **Image to PDF (universal)** | `POST /api/pdf/image-to-pdf` | PNG+JPG+BMP+WebP+TIFF qabul qiladi | 45 min |
| 37 | **PDF to Image (universal)** | `POST /api/pdf/pdf-to-image` | format param (jpg/png) | 30 min |
| 38 | **Grayscale PDF** | `POST /api/pdf/grayscale` | Ghostscript `gs -sColorConversionStrategy=Gray` | 45 min |
| 39 | **PDF to A4** | `POST /api/pdf/to-a4` | pdfcpu `ResizeFile` page size'ga | 45 min |
| 40 | **Search PDF** | `POST /api/pdf/search` | extract-text + regex match (page+context) | 30 min |
| 41 | **Text to PDF** | `POST /api/pdf/text-to-pdf` | gofpdf yoki Gotenberg (txt → html → pdf) | 45 min |
| 42 | **PDF Receipt** | `POST /api/pdf/receipt` | template + gofpdf (json → invoice PDF) | 60 min |

**Jami: 30 → 42 endpoint, ~6 soat.**

---

### 🟡 BOSQICH 3 — OCR + Forms + Sign (jami ~6 soat, 5 endpoint)

> Yangi tool: **Tesseract** (yangi container ~600 MB), **openssl certs**, **Ghostscript** (apt).

| # | Feature | Endpoint | Underlying | Effort |
|---|---------|----------|------------|--------|
| 43 | **OCR PDF** | `POST /api/pdf/ocr` | `ocrmypdf` wrapper (Tesseract orqali) yoki `gosseract` Go binding | 2 soat |
| 44 | **Scan to PDF** | `POST /api/pdf/scan-to-pdf` | image → OCR → PDF (#43 variant) | 30 min |
| 45 | **Fill PDF Form** | `POST /api/pdf/form/fill` | pdfcpu `FillFormFile` | 60 min |
| 46 | **Extract Form** | `POST /api/pdf/form/extract` | pdfcpu `ExportFormFile` | 30 min |
| 47 | **Sign PDF** | `POST /api/pdf/sign` | pdfcpu `SignFile` (X.509 cert majburiy) | 2 soat |
| 48 | **PDF/A** | `POST /api/pdf/to-pdfa` | Ghostscript `gs -dPDFA` | 30 min |

**Jami: 42 → 48 endpoint, ~6 soat.**

---

### 🟢 BOSQICH 4 — Image format kengaytmalari (jami ~3 soat, 7 endpoint)

> Hech narsa qo'shilmaydi, faqat #36-37'ni ekstraktsiya qilamiz.

| # | Feature | Endpoint | Underlying | Effort |
|---|---------|----------|------------|--------|
| 49 | **WebP to PDF** | `POST /api/pdf/webp-to-pdf` | golang.org/x/image/webp | 20 min |
| 50 | **BMP to PDF** | `POST /api/pdf/bmp-to-pdf` | golang.org/x/image/bmp | 20 min |
| 51 | **TIFF to PDF** | `POST /api/pdf/tiff-to-pdf` | golang.org/x/image/tiff | 20 min |
| 52 | **HEIC to PDF** | `POST /api/pdf/heic-to-pdf` | `goheif` (HEIF/HEIC support) | 30 min |
| 53 | **Photo to PDF** | `POST /api/pdf/photo-to-pdf` | universal image (#36 alias, multi-photo) | 20 min |
| 54 | **Drawing to PDF** | `POST /api/pdf/drawing-to-pdf` | image-to-pdf alias | 15 min |
| 55 | **PNG/JPG/etc to PDF** alias | URL alias | 1 line in router | 10 min |

**Jami: 48 → 55 endpoint, ~3 soat.**

---

### 🟣 BOSQICH 5 — Markup va Edit (jami ~5 soat, 9 endpoint)

> pdfcpu Annotations API'sini ishlatamiz. Real text editing (parsing content streams) hozircha skip — overlay yondashuvi.

| # | Feature | Endpoint | Underlying | Effort |
|---|---------|----------|------------|--------|
| 56 | **Add Text to PDF** | `POST /api/pdf/add-text` | pdfcpu `AddWatermarksFile` (watermark generalized) | 30 min |
| 57 | **Add Stamp to PDF** | `POST /api/pdf/stamp` | pdfcpu `Stamp` (image overlay) | 30 min |
| 58 | **Insert Image into PDF** | `POST /api/pdf/insert-image` | pdfcpu image watermark + position | 30 min |
| 59 | **Annotate PDF** | `POST /api/pdf/annotate` | pdfcpu `AddAnnotationsFile` (highlight/note/draw) | 60 min |
| 60 | **Highlight Text in PDF** | `POST /api/pdf/highlight` | pdfcpu annotation type=Highlight | 45 min |
| 61 | **Delete Text in PDF** (white-out) | `POST /api/pdf/delete-text` | pdfcpu Redact (text yoki area qora/oq blok) | 60 min |
| 62 | **Redact PDF** | `POST /api/pdf/redact` | pdfcpu redact (sensitive info hide) | 60 min |
| 63 | **PDF Links (manage)** | `POST /api/pdf/links` | pdfcpu Annotations URI type | 45 min |
| 64 | **PDF Bookmarks** | `POST /api/pdf/bookmarks` | pdfcpu `AddBookmarks` (TOC) | 45 min |

**Jami: 55 → 64 endpoint, ~5 soat.**

---

### 🟤 BOSQICH 6 — Office + Specialized (jami ~4 soat, 6 endpoint)

> Gotenberg LibreOffice ishlatamiz, ba'zilari maxsus kutubxonalar talab qiladi.

| # | Feature | Endpoint | Underlying | Effort |
|---|---------|----------|------------|--------|
| 65 | **PDF to Excel** | `POST /api/pdf/pdf-to-excel` | LibreOffice draw → calc, yoki `excelize` + table extract | 90 min |
| 66 | **PDF to PPT** | `POST /api/pdf/pdf-to-ppt` | LibreOffice headless `--convert-to pptx` | 45 min |
| 67 | **DOC to PDF** | `POST /api/pdf/doc-to-pdf` | word-to-pdf alias (eski .doc qabul qilish) | 15 min |
| 68 | **XML to PDF** | `POST /api/pdf/xml-to-pdf` | XSL-FO + Apache FOP, yoki XML → HTML → Gotenberg | 60 min |
| 69 | **TIFF to PDF** | (#51 ga ilova) | image-to-pdf | — |
| 70 | **File to PDF (universal)** | `POST /api/pdf/file-to-pdf` | extension'ga qarab tegishli endpoint'ga route | 45 min |

**Jami: 64 → 70 endpoint, ~4 soat.**

---

### 🟦 BOSQICH 7 — Utility + Analysis (jami ~5 soat, 6 endpoint)

| # | Feature | Endpoint | Underlying | Effort |
|---|---------|----------|------------|--------|
| 71 | **Compare PDF** | `POST /api/pdf/compare` | text diff + page render diff | 90 min |
| 72 | **Translate PDF** | `POST /api/pdf/translate` | extract-text + DeepL/Google API + reflow | 2 soat |
| 73 | **Translate Text in PDF** | (#72 alias) | language-aware overlay | 30 min |
| 74 | **Create PDF** | `POST /api/pdf/create` | gofpdf — text/images/forms'dan tuziladi | 90 min |
| 75 | **Print PDF (prepare)** | `POST /api/pdf/print-prep` | margin/size optimize, pdfcpu | 30 min |

**Jami: 70 → 75 endpoint, ~5 soat.**

---

### ⚫ BOSQICH 8 — Specialized formats (qo'shimcha, ixtiyoriy)

> Niche use-cases. AutoCAD/AI lar **Go'da to'liq qo'llab-quvvatlanmaydi**.

| # | Feature | Tool | Go feasibility | Effort |
|---|---------|------|----------------|--------|
| 76 | **DJVU to PDF** | DjVuLibre + `djvu` Go binding | ⚠️ 60% (CLI wrapper) | 60 min |
| 77 | **DWG to PDF** | LibreCAD CLI yoki online API | ❌ Go yo'q (CLI shell-out) | 2 soat |
| 78 | **PDF to DWG** | Specialized commercial tool | ❌ Yo'q | — |
| 79 | **PDF to AutoCAD** | (#78 ga o'xshash) | ❌ Yo'q | — |
| 80 | **AI to PDF** | Adobe Illustrator format | ❌ Adobe SDK kerak | — |
| 81 | **Blueprint to PDF** | Image to PDF alias (yuqori dpi) | ✅ | 20 min |
| 82 | **PDF to XML** | extract-text + struct schema | ✅ | 60 min |

**Jami: 75 → 82 endpoint (ammo 3 ta — Go'da yopilmaydi)**

---

### 🟫 BOSQICH 9 — Frontend-only (backend kerak emas)

> Backend ishi yo'q yoki minimal — frontend implementation.

| # | Feature | Joy | Backend yordami |
|---|---------|-----|-----------------|
| 83 | **PDF Reader** | Browser PDF.js | Faqat `/file/:id/download` |
| 84 | **Print PDF** | Browser native print | Yo'q |
| 85 | **PDF Fonts (preview)** | Frontend list | Inspect (#13) ishlatish |

---

## 📊 Yakuniy hisob

| Bosqich | Yangi endpoint | Vaqt | Total endpoint |
|---------|----------------|------|----------------|
| Hozir | — | — | **30** |
| 🔴 Bosqich 2 — Quick wins | +12 | 6 soat | 42 |
| 🟡 Bosqich 3 — OCR/Form/Sign | +6 | 6 soat | 48 |
| 🟢 Bosqich 4 — Image konvertorlar | +7 | 3 soat | 55 |
| 🟣 Bosqich 5 — Markup/Edit | +9 | 5 soat | 64 |
| 🟤 Bosqich 6 — Office/Special | +6 | 4 soat | 70 |
| 🟦 Bosqich 7 — Utility | +5 | 5 soat | 75 |
| ⚫ Bosqich 8 — Niche | +7 (–3 yo'q) | 4 soat | 79 |
| 🟫 Bosqich 9 — Frontend | +3 | — | (frontend) |
| **JAMI** | **+49 endpoint** | **~33 soat** | **79** |

**Go'da hal etilmaydigan: 3 ta** (DWG, AutoCAD, AI Illustrator) — komersial SDK kerak.

---

## 🛠️ Yangi tool stack (qo'shiladi)

| Tool | Hajmi | Ishlatadi | Bosqich |
|------|-------|-----------|---------|
| Tesseract + ocrmypdf | ~600 MB (yangi container) | OCR (#43, #44) | 3 |
| Ghostscript | ~50 MB (apt) | Grayscale (#38), PDF/A (#48) | 2, 3 |
| `golang.org/x/image` | 0 (Go module) | WebP/BMP/TIFF/HEIC | 4 |
| `gosseract` | 0 (Go binding) | OCR alternative | 3 |
| `gofpdf` / `unidoc` | 0 (Go module) | Create PDF, Receipt | 2, 7 |
| Apache FOP yoki XSLT | ~200 MB (Java) | XML → PDF (#68) | 6 |
| DeepL/Google Translate API | 0 (HTTP) | Translate (#72, #73) | 7 |
| DjVuLibre | ~10 MB (apt) | DJVU (#76) | 8 |

---

## 📁 Har bir endpoint uchun standart fayl tuzilmasi

(Phase 1'da o'rnatilgan pattern):
```
api/handler/<feature>.go      # POST + GET endpointlari
service/<feature>.go          # Create + Process + GetByID
api/models/<feature>.go       # Request + Job structs
storage/postgres/<feature>.go # CRUD on <feature>_jobs
migrations/postgres/00XX_*.sql # Yangi jadval (kerak bo'lsa)
pkg/queue/tasks.go            # TaskType const
pkg/queue/handlers.go         # Worker handler
api/router.go                 # Route registration
service/service.go            # Service Manager
storage/storage.go            # Storage interface
storage/postgres/postgres.go  # Repo factory
```

---

## 🚀 Implementation tartib (tavsiya etilgan)

**Hozir → Bosqich 2** (eng oson, ko'p quick wins, mavjud schema'larni ishlatadi):
1. #31 Reorder Pages (jadval bor)
2. #34 PDF to PNG (eng oson)
3. #35 PNG to PDF
4. #38 Grayscale PDF
5. #36 Image to PDF (universal)
6. ... va h.k.

**Keyin Bosqich 3** (yangi tool kerak — Tesseract container deploy qilinishi kerak)

**Keyin Bosqich 4-5** (markup features)

**Eng oxirida Bosqich 6-8** (niche, yuqori effort).

---

## ✅ Tasdiqlash

Bu reja bilan rozimisiz? "Boshla #31'dan" deysizmi yoki boshqa tartib?

Hozircha **Bosqich 2 (12 ta endpoint, 6 soat)** ni avval qilamiz, prod test'lardan o'tkazamiz, keyin keyingisiga o'tamiz — Phase 1'dagi pattern bilan bir xil.

---

*Yaratildi: 2026-05-10. Phase 1 tugagan, smoke test 26/26 PASS.*
