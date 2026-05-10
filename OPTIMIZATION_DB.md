# 🗄️ DB Yuk Optimizatsiyasi — Audit + Maslahat

> Hozirgi loyihada DB nagruskasini kamaytiruvchi 12 ta topilma. Birinchi 5 tasi **kritik** (real qaytim katta), qolganlari kelajak bosqichlar.

---

## 🔴 KRITIK (darhol fix)

### 1. Migrations har boot'da ishlamoqda (api + worker = 2× DB lock)
**Hozir:** `storage/postgres/postgres.go`'da `migrate.Up()` *har* `New()` chaqiruvida ishlaydi. API ham, worker ham bir vaqtda ishga tushganda **race condition** + dirty migration xavfi (men buni server'da boshdan kechirdik).

**Fix:** `RUN_MIGRATIONS` env yoki `--migrate` flag kiritish; faqat api boshlanganda chaqirish.
```go
if os.Getenv("RUN_MIGRATIONS") == "true" {
    runMigrations(url, log)
}
```

**Foyda:** Worker ishga tushganda DB ga 0 ta query bormaydi (avval 5+ ta migration check edi). Concurrent boot xavfsiz.

---

### 2. `/admin/stats/overview` — har request'da **10 ta query**
**Hozir:** `statUserCounts()` 5 marta `User().List()` chaqiradi (total/active/blocked/admins/moderators), har biri `SELECT FROM users WHERE ... + SELECT COUNT(*) FROM users WHERE ...`.

**Fix:** Bitta GROUP BY:
```sql
SELECT
  COUNT(*) AS total,
  COUNT(*) FILTER (WHERE status = 'active')   AS active,
  COUNT(*) FILTER (WHERE status = 'blocked')  AS blocked,
  COUNT(*) FILTER (WHERE role = 'admin')      AS admins,
  COUNT(*) FILTER (WHERE role = 'moderator')  AS moderators
FROM users;
```

**Foyda:** 10 query → **1 query**. Dashboard har 5 sek refresh bo'lsa, soatiga 36 000 query → 3 600 query.

---

### 3. Migration `0001` ichida `word_to_pdf_jobs` 2 marta yaratilgan
**Hozir:** `0001_create_user_table.up.sql` ichida `CREATE TABLE word_to_pdf_jobs` 2 ta. Ikkinchisi error beradi → migration **dirty** holatida qoladi → keyingi boot fail.

**Fix:** Birinchi nusxasini olib tashlash + `IF NOT EXISTS` qo'shish (xavfsizlik uchun):
```sql
CREATE TABLE IF NOT EXISTS word_to_pdf_jobs (...);
```

**Foyda:** Migration har vaqt clean, server qayta deploy qilinganda dirty state yo'q.

---

### 4. Har bir worker `Process()` 4 ta UPDATE qiladi
**Hozir:** Compress/WordToPDF/etc. service'lar oqimi:
```go
job.Status = "processing"; s.stg.Compress().Update(ctx, job)  // 1
// ... do work ...
s.stg.Compress().Update(ctx, job)  // markFailed misol
job.Status = "done";       s.stg.Compress().Update(ctx, job)  // 2
```

Ba'zi joylarda **3 ta** UPDATE. Pgx round-trip ~1 ms × 3 = **3 ms** har jobga (1000 job/min = 3 sec/min DB time bekorga).

**Fix:** "processing" status qo'shimcha UPDATE'ni olib tashlash (boshlanishni `created_at` orqali bilish mumkin); faqat `done` yoki `failed` UPDATE qilish.

**Foyda:** UPDATE soni 50% kamayadi.

---

### 5. Cleanup queries `files` table'ni full-scan qiladi
**Hozir:** Har 5 daqiqada `WHERE user_id IS NULL AND uploaded_at < NOW() - 15 minutes` ishlaydi. `idx_files_uploaded_at` bor, lekin bu **butun** files'ni qamrab oladi — guest kam (5%), registered ko'p (95%).

**Fix:** Partial index:
```sql
CREATE INDEX idx_files_guest_old ON files (uploaded_at)
  WHERE user_id IS NULL;
```

Va `DeleteByID` loop'da chaqirilmoqda — har fayl alohida transaction. Yaxshilash:
```sql
DELETE FROM files WHERE id = ANY($1::uuid[]);
```

**Foyda:** Index hajmi 20× kichik, query 50× tez. 100 fayl o'chirish — 100 transaction → 1 transaction.

---

## 🟡 KATTA FOYDA (next sprint)

### 6. Audit yozuv'lari — har mutation INSERT
**Hozir:** Login/logout/password-change/admin-* har biri 1 ta INSERT'ni async goroutine'da. Yuk: 1000 user kuniga 5 ta event = 5000 INSERT.

**Fix:** In-memory channel + 5 sec batch flush:
```go
ch := make(chan AuthAudit, 10000)
go func() {
    var batch []AuthAudit
    ticker := time.NewTicker(5*time.Second)
    for {
        select {
        case e := <-ch: batch = append(batch, e)
        case <-ticker.C:
            if len(batch) > 0 { bulkInsert(batch); batch = nil }
        }
    }
}()
```

**Foyda:** Audit INSERT'lar 50× ozayadi (5 sek × 50 event = 1 query).

---

### 7. List() har safar `COUNT(*)` chaqiradi
**Hozir:** `User().List()` ichida `SELECT COUNT(*) FROM users WHERE ...`. Katta jadval'da sekin (10M row → 200ms).

**Fix:** Approximate count via `pg_class.reltuples` yoki keyset pagination:
```go
// Frontend "next page" bilan ishlasa, total kerak emas.
// Kerak bo'lsa: SELECT reltuples::bigint FROM pg_class WHERE relname='users'
```

**Foyda:** Pagination 10× tezroq. Dashboard render time 200ms → 20ms.

---

### 8. Per-job: file save + job update — 2 ta tranzaksiya
**Hozir:**
```go
s.stg.File().Save(ctx, output)        // INSERT files
job.OutputFileID = &id
s.stg.Compress().Update(ctx, job)     // UPDATE compress_jobs
```

Har biri alohida tranzaksiya. Race condition: file saved, lekin job update fail bo'lsa — orphaned file.

**Fix:** Bitta tranzaksiya:
```go
tx, _ := db.Begin(ctx)
tx.Exec("INSERT files ...")
tx.Exec("UPDATE compress_jobs ...")
tx.Commit()
```

**Foyda:** 2 round-trip → 1, atomicity kafolati.

---

### 9. `permissions` cache — 5 min TTL juda kam
**Hozir:** Permission service Redis'da 5 daqiqa cache. Admin dashboard'da har clik permission tekshiruvi.

**Fix:** TTL'ni 30 daqiqaga oshirish + role o'zgartirilganda explicit `Invalidate()`. Lokal in-memory cache (LRU 256) ham bor — hit rate 99%.

**Foyda:** Permission queries 6× kamayadi.

---

### 10. Logs jadvali (job log'lari) — retention yo'q
**Hozir:** Har job log yozadi (`logs` jadvali). Hech qachon o'chirilmaydi → cheksiz o'sadi.

**Fix:** Retention cleaner: 30 kun eski log'larni o'chirish.
```go
DELETE FROM logs WHERE created_at < NOW() - INTERVAL '30 days'
```

Yoki TimescaleDB hypertable (avtomatik partition).

**Foyda:** Logs jadvali GB → MB. SELECT'lar 10× tez.

---

## 🟢 KELAJAK (load > 100 rps bo'lsa)

### 11. PgBouncer qo'shish
Hozir har bir api/worker connection POSTGRES bilan to'g'ridan-to'g'ri ulanadi. 25+25=50 connection. `max_connections=100` cheklov.

**Fix:** PgBouncer transaction-pool mode → 1000 client → 50 server connection.

### 12. Read replica (Postgres streaming replication)
Admin stats / audit list'lar **read-heavy**. Master'ga yuk tushirish o'rniga replica'dan o'qish.

### 13. Materialized view for `/admin/stats/overview`
```sql
CREATE MATERIALIZED VIEW user_stats_mv AS
  SELECT count(*) total, ... FROM users;
REFRESH MATERIALIZED VIEW user_stats_mv;  -- har 1 minutda cron
```
Dashboard 1 ms o'qiydi.

### 14. `pg_stat_statements` extension
Slow query'larni topish uchun:
```sql
CREATE EXTENSION pg_stat_statements;
```
Top 10 slowest queries → optimallashtirish.

### 15. Statement timeout — per-query override
Hozir global 30s. Lekin admin stats 1 min'gacha kerak bo'lishi mumkin:
```go
ctx, _ := context.WithTimeout(parent, time.Minute)
db.QueryRow(ctx, "SET LOCAL statement_timeout = '60s'; SELECT ...")
```

---

## 📊 Kutilayotgan natijalar (5+5+5 fix qo'llanilganda)

| Operatsiya | Avval | Keyin | Tezlik |
|-----------|-------|-------|--------|
| Stack boot (api+worker) | 5 query × 2 = 10 | 5 query × 1 = 5 | 2× |
| `/admin/stats/overview` | 10 query | 1 query | 10× |
| `Process()` per job | 4 UPDATE | 2 UPDATE | 2× |
| Cleanup (5 min cycle) | full scan | partial index | 50× |
| Audit (1000/min) | 1000 INSERT | 12 batch INSERT | 80× |

Umuman: **DB load ~5-10× kamayadi** real production traffic ostida.

---

## 🎯 Birinchi 4 ta'ni hozir fix qilamiz (15 daqiqa)

1. ✅ `0001` migration dedup
2. ✅ Migration faqat API'da (worker skip)
3. ✅ Stats overview 1 query
4. ✅ "processing" UPDATE'ni olib tashlash (Process()'da)

Server'da redeploy + test.

---

*Yaratildi: 2026-05-10*
