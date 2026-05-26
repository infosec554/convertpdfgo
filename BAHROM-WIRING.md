# Frontend ↔ Backend wiring — barcha 35 ta tool + Cloud export

Backend `convertpdfgo.com`'da live (HTTPS + OAuth + 35 PDF op). Frontend tool sahifalari oldin static mockup edi — bu patch hammasini funksional qiladi.

**Patch fayl**: [`deploy/frontend-wiring.patch`](deploy/frontend-wiring.patch) — 4459 satr, 2 commit, ~60 fayl.

## Nimalar ulanadi

### 35 ta PDF tool — barchasi funksional

**Asosiy oqim** (allaqachon mockup'i bor — widget MOCKUP TAGIDA qo'shiladi):
| Tool | Pattern |
|---|---|
| Compress, Merge, Split | core flows |
| Protect, Unlock | parol input |
| OCR, Watermark | til/option tanlash |
| PDF→Word/Excel/PPT/HTML/JPG, Word/Excel/PPT/HTML→PDF, JPG→PDF | konversiya |

**Yangi tool sahifalari** (mockup yo'q — to'g'ridan-to'g'ri funksional):
| Tool | Maxsus parametr |
|---|---|
| Rotate | burchak + sahifa range |
| Crop | top/right/bottom/left + sahifa range |
| Add page numbers | start, position, color, font size |
| Header & Footer | matn, font, color, position |
| QR Code | content, position, size |
| N-up | 2/4/6/9 + orientation + border |
| Inspect ⚡ SYNC | page count + metadata darhol |
| Detect blank | bo'sh sahifalar ro'yxati |
| Edit metadata | title/author/subject/keywords/creator |
| Remove pages | "1,4-6" formati |
| Extract pages | "1-3,5,7-9" formati |
| Scan to PDF | multi-image + til |
| Image to text (OCR) | rasm + til |
| HTML to PDF | textarea (fayl emas) |
| URL to PDF | URL + page size + landscape + wait_for |
| Extract text | txt / json |

### Fayl boshqaruvi
- **`/account/files`** — yuklangan fayllar ro'yxati (`GET /file/list`)
- **`POST /file/upload`** — har tool ichida, multipart
- **`GET /file/<id>/download`** — har tool natijasi tugmasidan + My files'dan
- **`DELETE /file/<id>`** — My files'dan

### Cloud Export ☁️
- **`/account/connections`** — Dropbox / Google Drive ulash / uzish
- **Har natija oynasida** "Save to Dropbox" / "Save to Drive" tugmasi avtomatik chiqadi (agar ulangan bo'lsa)
- Funksiya: `exportFileToCloud(fileId, "dropbox" | "gdrive")`

### Auth (allaqachon ulangan, eslatma)
- Google + GitHub OAuth
- `/auth/callback` SPA sahifa (token query → localStorage)
- Token refresh + logout
- `/me`, `/me/auth-history`, profil yangilash

---

## Patch'ni qo'llash

### A) `git am` (tavsiya)

```bash
cd ~/path/to/converto
git checkout -b feat/backend-wiring
curl -sL https://raw.githubusercontent.com/infosec554/convertpdfgo/main/deploy/frontend-wiring.patch | git am
git push origin feat/backend-wiring
# GitHub'da PR oching va main'ga merge qiling
# Push qilingach GitHub Actions Contabo'ga deploy qiladi (~2 daqiqa)
```

### B) `git apply` (bitta commit'siz)

```bash
cd ~/path/to/converto
git checkout -b feat/backend-wiring
curl -sL https://raw.githubusercontent.com/infosec554/convertpdfgo/main/deploy/frontend-wiring.patch | git apply --3way
git add -A
git commit -m "feat: wire 35 PDF tools + cloud export to backend"
git push origin feat/backend-wiring
```

---

## Arxitektura

```
tool page (server component)
    ↓
<XxxClient /> (client component)
    ↓
useJobRunner() — idle → uploading → running → done|failed
    ↓
uploadFile() → POST /file/upload
    ↓
runJob() → POST /api/pdf/<op> → 202 + { id }
    ↓
poll GET /api/pdf/<op>/<id> har 1.5s
    ↓
JobStatusPanel:
    ├── Download button
    └── <SaveToCloud /> — Dropbox/Drive tugmalari
```

**Maxsus**: `Inspect` sync (polling yo'q), `HTML/URL to PDF` upload talab qilmaydi (body'dagi matn/URL).

---

## Foydalanish (sinab ko'rish)

```bash
cd ~/path/to/converto
pnpm install
pnpm --filter web dev
# http://localhost:3000/compress
# http://localhost:3000/rotate
# http://localhost:3000/inspect
# http://localhost:3000/account/files
# http://localhost:3000/account/connections
```

---

## Hali to'g'rilanmagan

- **WebSocket** real-time progress (hozir polling)
- **Admin paneli** (alohida iterasiya)
- **Chat + Sign** AI/E-sign tool'lari (backend'da yo'q)
- **Marketing mockup'lar** 25 ta yangi sahifa uchun — hozir faqat funksional widget (eski 10 ta tool'da mockup tegmasdan widget pastda)

---

Savol bo'lsa yozing.
