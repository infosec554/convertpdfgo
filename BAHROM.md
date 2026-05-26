# Frontend — Contabo Deploy

Salom! Backend Contabo'ga deploy qilingan, frontend ham birinchi marta ulanib bo'ldi (live: <https://convertpdfgo.com>).

Endi CI/CD ishlashi uchun shu 4 ta faylni `BahromWeb/converto` repo'ga commit + push qil. Bundan keyin **har push'da avtomatik deploy** bo'ladi (~2 daqiqada).

---

## 1. `Dockerfile` (repo root)

```dockerfile
# Dockerfile — apps/web Next.js production image
FROM node:20-alpine AS deps
RUN corepack enable && corepack prepare pnpm@9.12.3 --activate
WORKDIR /app
COPY . .
RUN pnpm install --frozen-lockfile

FROM node:20-alpine AS builder
RUN corepack enable && corepack prepare pnpm@9.12.3 --activate
WORKDIR /app
COPY --from=deps /app /app
ENV NEXT_TELEMETRY_DISABLED=1
RUN pnpm --filter web build

FROM node:20-alpine AS runner
WORKDIR /app
ENV NODE_ENV=production
ENV NEXT_TELEMETRY_DISABLED=1
ENV PORT=3000
ENV HOSTNAME=0.0.0.0
COPY --from=builder /app/apps/web/.next/standalone ./
COPY --from=builder /app/apps/web/.next/static ./apps/web/.next/static
COPY --from=builder /app/apps/web/public ./apps/web/public
EXPOSE 3000
CMD ["node", "apps/web/server.js"]
```

---

## 2. `.dockerignore` (repo root)

```
node_modules
**/node_modules
.next
**/.next
.turbo
**/.turbo
.git
.github
*.md
.env*
!.env.example
.vscode
.idea
coverage
dist
build
```

---

## 3. `apps/web/next.config.mjs` (mavjudini ALMASHTIR)

```js
import path from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = path.dirname(fileURLToPath(import.meta.url));

/** @type {import('next').NextConfig} */
const nextConfig = {
  reactStrictMode: true,
  transpilePackages: ["@converto/ui", "@converto/utils", "@converto/types", "@converto/data"],
  output: "standalone",
  outputFileTracingRoot: path.join(__dirname, "../../"),
  experimental: {
    optimizePackageImports: ["lucide-react"],
  },
};

export default nextConfig;
```

> `output: "standalone"` + `outputFileTracingRoot` — bu ikkalasi monorepo'da Next standalone build uchun kerak.

---

## 4. `.github/workflows/deploy.yml` (yangi fayl)

```yaml
name: Deploy frontend to Contabo

on:
  push:
    branches: [main]
  workflow_dispatch:

jobs:
  deploy:
    runs-on: ubuntu-latest
    timeout-minutes: 15
    steps:
      - name: Deploy via SSH
        uses: appleboy/ssh-action@v1.2.0
        with:
          host: ${{ secrets.SSH_HOST }}
          username: ${{ secrets.SSH_USER }}
          password: ${{ secrets.SSH_PASSWORD }}
          script_stop: true
          script: |
            set -e
            cd /root/converto
            git fetch --quiet origin main
            git reset --hard origin/main
            cd /root/convertpdfgo
            docker compose --profile full up -d --build web
            sleep 5
            docker compose --profile full ps web
```

---

## 5. Commit + Push

```bash
git add Dockerfile .dockerignore apps/web/next.config.mjs .github/workflows/deploy.yml
git commit -m "feat(deploy): Dockerfile + Contabo CI/CD"
git push origin main
```

---

## ✅ Tekshirish

1. GitHub repo → **Actions** tab — workflow ishlayotganini ko'rasan
2. ~2 daqiqada `https://convertpdfgo.com` ga sening o'zgarishing tushadi

**Secrets** (Settings → Secrets and variables → Actions) — allaqachon qo'shilgan: `SSH_HOST`, `SSH_USER`, `SSH_PASSWORD` ✅

---

## ℹ️ Qo'shimcha

- Backend (`/api/*`, `/auth/google`, `/swagger`, va h.k.) — Caddy avtomatik backend'ga yuboradi
- Frontend SPA (`/`, `/compress-pdf`, `/auth/callback`) — sening Next.js
- OAuth callback: backend → `https://convertpdfgo.com/auth/callback?access_token=...&refresh_token=...` → frontend SPA tokenni saqlaydi

Savol bo'lsa, yoz.
