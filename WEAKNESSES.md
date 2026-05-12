# 🔍 convertpdfgo — Tuzatish ro'yxati

> Bitta flat priority list. Yuqorida — eng katta og'riq. Pastda — kosmetik.
> Har item: bir qatorlik fix yo'li + soat hisobi. Birma-bir bosamiz.

---

## 🔴 P0 — bularni qilmasdan keyingi feature yozmaslik kerak

| # | Muammo | Fix | Vaqt |
|---|--------|-----|------|
| 1 | ~~Integration test yo'q~~ ✅ **DONE** — `tests/integration/`, 25 test, `make test` + `make test-integration` | done | (3.5s) |
| 2 | ~~CI yo'q~~ ✅ **DONE** — vet+gofmt / unit / build / docker / **integration (live stack)** / vuln scan | done | (2.5s) |
| 3 | ~~Postgres parol drift~~ ✅ **DONE** — `deploy/postgres/sync-password.sh` har boot'da `ALTER USER` via Unix socket. Drift simulyatsiyasi prod'da pass. | done | (45m) |
| 4 | ~~Schema↔kod sync yo'q~~ ✅ **DONE** — `tests/schema/schema_test.go` static check. 2 ta yangi bug topdi: `userstats.go` jadval bo'lmagan `convert_*_pdf_jobs`'ni so'rardi (`/me/stats` 500). | done | (1h) |

> P0 jami: **8.5 soat**. Bularsiz har keyingi feature yana 20 ta yashirin bug bilan keladi.

---

## 🟠 P1 — har keyingi developer (yoki kelajakdagi siz) uchun og'riq

| # | Muammo | Fix | Vaqt |
|---|--------|-----|------|
| 5 | ~~UserID inkonsistent~~ ✅ **DONE** — `userPtr(c)` helper + 4 ta service/model `*string`'ga konvert. | done | (1h) |
| 6 | ~~Service `Create` 3 xil~~ ✅ **DONE** — inspect + detect-blank ham `req T` pattern'ga keldi. Yagona shakl: `Create(ctx, req T, userID *string) (jobID, error)`. | done | (15m) |
| 7 | ~~Response envelope farq~~ ✅ **DONE** — `models.JobCreatedResponse{ID, Status}` + 4 ta `JobStatus*` const. 26 handler unified. | done | (1h) |
| 8 | ~~JSON tag yo'q~~ ✅ **DONE** — `tests/jsontags/` AST audit. `File` snake_case keldi, `Response` PascalCase aniq tag bilan, dead code (FileUploadRequest, MergeJobInputFile) o'chirildi. | done | (45m) |
| 9 | ~~Status code farq~~ ✅ **DONE** (#7 bilan) — async = 202, sync inspect = 200. PostJob test client buni qattiq tekshiradi. | done | — |
| 10 | ~~OID-25 pgx bug~~ ✅ **DONE** — `tests/sqlcasts/` `(…)::TYPE` ichida cast'siz `$N` qidiradi. Sanity bilan 3 ta historik bug detect qilindi. | done | (45m) |
| 11 | ~~`/file/:id/download` owner check yo'q~~ ✅ **DONE** — `OptionalAuthMiddleware` + handler ownership gate. Guest files open; owned files faqat owner/admin'ga. Prod'da curl bilan tasdiqlangan. | done | (45m) |
| 12 | ~~Manual tarball deploy~~ ✅ **DONE** — release.yml har main push'da GHCR'ga `sha-<short>` + `:main`. `deploy/deploy.sh <tag>` pull qiladi. Server'da bir martalik GHCR public/login setup kerak. | done | (1h) |
| 13 | ~~Audit silent failures~~ ✅ **DONE** — `metrics.SilentError(op)` counter, swallow-error joylarga wired; `deploy/alerts.yml` Prometheus rule (op > 1/min for 5m → page). | done | (45m) |
| 14 | ~~Rate limiter UA-key~~ ✅ **DONE** — IP-only key, UA komponentini olib tashladim. UA-less callers endi alohida-alohida bucket'larda. | done | (10m) |

> P1 jami: **~11 soat**.

---

## 🟡 P2 — code quality, future-proof

| # | Muammo | Fix | Vaqt |
|---|--------|-----|------|
| 15 | Fayl typolar: `instpect.go`, `merga.go` | Rename + import update | 30m |
| 16 | Aralash komment (uz+en), `// 👉 YANGI QATOR`, `// TO'G'RI variant:` | Cleanup pass | 1s |
| 17 | `/healthz` va `/readyz` bir xil — readyz DB ping qilishi kerak | Pool `Ping(ctx)` | 15m |
| 18 | godotenv error har boot'da log'da (`.env` containerda yo'q) | Container'da chaqirmaslik (build-time flag) | 15m |
| 19 | Cleaner metrics yo'q (qancha o'chirildi qachon) | Prometheus counter | 30m |
| 20 | Job progress (0-100%) yo'q — UX yomon katta PDF'da | `progress_pct INT` ustun + worker periodic update | 2s |
| 21 | Webhook callback / SSE yo'q — frontend poll qilishi kerak | `/api/jobs/:id/events` SSE stream | 2s |
| 22 | Slow query log yo'q | `log_min_duration_statement = 500ms` postgres'da | 15m |
| 23 | Cache TTL global 24h — feature-specific bo'lishi kerak | `cache.New(redis).With("compress", 7*24h)` | 30m |
| 24 | LibreOffice worker concurrency bo'sh — sekvensial konversiyalar tiqilib qoladi | Separate `office` queue + dedicated worker | 1s |
| 25 | Idempotency-Key header yo'q — duplicate POST 2 ta job yaratadi | Redis cache `idempotency:<hash>` 1 daqiqaga | 1s |
| 26 | `Content-Type` `.pdf` (extension) qaytariladi, MIME emas | Save'da `mime.TypeByExtension()` | 30m |
| 27 | Path traversal — `fileHeader.Filename` to'g'ridan-to'g'ri ishlatiladi | `filepath.Clean` + reject `..` | 30m |
| 28 | Index'lar: `*_jobs.user_id` ba'zilarida yo'q | Migration: idx audit | 1s |

> P2 jami: **~13 soat**.

---

## 🐛 Real bug'lar (integration test sweep'da topilgan, alohida ro'yxat)

| Marker | Muammo | Fix yo'li | Severity |
|--------|--------|-----------|----------|
| ~~**pdf-to-word**~~ ✅ | ~~Gotenberg `/forms/libreoffice/convert` faqat office→PDF~~ — worker endi `pkg/libreoffice` orqali to'g'ridan-to'g'ri LibreOffice CLI'ga shell qiladi (`--infilter=writer_pdf_import`). DOCX (ZIP) magic prod'da tasdiqlangan. | done | 🟠 |

## 🟢 P3 — kosmetik, "polish"

| # | Muammo | Fix | Vaqt |
|---|--------|-----|------|
| 29 | README quickstart yo'q yoki ozod | `make up && curl /healthz` 5-line quickstart | 1s |
| 30 | Swagger regen manual (`make generate`) | Pre-commit hook | 30m |
| 31 | Architecture diagram yo'q | Mermaid PNG: queue flow | 1s |
| 32 | `addbackground.go` model bor, endpoint yo'q (dead code) | Delete | 15m |
| 33 | `translate_jobs` jadval bor, endpoint yo'q | Phase 7'da implement | (later) |
| 34 | LICENSE yo'q (yoki tekshirilmagan) | MIT/Apache choose | 5m |
| 35 | nginx config repo'da yo'q (faqat server'da) | `deploy/nginx.conf` | 30m |
| 36 | Test fixture'lar `/tmp/fixtures/` manual | `testdata/` repo'ga | 30m |
| 37 | bcrypt cost hardcoded | config'dan | 15m |
| 38 | golangci-lint config yo'q | `.golangci.yml` + CI | 30m |
| 39 | Backup strategy yo'q (postgres dump) | `pg_dump` cron + retention | 1s |
| 40 | `pdf_inspect_jobs.created_at` DB default + Go `time.Now()` — dual source | DB default'ni o'chir, faqat Go | 15m |

> P3 jami: **~7 soat**.

---

## 📊 Hisob

| Daraja | Soni | Vaqt |
|--------|------|------|
| 🔴 P0 | 4 | 8.5s |
| 🟠 P1 | 10 | 11s |
| 🟡 P2 | 14 | 13s |
| 🟢 P3 | 12 | 7s |
| **JAMI** | **40** | **~40 soat** (1 hafta) |

---

## 🚀 Boshlash

**#1 — Integration test runner** (4 soat). Bu birinchi.

Bajariladigan ishlar:
1. `Makefile`'ga `test`, `test:integration`, `test:unit` target qo'shish
2. `tests/integration/conversions_test.go` — hozirgi `test_all.sh` Go test'iga ko'chirish
3. `testdata/` papka — test fixture'lar (pdf, jpg, docx, xlsx, pptx)
4. Lokal va CI'da bir xil ishlasin (env: `BASE_URL`, `SKIP_OFFICE`)
5. **Real assertion'lar**: output download size > 0, content-type to'g'ri, PDF page count kutilgandek

`boshla` deysizmi yoki #1 ning detallari boshqacha bo'lsin?
