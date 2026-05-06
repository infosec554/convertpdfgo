# 🔍 convertpdfgo — Qolgan Ishlar (Honest Audit)

> 2026-05-06 holatiga ko'ra. Optimizatsiya 95% tugagan, lekin **production'ga ketishdan oldin** quyidagilar kerak.

---

## 🟡 Mavjud middleware'lar (allaqachon bor)

| Middleware | Joyi | Maqsadi |
|------------|------|---------|
| `gin.Default()` | router | Recovery + access log |
| `gzip.Gzip()` | router | Response compression |
| `RateLimiterMiddleware` | `pkg/middleware/rate_limiter.go` | 1 req/sec/IP, burst 3 |
| `AuthorizerMiddleware` | `api/handler/auth.go` | JWT validation + blacklist tekshiruvi |
| `AdminMiddleware` | `api/handler/auth.go` | role=admin tekshiruvi |

❌ **Yo'qlari:** CORS, Security Headers, Request ID, Prometheus HTTP middleware, body size limit

---

## 🔴 To'liq qolganlar (priority bo'yicha)

### A. Birinchi navbat (production-ready bo'lish uchun) — **15-30 daqiqa**

| # | Item | Fayl | Effort |
|---|------|------|--------|
| A1 | **Build verifikatsiya** | — | network qaytsa |
| A2 | **CORS middleware** | `pkg/middleware/cors.go` | 5 daqiqa |
| A3 | **Security headers** (XSS, frame deny, HSTS) | `pkg/middleware/security.go` | 5 daqiqa |
| A4 | **Request ID middleware** | `pkg/middleware/requestid.go` | 5 daqiqa |
| A5 | **Prometheus HTTP middleware** | `pkg/middleware/metrics.go` | 5 daqiqa |
| A6 | **Body size limit** (50MB max) | `pkg/middleware/bodylimit.go` | 3 daqiqa |
| A7 | **JWT secret fail-fast** (bo'sh bo'lsa startup fail) | `cmd/main.go` | 3 daqiqa |
| A8 | **`.env.example`** | root | 5 daqiqa |
| A9 | **Worker `/metrics` endpoint** | `cmd/worker/main.go` | 5 daqiqa |
| A10 | **OAuth cookie Secure/SameSite** | `api/handler/oauth.go` | 3 daqiqa |
| A11 | **statement_timeout pgxpool** | `storage/postgres/postgres.go` | 3 daqiqa |
| A12 | **Error sanitization** (handleResponse'dan err.Error olib tashlash) | `api/handler/handler.go` | 5 daqiqa |

### B. Ikkinchi navbat — **1-2 soat**

| # | Item | Joyi |
|---|------|------|
| B1 | README.md yangilash (yangi endpoint'lar, MinIO, Prometheus) | root |
| B2 | Swagger regen (`swag init`) | docs |
| B3 | Per-session tracking (Redis: `sessions:{user_id}:[]jti`) | new |
| B4 | "Logout all sessions" endpoint | auth |
| B5 | Password change → revoke all sessions | auth |
| B6 | TOCTOU fix — atomic file delete | cleaner |
| B7 | Multipart `MaxMemory` config | upload handler |

### C. Katta ish — alohida sprint

| # | Item | Effort |
|---|------|--------|
| C1 | **Test coverage** (jwt, cache, blacklist, auth flows) | 1 kun |
| C2 | **OpenTelemetry tracing** (Jaeger) | 1 kun |
| C3 | **Multi-Gotenberg + nginx LB** | 0.5 kun |
| C4 | **Integration tests** (testcontainers + asynq) | 1 kun |
| C5 | **Load testing** (k6 / vegeta) + tuning | 0.5 kun |

---

## ✅ Yaxshi tomonlari (allaqachon bor)

- Async queue (Asynq + 19 ta task type)
- Redis result caching
- File deduplication (SHA256)
- 2 darajali cleanup (5min/15min + 6h/7d)
- Prometheus metrics (job duration, cache hit/miss)
- pprof endpoint (lokal)
- Auth: register/login/logout/refresh/OAuth/password reset/audit
- Brute-force shield (5 fail/10min)
- Token blacklist (Redis)
- Graceful shutdown
- HTTP/2 + connection pool (Gotenberg)
- gzip + sync.Pool + bufpool
- Multi-stage Dockerfile
- docker-compose: API/worker/Postgres/Redis/Gotenberg/Asynqmon/MinIO/Prometheus/Grafana
- Cron-based cleanup (har 6 soat)

---

## 🎯 Hozir qiladigan ishlar (A1-A12)

Quyida sanab o'tilgan tartibda bajaramiz.
