# 🛡️ pdfninja — Admin Panel

> Joriy holat (2026-05-13): admin panel **deyarli to'liq tugagan**. Bu hujjat aktual qurilgan tarkibni va minimal qolgan ishlarni tasvirlaydi.

---

## 🎯 Rol modeli — 2 darajali

**Faqat 2 ta rol** mavjud: `admin` va `user`. Eski `moderator` / `superadmin` migratsiya 0014 da `admin` ga yig'ildi.

```
admin   ← barcha permissionlar (users:*, jobs:*, files:*, audit:*, stats:*)
user    ← admin endpointlariga ruxsat yo'q (oddiy file/PDF ishlash huquqi)
```

JWT'da `user_role` claim shu ikkisidan biri bo'ladi.

---

## ✅ Qurilgan endpointlar

### `/admin/users` — user management
| Method | Path | Permission |
|--------|------|------------|
| GET    | `/admin/users` | `users:read` |
| GET    | `/admin/users/:id` | `users:read` |
| PUT    | `/admin/users/:id/role` | `users:write` |
| PUT    | `/admin/users/:id/status` | `users:write` |
| DELETE | `/admin/users/:id` | `users:delete` |
| POST   | `/admin/users/:id/restore` | `users:write` |
| POST   | `/admin/users/:id/reset-password` | `users:write` |
| POST   | `/admin/users/:id/revoke-tokens` | `users:write` |
| POST   | `/admin/users/invite` | `users:write` |

### `/admin/files` — cross-user files
- GET `/admin/files` — `files:read`
- DELETE `/admin/files/:id` — `files:delete`
- GET `/admin/files/cleanup` — `files:delete` (eski fayllarni o'chirish)

### `/admin/jobs` — Asynq inspector
- GET `/admin/jobs/:queue/:state` — `jobs:read`
- POST `/admin/jobs/:queue/:id/retry` — `jobs:moderate`
- POST `/admin/jobs/:queue/:id/archive` — `jobs:moderate`
- DELETE `/admin/jobs/:queue/:id` — `jobs:moderate`
- GET `/admin/jobs/logs/:id` — `jobs:read`

### `/admin/audit` — auth_audit reader
- GET `/admin/audit` — `audit:read`
- GET `/admin/audit/recent` — `audit:read`
- GET `/admin/audit/user/:id` — `audit:read`
- GET `/admin/audit/suspicious` — `audit:read`

### `/admin/stats` — dashboard
- GET `/admin/stats/overview` — users + queue depth (1 query)
- GET `/admin/stats/users` — per-day signups
- GET `/admin/stats/jobs` — jobs per day per type
- GET `/admin/stats/storage` — top disk consumers
- GET `/admin/stats/cache` — Prometheus cache hit rate

### `/admin/auth` — login lockouts
- GET `/admin/auth/locked` — `audit:read`
- POST `/admin/auth/unlock` — `users:write`

### `/admin/roles` — role CRUD (kelajak uchun, hozir o'rinli emas)

### `/admin/contacts` — user→admin inbox + admin reply

---

## 🔧 Middleware oqimi

```
/admin/* group:
    1. AuthorizerMiddleware  ← JWT validate
    2. AdminAuditMiddleware  ← har bir admin so'rovni auth_audit ga yozadi
    3. RequirePermission(...) ← per-route permission gate
```

`RequirePermission("users:write")` Redis-cached (5 min TTL) — qora yo'l DB ga bormaydi.
`AdminMiddleware` (eski `role==admin` versiyasi) `/admin/jobs/logs/:id` uchun saqlangan.

---

## 🚀 Bootstrap (birinchi admin yaratish)

```bash
# 1. Migration
make migrate

# 2. Userni admin qilish (oldindan registratsiya qilingan bo'lishi shart)
go run ./cmd/admin-bootstrap promote --email=zarif@example.com --role=admin

# 3. Qabul qilingan permissionlarni tekshirish
go run ./cmd/admin-bootstrap list-roles
```

---

## 📁 Asosiy fayllar

```
api/handler/
  admin_middleware.go        — RequirePermission, RequireAnyPermission
  admin_audit_middleware.go  — barcha /admin/* so'rovlarni audit ga yozadi
  admin_users.go             — user list/role/status/delete/restore/invite
  admin_jobs.go              — Asynq inspector
  admin_audit.go             — auth_audit viewer
  admin_files.go             — cross-user file management
  admin_stats.go             — dashboard
  admin_locks.go             — login lockout viewer

service/
  permission_service.go      — HasPermission(role, perm) + 2-katlam cache
  user_service.go            — UpdateRole(2 ta valid: user|admin)

cmd/admin-bootstrap/main.go  — promote + list-roles + list-permissions CLI

migrations/postgres/
  0005_admin_rbac.up.sql            — permissions + role_permissions
  0014_collapse_roles_and_invites.up.sql — 4→2 role kollapsi
```

---

## ❌ HALI qoldirilgan ish

- **2FA admin'lar uchun** (TOTP) — keyinroq
- **Webhook** (yangi user / suspicious login → Slack/Telegram) — keyinroq
- **Audit log retention** — hozir cheksiz; 90 kunlik retention task'i kerak
- **Admin UI** — bu repo faqat REST API; frontend alohida loyiha

---

*Yangilangan: 2026-05-13. Rol modeli 2 darajali (admin, user).*
