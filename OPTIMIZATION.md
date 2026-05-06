# 🚀 convertpdfgo — Optimizatsiya Reja & Holati

> 5 fazali yo'l xaritasi. Hozirgi holat: **Faza 1, 2, 3 (caching qismi), 4 va 5 ning ko'p qismi tugatildi.**

---

## ✅ FAZA 1 — Tezkor G'alabalar (DONE)

| # | Qadam | Fayl |
|---|-------|------|
| 1.1 | HTTP client pool (Gotenberg) | [pkg/gotenberg/clint.go](pkg/gotenberg/clint.go) |
| 1.2 | Streaming I/O (io.Pipe + io.Copy) | [pkg/gotenberg/clint.go](pkg/gotenberg/clint.go), 5 service |
| 1.3 | Rate limiter memory leak fix | [pkg/middleware/rate_limiter.go](pkg/middleware/rate_limiter.go) |
| 1.4 | Graceful shutdown + bind 0.0.0.0 | [cmd/main.go](cmd/main.go) |
| 1.5 | Markazlashgan timeouts (DB=5s, Process=5min) | [pkg/timeouts/timeouts.go](pkg/timeouts/timeouts.go) |

---

## ✅ FAZA 2 — Async Job Queue (DONE)

| # | Qadam | Fayl |
|---|-------|------|
| 2.1 | Asynq dependency | `go.mod` |
| 2.2 | pkg/queue paketi (client, tasks, handlers, payloads) | [pkg/queue/](pkg/queue/) |
| 2.3 | Worker binary | [cmd/worker/main.go](cmd/worker/main.go) |
| 2.4 | 19 ta service'da `Create + Process` ajratildi | service/* |
| 2.5 | 19 ta handler enqueue pattern'iga o'tdi | api/handler/* |
| 2.6 | docker-compose ga **asynqmon** (port 8081) | [docker-compose.yml](docker-compose.yml) |
| 2.7 | 3 ta queue: critical / default / low | [pkg/queue/queue.go](pkg/queue/queue.go) |
| 2.8 | Unlock — password task payload (DB ga yozilmaydi) | [pkg/queue/tasks.go](pkg/queue/tasks.go) |

**Pattern:**
```
POST /api/pdf/X      → Create() (DB pending) + Enqueue() → 202 Accepted (~5ms)
GET  /api/pdf/X/:id  → DB status (~5ms)
worker.Process(id)   → service.Process() → DB done/failed
```

---

## ✅ FAZA 3 — Caching va Deduplication (1-qism DONE)

| # | Qadam | Fayl |
|---|-------|------|
| 3.1 | SHA256 streaming hashing | [pkg/hash/hash.go](pkg/hash/hash.go) |
| 3.2 | Redis result cache wrapper | [pkg/cache/cache.go](pkg/cache/cache.go) |
| 3.3 | `files.sha256` ustun + 7 ta indeks | [migrations/postgres/0003_*.sql](migrations/postgres/0003_add_file_sha256_and_indexes.up.sql) |
| 3.4 | File upload — single-pass hashing va dedup | [api/handler/file.go](api/handler/file.go) |
| 3.5 | Compress + 5 converter result cache (24h TTL) | service/compress, word_to_pdf, excel_to_pdf, ppt_to_pdf, pdf_to_word, html_to_pdf |
| 3.6 | `FindBySHA256` storage metodi | [storage/postgres/files.go](storage/postgres/files.go) |
| **TODO** | MinIO (S3-compatible) — keyinroq | docker-compose, pkg/objectstore |

**Cache key'lar:**
- `pdf:compress:{sha}:{level}`
- `pdf:word_to_pdf:{sha}`, `pdf:excel_to_pdf:{sha}`, `pdf:ppt_to_pdf:{sha}`
- `pdf:pdf_to_word:{sha}`, `pdf:html_to_pdf:{contentHash}`

---

## ✅ FAZA 4 — DB va Memory Optimizatsiya (DONE)

| # | Qadam | Fayl |
|---|-------|------|
| 4.1 | pgxpool tuning (Max=25, Min=5, Lifetime=1h) | [storage/postgres/postgres.go](storage/postgres/postgres.go) |
| 4.2 | DB indekslar (files, jobs jadvallarida) | migration 0003 |
| 4.3 | sync.Pool buffer reuse | [pkg/bufpool/bufpool.go](pkg/bufpool/bufpool.go) |
| **TODO** | Logging volume kamaytirish (Info → Debug) | hamma service |

---

## ✅ FAZA 5 — Production-grade (asosan DONE)

| # | Qadam | Fayl |
|---|-------|------|
| 5.1 | gzip middleware (response compression) | [api/router.go](api/router.go) |
| 5.2 | pprof endpoint (lokal :6060) | [cmd/main.go](cmd/main.go) |
| 5.3 | Cron-based cleanup (har 6 soat) | [pkg/cleaner/cleaner.go](pkg/cleaner/cleaner.go) |
| 5.4 | Multi-stage Dockerfile (~15MB image) | [Dockerfile](Dockerfile) |
| 5.5 | docker-compose: API + worker container'lari (profile=full) | [docker-compose.yml](docker-compose.yml) |
| 5.6 | Health check (`/healthz`, `/readyz`) | [api/router.go](api/router.go) |
| 5.7 | Makefile: `up-full`, `worker`, `pprof`, `tidy`, `lint` | [Makefile](Makefile) |
| **TODO** | Prometheus metrics | new |
| **TODO** | OpenTelemetry tracing | new |
| **TODO** | Multiple Gotenberg instances + nginx | docker-compose |

---

## 🚀 Ishga tushirish

### Lokal (development)
```bash
# Network tiklangach:
GOSUMDB=off go mod tidy
make build

# Infrastructure:
make up                     # postgres + redis + gotenberg + asynqmon

# 2 alohida terminal:
make run                    # API (port 8080)
make worker                 # Worker (concurrency=20)
```

### Docker production
```bash
make up-full                # hammasi: postgres + redis + gotenberg + asynqmon + API + worker
```

### Monitoring
- **Swagger:** http://localhost:8080/swagger/index.html
- **Asynqmon:** http://localhost:8081 (queue monitoring)
- **pprof:** http://127.0.0.1:6060/debug/pprof/ (heap, goroutines)
- **Health:** http://localhost:8080/healthz

---

## 📊 Kutilayotgan natija

| Metrika | Avval | Hozir |
|---------|-------|-------|
| 100MB PDF konvertatsiya RAM | ~150MB | ~5MB |
| `POST /api/pdf/word-to-pdf` latency | 30s sync | ~5ms (queued) |
| Bir xil fayl qayta upload | yangi yozuv + disk | dedup'd, eski ID qaytadi |
| Bir xil compress qayta | 30s ishlaydi | ~5ms (cache hit) |
| Container o'chirilganda | so'rovlar yo'qoladi | 30s drain |
| TCP connection per request | yangi | reuse |
| Rate limiter memory | cheksiz o'sadi | 10min TTL |
| pprof | yo'q | :6060 |
| Disk to'lib qoladi | manual cleanup | har 6h auto |

---

## ✅ Bonus optimizatsiyalar (kech qo'shilgan)

| # | Item | Holat |
|---|------|-------|
| B.1 | **Aggressive cleanup** — guest fayllar 15 min, har 5 min | ✅ |
| B.2 | **Prometheus metrics** — `/metrics`, job histogram, cache counters | ✅ |
| B.3 | **MinIO** — `pkg/objectstore/` interface + LocalStore + MinIOStore | ✅ |
| B.4 | **Logging review** — `Info` shovqinlari olib tashlandi (role/sysuser/otp/user/redis/merge/file) | ✅ |
| B.5 | **Grafana** — docker-compose'da, `/metrics` Prometheus orqali | ✅ |
| B.6 | **Auth qayta yozildi** — register/login/logout/refresh/password-reset/profile + token blacklist | ✅ |
| TODO | **OpenTelemetry tracing** — Jaeger (kelajakda) | — |
| TODO | **Multiple Gotenberg + nginx** — horizontal scaling | — |

---

## 🌐 Tarmoq portlari (yakuniy)

| Port | Servis | UI URL |
|------|--------|--------|
| 8080 | API | `/swagger/index.html` |
| 8081 | Asynqmon | `http://localhost:8081` |
| 9090 | Prometheus | `http://localhost:9090` |
| 3001 | Grafana | `http://localhost:3001` (admin/admin) |
| 9001 | MinIO console | `http://localhost:9001` |
| 9000 | MinIO S3 API | `http://localhost:9000` |
| 6060 | pprof (lokal) | `http://127.0.0.1:6060/debug/pprof/` |

---

*Yangilangan: 2026-05-06*
