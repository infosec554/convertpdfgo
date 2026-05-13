# 🔍 pdfninja — Qolgan Ishlar

> Yangilangan: 2026-05-13. Production-readiness uchun qaysi ish qolganini sodda ko'rsatadi.

---

## ✅ Tugagan ishlar (zarari yo'q, lekin allaqachon mavjud)

### Middleware (router.go da wired)
- `RequestID()` — har so'rovga unique ID
- `SecurityHeaders()` — XSS, frame-deny, HSTS
- `CORS()` — preflight + Allow-Origin
- `BodyLimit(110 MiB)` — upload cap
- `PrometheusMetrics()` — duration + count
- `RateLimiterMiddleware` — 1 req/sec/IP
- `AuthorizerMiddleware` + `OptionalAuthMiddleware`
- `AdminMiddleware` + `RequirePermission` (Redis-cached)
- `AdminAuditMiddleware` (har bir admin so'rov auth_audit ga)

### Security
- JWT secret fail-fast (`JWT_REQUIRED=true` da `MustValidateSecret` panic qiladi placeholder/short key da)
- OAuth state cookie: HttpOnly + Secure (HTTPS da) + SameSite=Strict/Lax
- 5xx javoblarda `err.Error()` echo qilinmaydi (sirli yo'llar leak bo'lmaydi)
- pgxpool `statement_timeout = 30s` — runaway query oldi olinadi

### Infrastructure
- Worker `/metrics` (:8090) — Prometheus scraping
- Postgres daily backup (systemd timer)
- 36 PDF endpoint, 24+ auth/file/admin/profile endpoint
- Multi-stage Dockerfile (~80MB)
- Asynq queue + 3 priority tier + Asynqmon UI
- Result caching (Redis 24h) + SHA256 dedup

### Tests
- `pkg/jwt` (generate/parse/expire/tamper/secret-validate)
- `pkg/blacklist` (revoke/ttl/nil-safe)
- `pkg/cache`, `pkg/loginlimiter`, `pkg/hash`, `pkg/bufpool`, `pkg/timeouts`
- `service/permission_service` (cache effectiveness)
- `service/user_service` (2-role validation)
- `tests/schema`, `tests/jsontags`, `tests/sqlcasts`
- `tests/integration` (server kerakli)

---

## 🟡 Qolgan ishlar

### B. Quality-of-life — har biri 30-60 min
| # | Item | Joyi |
|---|------|------|
| B1 | README.md yangilash (endpoint list, MinIO, Prometheus) | root |
| B2 | Swagger regen (`swag init`) | docs |
| B3 | "Logout all sessions" endpoint (RevokeUser allaqachon bor, faqat handler kerak) | auth_password.go |
| B4 | Idempotency-Key header (duplicate POST himoyasi) | router |
| B5 | Path traversal himoyasi: `fileHeader.Filename` `filepath.Clean` + reject `..` | file.go |

### C. Phase 2 PDF endpointlari — har biri ~8 fayl, 1-2 soat
PDF_ROADMAP.md da 79 ta planlangan, 36 ta qurilgan. Quick-wins:
- `/pdf/organize` (reorder pages) — pdfcpu da bor
- `/pdf/grayscale` — Ghostscript Dockerfile ga qo'shish
- `/pdf/pdf-to-png`, `/pdf/png-to-pdf` — poppler PNG converter
- `/pdf/search` — pdfcpu text search
- `/pdf/text-to-pdf` — gofpdf
- WebP/BMP/TIFF konverterlar

### D. Katta ish — alohida sprint
| # | Item | Effort |
|---|------|--------|
| D1 | OpenTelemetry tracing (Jaeger) | 1 kun |
| D2 | Load testing (k6/vegeta) + tuning | 0.5 kun |
| D3 | k8s manifests (hozir faqat docker-compose) | 0.5 kun |
| D4 | Audit batching (5 sec flush, hozir per-INSERT) | 2-3 soat |
| D5 | Keyset pagination (List() COUNT(*) ni olib tashlash) | 2-3 soat |

---

## 🎯 Production-readiness vaziyat

**Hozirgi holatda deploy qilish mumkin.** Asosiy infra tugagan. B blok (5 ta polish) deploydan keyin ham qo'shilishi mumkin.

Critical bloker yo'q.

---

*Hujjat 2026-05-13 da yangilandi. Rol modeli 2 darajali (admin, user).*
