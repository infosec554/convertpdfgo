# 🛡️ convertpdfgo — Admin Panel Reja

> Hozirgi adminka **deyarli yo'q**: faqat `user_role == "admin"` deb hardcoded tekshiruv. Bu hujjat to'liq admin panel logikasini bosqichma-bosqich quradi.

---

## 🔴 Joriy holat (audit)

| Komponent | Holat |
|-----------|-------|
| `AdminMiddleware` | ✅ bor — `claims["user_role"] == "admin"` tekshiradi |
| `users.role` ustun | ❌ yo'q — JWT'ga "user_role" kelmaydi (login'da `issueTokens(uid, "user")` hardcoded) |
| **Birinchi admin yaratish** | ❌ yo'q — qanday qilib admin bo'linadi? |
| User management (list/disable/delete) | ❌ yo'q |
| Role-Permission (RBAC) | ⚠️ qisman — `roles` jadvali bor lekin `permissions` yo'q |
| Sysusers (alohida tizim adminlar) | ⚠️ create endpoint bor, lekin login flow'i yo'q |
| Audit viewer (all events) | ❌ yo'q (faqat `/me/auth-history`) |
| Job moderation | ❌ yo'q (cancel/retry/list-all) |
| System stats (DB/disk/queue) | ❌ yo'q |
| Admin action audit | ❌ admin nima qildi — log yo'q |

---

## 🎯 Maqsad: "Real" admin panel

3 darajali RBAC:

```
Superadmin
  ├── barcha permissionlar
  └── boshqa adminlarni yarata oladi

Admin
  ├── users:*, jobs:*, files:read, audit:read, stats:read
  └── superadmin yarata olmaydi

Moderator
  ├── jobs:read, jobs:cancel, files:read, audit:read
  └── userlarni o'chira olmaydi
```

---

## 📋 BOSQICHLAR

### **B1 — Bootstrap & RBAC asosi (1-2 soat)**

**Migration:** `0005_admin_rbac.up.sql`
- `users` jadvaliga `role VARCHAR(20) NOT NULL DEFAULT 'user'` ustun
- `permissions` jadvali (id, name, description)
- `role_permissions` jadvali (role, permission_id)
- Default rol va perm'lar seed: `user`, `moderator`, `admin`, `superadmin`

**Bootstrap logikasi:**
- `cmd/main.go` startup'da: agar `ADMIN_BOOTSTRAP_EMAIL` env bo'lsa va shu email bilan user mavjud bo'lsa → uning role'ini `superadmin` qilish
- Yoki alohida CLI: `go run ./cmd/admin-bootstrap --email=...` 

**JWT'ga rol qo'shish:**
- `service/user_service.go`'da `GetByID` user.Role qaytaradi
- `auth_register_login.Login()` da `issueTokens(uid, user.Role)` (hardcoded "user" emas)

---

### **B2 — Admin middleware permission-based (30 daqiqa)**

**Yangi:** `pkg/middleware/permission.go`

```go
// RequirePermission("users:write") — user_role'dan permissions list olib tekshiradi
func (h Handler) RequirePermission(perm string) gin.HandlerFunc {
    return func(c *gin.Context) {
        role := c.GetString("user_role")
        if !h.services.Role().HasPermission(role, perm) {
            c.AbortWithStatusJSON(403, gin.H{"error": "permission denied: " + perm})
            return
        }
        c.Next()
    }
}
```

**Cache:** Role→permissions Redis'da 5 daqiqa cache (har request DB'ga bormaslik).

**Hozirgi `AdminMiddleware`'ni** saqlab qolaylik (oddiy `role==admin` uchun), lekin yangi joylarda `RequirePermission` ishlatamiz.

---

### **B3 — User management endpoints (1 soat)**

`api/handler/admin_users.go`

| Method | Path | Permission | Tavsif |
|--------|------|------------|--------|
| GET | `/admin/users` | `users:read` | Pagination, filter (status, role, search by email) |
| GET | `/admin/users/:id` | `users:read` | Bitta user + audit history |
| PUT | `/admin/users/:id/role` | `users:write` | Rol o'zgartirish (superadmin → admin etc.) |
| PUT | `/admin/users/:id/status` | `users:write` | active/blocked/deleted |
| DELETE | `/admin/users/:id` | `users:delete` | Soft delete (status=deleted) |
| POST | `/admin/users/:id/revoke-tokens` | `users:write` | Hamma session'ini chiqarish (refresh tokens revoke) |

---

### **B4 — Job moderation (45 daqiqa)**

`api/handler/admin_jobs.go`

| Method | Path | Permission | Tavsif |
|--------|------|------------|--------|
| GET | `/admin/jobs` | `jobs:read` | Barcha jobs, filter (status, type, user_id, date range) |
| GET | `/admin/jobs/:type/:id` | `jobs:read` | Bitta job + log + input/output |
| POST | `/admin/jobs/:type/:id/retry` | `jobs:moderate` | Failed job'ni qayta queue'ga qo'shish |
| DELETE | `/admin/jobs/:type/:id` | `jobs:moderate` | Job + output file delete |
| GET | `/admin/jobs/queue/stats` | `jobs:read` | Asynq queue depth, processing count, failure rate |

**Asynq inspector ishlatish:**
```go
inspector := asynq.NewInspector(queue.RedisOpt(cfg))
queues, _ := inspector.Queues()
for _, q := range queues {
    info, _ := inspector.GetQueueInfo(q)
    // info.Pending, info.Active, info.Completed, ...
}
```

---

### **B5 — Audit viewer (30 daqiqa)**

| Method | Path | Permission | Tavsif |
|--------|------|------------|--------|
| GET | `/admin/audit` | `audit:read` | Pagination, filter (event, email, user_id, IP, date range) |
| GET | `/admin/audit/user/:id` | `audit:read` | Bitta user'ning audit'i |
| GET | `/admin/audit/suspicious` | `audit:read` | Faqat `login_fail` 10+ marta yoki noma'lum IP'lar |

---

### **B6 — System stats dashboard (45 daqiqa)**

`api/handler/admin_stats.go`

| Method | Path | Permission | Tavsif |
|--------|------|------------|--------|
| GET | `/admin/stats/overview` | `stats:read` | Aggregate: users count, jobs (24h/7d/30d), disk usage, queue depth |
| GET | `/admin/stats/users` | `stats:read` | New users per day (chart data, 30 days) |
| GET | `/admin/stats/jobs` | `stats:read` | Jobs per day per type |
| GET | `/admin/stats/storage` | `stats:read` | Disk usage by user (top 50) |
| GET | `/admin/stats/cache` | `stats:read` | Cache hit rate (Prometheus orqali) |

**Endpoint `/metrics` allaqachon bor — admin shu bilan Grafana'dan foydalanadi.**

---

### **B7 — Admin action audit (30 daqiqa)**

Migration: `auth_audit` jadvaliga yana eventlar:
```
admin_user_role_change, admin_user_block, admin_user_delete,
admin_job_retry, admin_job_delete,
admin_login (kim admin panel'ga kirdi)
```

`pkg/middleware/admin_audit.go` — barcha `/admin/*` so'rovlarni audit yozadi:
```go
// Method, Path, ActorID, IP, body (sensitive maydonlar redacted)
```

---

### **B8 — Bootstrap CLI tool (30 daqiqa)**

`cmd/admin-bootstrap/main.go`

```bash
# Birinchi superadmin yaratish
go run ./cmd/admin-bootstrap create-superadmin --email=admin@convertpdfgo.com

# Mavjud user'ni ko'tarish
go run ./cmd/admin-bootstrap promote --email=zarif@example.com --role=admin

# Default permissionlarni seed qilish (idempotent)
go run ./cmd/admin-bootstrap seed-permissions
```

---

## 🗂️ Permission ro'yxati

```sql
INSERT INTO permissions (id, name, description) VALUES
  -- User management
  ('users:read',         'View users'),
  ('users:write',        'Edit users (role, status)'),
  ('users:delete',       'Delete users'),

  -- Sysusers (boshqa adminlar)
  ('sysusers:read',      'View sysusers'),
  ('sysusers:write',     'Create/edit sysusers'),

  -- Roles
  ('roles:read',         'View roles'),
  ('roles:write',        'Create/edit roles'),

  -- Jobs
  ('jobs:read',          'View all jobs'),
  ('jobs:moderate',      'Retry/delete jobs'),

  -- Files
  ('files:read',         'View all files'),
  ('files:delete',       'Delete any file'),

  -- Audit
  ('audit:read',         'View audit log'),

  -- Stats
  ('stats:read',         'View system stats')
;

-- Role → Permissions mapping
-- moderator: jobs:read, jobs:moderate, audit:read, files:read
-- admin:     hammasi (sysusers:write, roles:write tashqari)
-- superadmin: hammasi (jumladan, sysusers:write, roles:write)
```

---

## 📁 Yaratiladigan fayllar (overview)

```
migrations/postgres/
  └── 0005_admin_rbac.up.sql
  └── 0005_admin_rbac.down.sql

cmd/admin-bootstrap/
  └── main.go                       # CLI tool

pkg/middleware/
  └── permission.go                 # RequirePermission(perm)
  └── admin_audit.go                # Auto-audit /admin/*

api/handler/
  ├── admin_users.go                # B3 endpoints
  ├── admin_jobs.go                 # B4 endpoints
  ├── admin_audit.go                # B5 endpoints
  └── admin_stats.go                # B6 endpoints

api/models/
  ├── admin.go                      # CreateUserAdminReq, UpdateRoleReq, etc.
  └── permission.go                 # Permission, RolePermission

storage/
  ├── storage.go                    # IPermissionStorage, IAdminStorage
  └── postgres/
      ├── permission.go             # Permission CRUD
      └── admin.go                  # User list/update/disable

service/
  ├── permission_service.go         # HasPermission(role, perm) + Redis cache
  └── admin_service.go              # User mgmt biznes logikasi

api/router.go
  └── /admin/* group RequirePermission(...) bilan
```

---

## ⏱️ Vaqt sarflashi

| Bosqich | Vaqt | Priority |
|---------|------|----------|
| B1 — Bootstrap & RBAC asosi | 1-2 soat | 🔴 Critical |
| B2 — Permission middleware + cache | 30 min | 🔴 Critical |
| B3 — User management | 1 soat | 🟠 High |
| B4 — Job moderation | 45 min | 🟠 High |
| B5 — Audit viewer | 30 min | 🟡 Medium |
| B6 — System stats | 45 min | 🟡 Medium |
| B7 — Admin action audit | 30 min | 🟠 High |
| B8 — Bootstrap CLI | 30 min | 🔴 Critical |
| **JAMI** | **5-6 soat** | |

---

## 🎬 Ishga tushirish (B1+B2+B8 dan keyin)

```bash
# 1. DB migration
make migrate

# 2. Permissions seed
go run ./cmd/admin-bootstrap seed-permissions

# 3. Birinchi superadmin
go run ./cmd/admin-bootstrap create-superadmin --email=admin@example.com

# 4. Login → access token endi {role: superadmin} bilan keladi
curl -X POST http://localhost:8080/auth/login \
  -d '{"email":"admin@example.com","password":"..."}'

# 5. Admin endpoint'ni ishlatish
curl http://localhost:8080/admin/users \
  -H "Authorization: Bearer $TOKEN"
```

---

## ❌ Hozir BO'LMAGAN narsalar (qoladi)

- 2FA admin'lar uchun (TOTP)
- Admin UI (faqat REST API; frontend alohida loyihada bo'ladi)
- Audit log retention (hozir cheksiz; kelajakda 90 kun)
- Webhook (yangi user yaratilganda Slack/Telegram)

---

## ✅ B1-B8 tugagandan keyin nima bo'ladi

- ✅ JWT'da role + permissions
- ✅ Birinchi superadmin yaratish (CLI)
- ✅ User management (list, role change, block, delete, force logout)
- ✅ Job moderation (list, retry, delete)
- ✅ Audit log viewer (filter, search)
- ✅ System stats (overview, users/day, jobs/day, disk)
- ✅ Har bir admin amali — audit'ga yoziladi
- ✅ Permission-based access control (granular)
- ✅ Redis cache (role→permissions, 5 min TTL)

**Tahminiy effort: 5-6 soat**

---

*Yaratilgan: 2026-05-06*
