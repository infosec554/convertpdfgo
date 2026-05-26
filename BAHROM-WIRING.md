# Frontend ↔ Backend wiring — 10 ta tool

Backend hozir `convertpdfgo.com`'da live. Sayt UI mavjud, lekin tool sahifalari hech qanday API chaqirmaydi. Bu patch fayl 10 ta tool'ni real API'ga ulaydi.

**Patch fayl**: [`deploy/frontend-wiring.patch`](deploy/frontend-wiring.patch) — 1814 satr, 30 ta fayl.

## Nima ulanadi

| Tool | Backend op | Pattern |
|---|---|---|
| Compress | `/api/pdf/compress` | single file + radio (low/medium/high) |
| Merge | `/api/pdf/merge` | multi-file (2-50 PDF) |
| Split | `/api/pdf/split` | single file + range (`"1-3,5,7-9"`) → multi output |
| Protect | `/api/pdf/protect` | single file + parol + tasdiq |
| Unlock | `/api/pdf/unlock` | single file + parol |
| OCR | `/api/pdf/ocr` | single file + til (eng/rus/uzb/uzb_cyrl/kir/multi) |
| Watermark | `/api/pdf/watermark` | single file + matn + position + opacity + burchak |
| PDF→Word | `/api/pdf/pdf-to-word` | single file → `.docx` |
| Word→PDF | `/api/pdf/word-to-pdf` | `.doc/.docx` → `.pdf` |
| JPG→PDF | `/api/pdf/jpg-to-pdf` | multi-image (JPG/PNG) → 1 PDF |

Va `app/account/files/page.tsx` — **"My files"** sahifa (`GET /file/list`).

## Patch'ni qo'llash

Ikki yo'l bor — qaysi qulay bo'lsa.

### A) `git am` bilan (tavsiya — commit history bilan)

```bash
cd ~/path/to/converto
git checkout -b feat/backend-wiring
curl -sL https://raw.githubusercontent.com/infosec554/convertpdfgo/main/deploy/frontend-wiring.patch | git am
git push origin feat/backend-wiring
# Keyin GitHub'da PR oching va merge qiling
```

### B) `git apply` bilan (commit'siz)

```bash
cd ~/path/to/converto
git checkout -b feat/backend-wiring
curl -sL https://raw.githubusercontent.com/infosec554/convertpdfgo/main/deploy/frontend-wiring.patch | git apply
git add -A
git commit -m "feat: wire 10 tools to backend API"
git push origin feat/backend-wiring
```

### C) Qo'lda nusxa olish (eng oson)

Patch'ni download qil, ko'r, qo'lda paste qil. Yoki Telegram/Discord orqali zip yuboraman. Faqat ayting.

## Patch ichida nima bor

**Yangi (24 fayl):**
- `apps/web/components/tools/file-dropzone.tsx` — drag-drop file picker
- `apps/web/components/tools/use-job-runner.ts` — upload + poll hook
- `apps/web/components/tools/job-status.tsx` — pending/processing/done UI
- `apps/web/components/tools/simple-tool-client.tsx` — universal "1 fayl in → 1 fayl out" client
- `apps/web/app/(tools)/<slug>/<slug>-client.tsx` × 10 — har tool'ning client komponenti
- `apps/web/app/account/files/page.tsx` — "My files"
- (CI/CD): `Dockerfile`, `.dockerignore`, `.github/workflows/deploy.yml`

**O'zgartirilgan:**
- `apps/web/lib/api.ts` — `uploadFile`, `downloadFile`, `runJob`, `runJobSync` qo'shildi
- `apps/web/next.config.mjs` — `output: "standalone"` + `outputFileTracingRoot`
- `apps/web/app/(tools)/<slug>/page.tsx` × 10 — har biriga `<XxxClient />` qo'shildi (statik mockup tegmadi)

## Sinab ko'rish

```bash
cd ~/path/to/converto
pnpm install
pnpm --filter web dev
# Brauzerda: http://localhost:3000/compress
```

Yoki to'g'ridan-to'g'ri production'da: push qilsangiz, GitHub Actions Contabo'ga deploy qiladi, ~2 daqiqada `https://convertpdfgo.com/compress` da yangi widget paydo bo'ladi.

## Arxitektura

```
tool page (server component, marketing card)
        ↓
<XxxClient /> (client component)
        ↓
useJobRunner() — phase: idle → uploading → running → done|failed
        ↓
uploadFile() → POST /file/upload → { id }
        ↓
runJob() → POST /api/pdf/<op> → 202 + { id }
        ↓
poll GET /api/pdf/<op>/<id> har 1.5s → done|failed
        ↓
downloadFile() → GET /file/<output_id>/download → browser save
```

## Hali ulanmagan

- 25 ta boshqa PDF op (rotate, crop, header-footer, qr-code, inspect, ...): qisqasi `SimpleToolClient`'dan nusxa olib pattern'ni takrorlash — patch'ni o'qing, juda oson.
- WebSocket (`/ws`) — polling o'rniga real-time progress. Bahrom xohlasa qo'shamiz.
- Cloud Export (Save to Dropbox/GDrive) — tool natija oynasida tugma.
- Admin paneli — alohida iterasiya.

## Savol

Patch'ni qo'llash bo'yicha biror narsa noaniq bo'lsa — yozing.
