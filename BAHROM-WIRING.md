# Frontend wiring — bitta buyruq, hammasi tushadi

Backend `convertpdfgo.com`'da live. Quyidagi patch frontend'ga **35 ta PDF tool + cloud export + My files**'ni ulaydi va 10 ta tool sahifasidagi **dublikat upload zonasini olib tashlaydi**.

Patch fayl: <https://gist.github.com/infosec554/226ffa333b155e4930f233be05d80f74> · 3 commit · ~70 fayl

---

## Qo'llash — bir nafasda

```bash
cd ~/path/to/converto
git checkout main && git pull
git checkout -b feat/backend-wiring
curl -sL https://gist.githubusercontent.com/infosec554/226ffa333b155e4930f233be05d80f74/raw/frontend-wiring.patch | git am
git push origin feat/backend-wiring
```

GitHub'da PR och → `main`'ga merge → GitHub Actions ~2 daqiqada Contabo'ga deploy qiladi.

> Konflikt chiqsa: `git am --abort` keyin `git apply --3way` orqali yana urinib ko'r:
> ```bash
> curl -sL <patch-url> | git apply --3way --whitespace=fix
> git add -A && git commit -m "feat: wire backend + merge tool cards"
> ```

---

## Patch'da nima bor

**3 commit:**
1. `feat: wire backend API to all 10 tools + foundation` — `useJobRunner`, `SimpleToolClient`, `useFileApi`, dropzone, status panel
2. `feat: wire remaining 25 PDF ops + cloud export` — qolgan tool client'lari, `/account/files`, `/account/connections`, `SaveToCloud`
3. `feat(tools): merge 10 mockups into single functional cards` — **eng yangi**: `compress, merge, split, word-to-pdf, jpg-to-pdf, pdf-to-word, protect, unlock, ocr, watermark` — bitta `<slug>-card.tsx`, dublikat upload yo'q, optional Discard tugmasi

**35 ta tool — barchasi funksional:**
| Guruh | Tool'lar |
|---|---|
| Asosiy | Compress · Merge · Split · Rotate · Crop · Extract · Remove pages |
| Konversiya | PDF↔Word/Excel/PPT/HTML/JPG · Word/Excel/PPT/HTML/URL→PDF · JPG→PDF · Scan→PDF |
| Xavfsizlik | Protect · Unlock |
| OCR | OCR · OCR detect lang · OCR extract text · OCR structured · Image to text |
| Tahrir | Watermark · Add page numbers · Header & Footer · N-up · QR code |
| Metadata | Inspect ⚡SYNC · Detect blank · Edit metadata · Extract text |

**Fayl boshqaruvi:** `POST /file/upload` · `GET /file/<id>/download` · `DELETE /file/<id>` · `/account/files`

**Cloud export:** `/account/connections` (Dropbox/Drive ulash) — har natija oynasida tugma avtomatik chiqadi

---

## Lokal sinov

```bash
pnpm install
pnpm --filter web dev
# http://localhost:3000/compress
# http://localhost:3000/protect       — endi bitta upload zonasi
# http://localhost:3000/account/files
```

---

## Hali yo'q

- WebSocket progress (hozir 1.5s polling)
- Admin paneli
- `chat`, `sign` tool'lari — backend'da yo'q

Savol bo'lsa yoz.
