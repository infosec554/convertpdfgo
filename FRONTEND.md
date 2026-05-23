# convertpdfgo — Frontend bilan ishlash bo'yicha qo'llanma

> **Mijoz/frontend dasturchi uchun.** Loyihaning hamma 35 ta PDF operatsiyasi, autentifikatsiya, fayl boshqaruvi va WebSocket'lar haqida amaliy ma'lumot.

**Base URL:** `https://convertpdfgo.com`
**Swagger UI:** https://convertpdfgo.com/swagger/index.html
**OpenAPI JSON:** https://convertpdfgo.com/swagger/doc.json

---

## 1. Asosiy tushunchalar

### 1.1 Foydalanuvchi turlari

| Tur | Yuklash limiti | Auth kerakmi |
|---|---|---|
| **Mehmon** (guest) | 20 MB | Yo'q |
| **Ro'yxatdan o'tgan** | 30 MB | Ha (JWT Bearer) |

20 MB'dan katta fayl uchun mehmon HTTP `401` oladi:
```json
{"StatusCode":401,"Description":"Unauthorized","Data":{
  "limit_bytes":20971520,
  "file_size":34567890,
  "sign_up_path":"/auth/google"
}}
```

→ Frontend bu javobni ko'rsa, "Katta fayl uchun Google/GitHub orqali kiring" tugmasini ko'rsatishi kerak.

### 1.2 Autentifikatsiya — faqat OAuth

Email+parol login YO'Q. Faqat ikkita yo'l:

```
https://convertpdfgo.com/auth/google
https://convertpdfgo.com/auth/github
```

Brauzerda ochiladi → user provider'da ruxsat beradi → `access_token` + `refresh_token` qaytadi.

```json
{
  "StatusCode": 200,
  "Description": "OK",
  "Data": {
    "access_token":  "eyJ...",
    "refresh_token": "eyJ..."
  }
}
```

**Har bir auth-talab qiluvchi so'rovda:**
```
Authorization: Bearer <access_token>
```

**Token muddati tugagach** (1 soat):
```
POST /auth/refresh
{"refresh_token": "<refresh_token>"}
```

### 1.3 Javob formati — har doim bir xil

Hamma API javobi shu shaklda:
```json
{
  "StatusCode": <http_status>,
  "Description": "OK" | "Bad Request" | "Unauthorized" | "Internal Server Error",
  "Data": <payload yoki xato detalisi>
}
```

### 1.4 Validatsiya xatolari — frontend uchun tayyor

400 javob har bir field uchun xato beradi:
```json
{
  "StatusCode": 400,
  "Description": "Bad Request",
  "Data": {
    "errors": {
      "input_file_id": "InputFileID is required",
      "mode": "Mode is required"
    }
  }
}
```

Frontend bevosita form-fielddagi xato sifatida ko'rsatishi mumkin.

---

## 2. Job hayot sikli — async ishlashning umumiy pattern'i

35 ta PDF operatsiyaning **34 tasi async** (faqat `/inspect` sync). Pattern bir xil:

### 2.1 Job yaratish — `POST /api/pdf/<op>`

```http
POST /api/pdf/compress
Content-Type: application/json

{"input_file_id": "uuid-here", "compression": "medium"}
```

Javob (HTTP **202**):
```json
{
  "StatusCode": 202,
  "Data": {"id": "job-uuid", "status": "pending"}
}
```

### 2.2 Polling — `GET /api/pdf/<op>/<job_id>`

Har 1-2 soniyada chaqiring:
```http
GET /api/pdf/compress/<job_id>
Authorization: Bearer <token>
```

3 ta holatdan biri:
```json
// 1. Hali ishlanyapti
{"Data": {"id": "...", "status": "pending"}}

// 2. Tugadi
{"Data": {"id": "...", "status": "done", "output_file_id": "result-uuid"}}

// 3. Xato
{"Data": {"id": "...", "status": "failed", "error": "..."}}
```

### 2.3 Natijani yuklab olish

```http
GET /file/<output_file_id>/download
Authorization: Bearer <token>   ← agar user ro'yxatdan o'tgan bo'lsa
```

### 2.4 Tezroq variant — WebSocket bilan real-time

Polling o'rniga WebSocket'dan job-update event'larini olish:
```
wss://convertpdfgo.com/ws?token=<access_token>
```

Server JSON event yuboradi:
```json
{"type":"job.update", "job_id":"...", "status":"done", "output_file_id":"..."}
```

---

## 3. Fayl boshqaruvi

| Endpoint | Method | Auth | Vazifa |
|---|---|---|---|
| `/file/upload` | POST | Optional (limit farqi) | Multipart file yuklash |
| `/file/{id}` | GET | Optional (ACL) | Fayl metadata |
| `/file/{id}/download` | GET | Optional (ACL) | Binary content |
| `/file/{id}` | DELETE | Optional (ACL) | O'chirish |
| `/file/list` | GET | **Required** | Mening fayllarim |

**Upload misol:**
```js
const fd = new FormData();
fd.append("file", fileBlob);
fetch("https://convertpdfgo.com/file/upload", {
  method: "POST",
  headers: { Authorization: `Bearer ${token}` },  // optional
  body: fd
}).then(r => r.json()).then(j => console.log(j.Data.id));
```

**Dedup:** Bir xil content (SHA256 mos) yuklasangiz, eski `file_id` qaytariladi va `"dedup": true` belgisi keladi. **Dedup user-scoped:** mehmon va auth user har biri o'ziga alohida (privacy himoyalangan).

**Auto-cleanup:** Hamma yuklangan fayllar **5 daqiqadan keyin** avtomatik o'chiriladi (privacy uchun). Foydalanuvchi natijani **darhol yuklab olishi kerak**. Frontend "5 daqiqada o'chadi" ogohlantirishi tavsiya etiladi.

---

## 4. 35 ta PDF Operatsiyasi — to'liq qo'llanma

### Sahifa bilan ishlash (5 ta)

#### `/api/pdf/merge` — bir nechta PDF'ni birlashtirish

| Maydon | Tur | Talab | Izoh |
|---|---|---|---|
| `input_file_ids` | uuid[] | ✅ | Min 2, **max 50** PDF |

**Misol:**
```json
{"input_file_ids": ["uuid1", "uuid2", "uuid3"]}
```

**Savol — "Frontend nechta fayl yuklashga ruxsat berishi kerak?"** → 2 dan 50 gacha. Tartib saqlanadi.

**Frontend UI:** Multi-file picker + drag-and-drop tartib o'zgartirish.

---

#### `/api/pdf/split` — PDF'ni bo'laklarga ajratish

| Maydon | Tur | Talab | Izoh |
|---|---|---|---|
| `input_file_id` | uuid | ✅ | |
| `split_ranges` | string | ✅ | "1-3,4-5" yoki "1,3,5" |

**Misol:** `"1-3,4-7,8-10"` → 3 ta alohida PDF chiqaradi (1-3, 4-7, 8-10 sahifalari).

**Output:** `output_file_ids: [uuid, ...]` — har bir bo'lak alohida.

**Frontend UI:** Range picker yoki "Har sahifani alohida fayl qil" tugmasi.

---

#### `/api/pdf/removepage` — sahifalarni o'chirish

| Maydon | Tur | Talab | Izoh |
|---|---|---|---|
| `input_file_id` | uuid | ✅ | |
| `pages_to_remove` | string | ✅ | "1,4-6" |

---

#### `/api/pdf/extract` — kerakli sahifalarni ajratib olish

| Maydon | Tur | Talab | Izoh |
|---|---|---|---|
| `input_file_id` | uuid | ✅ | |
| `page_ranges` | string | ✅ | "1-3,5,7-9" |

⚠️ **Diqqat:** field nomi `page_ranges` (lekin removepage'da `pages_to_remove`).

---

#### `/api/pdf/compress` — siqish

| Maydon | Tur | Talab | Qiymatlar |
|---|---|---|---|
| `input_file_id` | uuid | ✅ | |
| `compression` | string | ✅ | `low` / `medium` / `high` |

| Daraja | Natija |
|---|---|
| `low` | ~30-50% kichikroq, eng yuqori sifat |
| `medium` | ~50-70% kichikroq, normal sifat (tavsiya) |
| `high` | ~70%+ kichikroq, past sifat (skan/email uchun) |

**Frontend UI:** Radio button + estimated savings ko'rsatkichi.

---

### Vizual o'zgartirishlar (6 ta)

#### `/api/pdf/rotate` — aylantirish

| Maydon | Tur | Talab | Qiymatlar |
|---|---|---|---|
| `input_file_id` | uuid | ✅ | |
| `angle` | int | ✅ | 90, 180, 270 |
| `pages` | string | ❌ | "all" (default), "1-3,5" |

**Savol — "10 varoqli PDF'ni hammasini aylantirsa bo'ladimi?"** → Ha, ✅
```json
{"input_file_id": "uuid", "angle": 90, "pages": "all"}
```
Yoki `pages` qoldirib qo'ying — default "all".

**Savol — "Faqat 3 va 7-sahifani aylantirsa-chi?"** → Ha:
```json
{"input_file_id": "uuid", "angle": 180, "pages": "3,7"}
```

---

#### `/api/pdf/crop` — kesish

| Maydon | Tur | Talab | Izoh |
|---|---|---|---|
| `input_file_id` | uuid | ✅ | |
| `top` / `bottom` / `left` / `right` | int | ❌ | Pikselda |
| `pages` | string | ❌ | "all" yoki range |
| `box` | string | ❌ | mediabox/cropbox |

---

#### `/api/pdf/add-page-numbers` — sahifa raqamlari

| Maydon | Tur | Talab | Izoh |
|---|---|---|---|
| `input_file_id` | uuid | ✅ | |
| `first_number` | int (≥1) | ✅ | Birinchi raqam |
| `page_range` | string | ✅ | "all" yoki range |
| `position` | string | ✅ | `bottom-right` va h.k. |
| `color` | string | ✅ | "black", "red", hex |
| `font_size` | int (≥1) | ✅ | px (odatda 10-14) |

---

#### `/api/pdf/header-footer` — sarlavha/quyi qatorlar

| Maydon | Tur | Talab |
|---|---|---|
| `input_file_id` | uuid | ✅ |
| `header_text` | string | ❌ |
| `footer_text` | string | ❌ |
| `font_size` | int | ❌ (default 12) |
| `font_color` | string | ❌ (default "black") |
| `position` | string | ❌ ("left"/"center"/"right") |

---

#### `/api/pdf/qr-code` — QR kod qo'shish

| Maydon | Tur | Talab |
|---|---|---|
| `input_file_id` | uuid | ✅ |
| `qr_content` | string | ✅ |
| `position` | string | ✅ |
| `size` | int | ✅ (pikselda) |

---

#### `/api/pdf/watermark` — vodyanoy znak

| Maydon | Tur | Talab | Izoh |
|---|---|---|---|
| `input_file_id` | uuid | ✅ | |
| `mode` | string | ✅ | `text` yoki `image` |
| `text` | string | mode=text uchun | |
| `image_file_id` | uuid | mode=image uchun | Avval rasm yuklang |
| `position` | string | ❌ | pdfcpu: `c`, `tl`, `tr`, `bl`, `br` |
| `rotation` | int | ❌ | -90 dan 90 gacha |
| `opacity` | float | ❌ | 0.0-1.0 (default 0.4) |
| `pages` | string | ❌ | "all" yoki range |

---

### Xavfsizlik (2 ta)

#### `/api/pdf/protect` — parol qo'yish

```json
{"input_file_id": "uuid", "password": "secret123"}
```

#### `/api/pdf/unlock` — parolni olib tashlash

```json
{"input_file_id": "uuid", "password": "secret123"}
```

**Eslatma:** Agar PDF haqiqatda himoyalangan bo'lmasa, job `failed` qaytadi ("not encrypted").

---

### Tahlil (2 ta)

#### `/api/pdf/inspect` — PDF metadata o'qish ⚡ SYNC

⚠️ **Yagona sync operatsiya.** POST darhol 200 + natija qaytaradi (polling kerakmas):

```json
{"file_id": "uuid"}  ← field nomi `file_id`, `input_file_id` emas!
```

Javob:
```json
{
  "id": "...",
  "file_id": "...",
  "page_count": 12,
  "title": "...",
  "author": "...",
  "subject": "...",
  "keywords": "...",
  "status": "done"
}
```

**Yomon PDF uchun HTTP 422** "invalid or malformed PDF" — yangi xatti-harakat.

---

#### `/api/pdf/detect-blank` — bo'sh sahifalarni topish

```json
{"input_file_id": "uuid"}
```
Javob: `{"blank_pages": [3, 7, 12]}`

---

### Rasm ↔ PDF (2 ta)

#### `/api/pdf/jpg-to-pdf` — rasmlardan PDF

| Maydon | Tur | Talab | Izoh |
|---|---|---|---|
| `input_file_ids` | uuid[] | ✅ | Min 1, **max 50** rasm |

**Savol — "Nechta rasmni bir vaqtda birlashtirsa bo'ladi?"** → 1 dan 50 gacha JPG/PNG. Tartib saqlanadi, har biri = 1 sahifa.

**Output:** `output_file_id` (yagona PDF) + `zip_file_id` (ixtiyoriy, alohida ZIP).

---

#### `/api/pdf/pdf-to-jpg` — PDF har sahifasini JPG qilish

```json
{"input_file_id": "uuid"}
```

**Output:** 
```json
{"output_file_ids": ["jpg1", "jpg2", "..."], "zip_file_id": "zip-uuid"}
```

10 varoqli PDF → 10 ta JPG + 1 ta ZIP.

---

### Office → PDF (5 ta)

| Operatsiya | Endpoint | Qabul qiladigan formatlar |
|---|---|---|
| Word → PDF | `/api/pdf/word-to-pdf` | `.doc`, `.docx` |
| Excel → PDF | `/api/pdf/excel-to-pdf` | `.xls`, `.xlsx` |
| PowerPoint → PDF | `/api/pdf/ppt-to-pdf` | `.ppt`, `.pptx` |
| HTML → PDF | `/api/pdf/html-to-pdf` | HTML matn (file emas!) |
| URL → PDF | `/api/pdf/url-to-pdf` | URL |

**Hammasi bir xil shape:**
```json
{"input_file_id": "uuid"}
```

**`html-to-pdf` farqli:**
```json
{"html_content": "<html><body><h1>Salom</h1></body></html>"}
```

**`url-to-pdf` farqli:**
```json
{
  "url": "https://example.com",
  "page_size": "A4",       // ixtiyoriy: A3, A4, A5, B4, B5
  "landscape": false,      // ixtiyoriy
  "wait_for": "2s"         // ixtiyoriy: render kutish
}
```

Backend Gotenberg (Chromium) ishlatadi → CSS to'liq qo'llab-quvvatlanadi.

---

### PDF → Office (3 ta)

| Operatsiya | Endpoint | Chiqish |
|---|---|---|
| PDF → Word | `/api/pdf/pdf-to-word` | `.docx` |
| PDF → PowerPoint | `/api/pdf/pdf-to-ppt` | `.pptx` (1 sahifa = 1 slayd) |
| PDF → Excel | `/api/pdf/pdf-to-excel` | `.xlsx` (layout-aware) |
| PDF → HTML | `/api/pdf/pdf-to-html` | `.html` |

**Hammasi bir xil:** `{"input_file_id": "uuid"}`

---

### OCR oilasi (6 ta)

Hammasi `lang` parametrini qabul qiladi. **Tesseract til kodlari:**

| Kod | Til |
|---|---|
| `eng` | Inglizcha (default) |
| `rus` | Ruscha |
| `uzb` | O'zbekcha (lotin) |
| `uzb_cyrl` | O'zbekcha (kirill) |
| `kir` | Qirg'izcha |
| `eng+rus` | Bir nechta — `+` bilan |

#### `/api/pdf/ocr` — searchable PDF yaratish

```json
{
  "input_file_id": "uuid",
  "lang": "eng+rus",
  "force_ocr": false
}
```

Output: matnli qatlam qo'shilgan yangi PDF.

#### `/api/pdf/scan-to-pdf` — rasm/skanlardan OCR PDF

```json
{
  "input_file_ids": ["jpg1", "jpg2"],   // max 50
  "lang": "uzb"
}
```

#### `/api/pdf/ocr-extract-text` — PDF'dan oddiy matn (.txt)

#### `/api/pdf/image-to-text` — bitta rasmdan matn (.txt)

⚠️ Kichkina rasm (3 piksel'dan kichik) `failed` qaytaradi.

#### `/api/pdf/ocr-detect-lang` — tilni aniqlash

```json
{"input_file_id": "uuid"}
```
Output: `{"detected_lang": "eng", "confidence": 0.97}`

⚠️ Juda kam matnli PDF'da "Too few characters" deb `failed` qaytadi.

#### `/api/pdf/ocr-structured` — OCR + tuzilma (HOCR/JSON)

---

### Boshqa (5 ta)

#### `/api/pdf/extract-text` — PDF'dan matn (OCR'siz)

```json
{
  "input_file_id": "uuid",
  "format": "txt"        // yoki "json"
}
```

#### `/api/pdf/nup` — bir varaqqa N sahifa

```json
{
  "input_file_id": "uuid",
  "n": 4,                // 2, 4, 6, 9
  "orientation": "portrait",  // yoki "landscape"
  "border": true
}
```

Foydali: 10 sahifali PDF → 4-up bilan 3 varaq.

#### `/api/pdf/metadata` — PDF metadata o'zgartirish

```json
{
  "input_file_id": "uuid",
  "title": "Yangi nom",
  "author": "Men",
  "subject": "...",
  "keywords": ["pdf", "convert"],
  "creator": "convertpdfgo"
}
```

⚠️ Faqat update mode. Hech bir field bermasangiz `failed` ("no metadata fields provided"). Eski metadata ko'rish uchun `/api/pdf/inspect` ishlating.

#### `/api/pdf/url-to-pdf` — yuqorida ko'rsatilgan

#### `/api/pdf/share` — public sharing link

```http
POST /api/pdf/share
{"file_id": "uuid", "ttl_minutes": 60}
```

Javob: `{"token": "abc..."}`

Public yuklab olish: `https://convertpdfgo.com/api/pdf/share/<token>`

Default TTL: 24 soat. Token expire bo'lsa 404.

---

## 5. WebSocket — real-time job updates

### Foydalanuvchi WS

```js
const ws = new WebSocket(`wss://convertpdfgo.com/ws?token=${accessToken}`);

ws.onmessage = (event) => {
  const data = JSON.parse(event.data);
  // {type:"job.update", job_id:"...", status:"done", output_file_id:"..."}
};
```

Backend job tugaganda darhol push qiladi — polling shart emas.

### Admin WS (faqat admin role)

```
wss://convertpdfgo.com/ws/admin?token=<admin_access_token>
```

Event'lar:
- `admin.user.registered`
- `admin.contact.new`
- `admin.auth.suspicious`

---

## 6. Cloud Export ("Save to ...")

| Endpoint | Vazifa |
|---|---|
| `GET /api/storage/providers` | Mavjud provider'lar ro'yxati |
| `GET /api/storage/{provider}/connect` | OAuth flow boshlash |
| `GET /oauth/{provider}/callback` | OAuth qaytishi |
| `POST /file/{id}/export` | Faylni cloud'ga eksport |
| `DELETE /api/storage/{provider}` | Provider'ni uzish |

Hozir mavjud:
- ✅ **Dropbox** (sozlangan)
- ✅ **Google Drive** (sozlangan)
- ⏸️ OneDrive (sozlanmagan — UI'da chiqmaydi)

**Misol — providers ro'yxatini olish:**
```json
[
  {"name":"dropbox", "display_name":"Dropbox", "configured":true, "connected":false},
  {"name":"gdrive",  "display_name":"Google Drive", "configured":true, "connected":true}
]
```

`connected: true` bo'lsa, "Save to ..." tugmasini ko'rsating. `connected: false` bo'lsa, oldin `/api/storage/{provider}/connect` ga yo'naltiring.

---

## 7. Foydalanuvchi profili

| Endpoint | Method | Vazifa |
|---|---|---|
| `/me` | GET | Profil ko'rish |
| `/me` | PUT | Ismni o'zgartirish (`{"name":"..."}`) |
| `/me/auth-history` | GET | Kirish tarixi |
| `/me/stats` | GET | PDF konversiya statistikasi |

**Hammasi JWT talab qiladi.**

Profil javobi:
```json
{
  "id": "uuid",
  "name": "Zarif",
  "email": "z@gmail.com",
  "status": "active",
  "role": "user",       // yoki "admin"
  "created_at": "2026-05-22T..."
}
```

---

## 8. Tavsiyalar va Edge case'lar

### 8.1 Polling

- **Interval:** 1.5-2 soniya
- **Timeout:** 60 soniya (ko'pchilik op 5-15s da tugaydi)
- **Yaxshiroq variant:** WebSocket `/ws` orqali real-time

### 8.2 Xato handling

| HTTP | Sabab | Frontend xatti-harakat |
|---|---|---|
| 400 | Validation | `Data.errors` map'idan har field uchun xato ko'rsat |
| 401 | Token yo'q yoki yaroqsiz | OAuth'ga yo'naltir |
| 401 + `sign_up_path` | Mehmon katta fayl yuklamoqchi | "Kirib oling" CTA |
| 403 | Ruxsat yo'q (admin endpoint'ga oddiy user) | "Sizda ruxsat yo'q" |
| 404 | Resurs topilmadi yoki ACL | Yo'q xabari |
| 422 | Yomon PDF (corrupt) | "Fayl buzilgan" |
| 429 | Rate limit / brute-force | "Birozdan keyin urinib ko'ring" |
| 500 + `request_id` | Server xatosi | Support'ga `request_id` yuborish |

### 8.3 Fayl auto-cleanup

Hamma yuklangan va yaratilgan fayl **5 daqiqada o'chiriladi**. Frontend:
- Natija tugashi bilan **darhol** yuklab olish tugmasini ko'rsating
- "Bu fayl 5 daqiqadan keyin o'chiriladi" ogohlantiruvi
- Cloud'ga avtomatik eksport opsiyasi (Dropbox/GDrive)

### 8.4 CORS

Backend Caddy orqali HTTPS, hamma standart CORS header'lar bor. Frontend istalgan domen'dan chaqirishi mumkin.

### 8.5 Field nomi inconsistency (diqqat!)

Eski API qoldig'i — har joyda bir xil emas:
- `inspect` ⇒ `file_id`
- `share` ⇒ `file_id`
- Boshqa ko'pchilik ⇒ `input_file_id`

Validation xatosida endi `input_file_id` (snake_case) qaytadi — frontend o'sha kalit bilan UI fielding'a bog'laydi.

---

## 9. Tezkor Misol — to'liq oqim

```js
// 1. Login (brauzer yangi tab)
window.location.href = "https://convertpdfgo.com/auth/google";
// → user OAuth → callback → token JSON qaytadi

// 2. Fayl yuklash
const fd = new FormData();
fd.append("file", pdfFile);
const upRes = await fetch("https://convertpdfgo.com/file/upload", {
  method: "POST",
  headers: { Authorization: `Bearer ${token}` },
  body: fd
});
const { Data: { id: fileId } } = await upRes.json();

// 3. Compress job yaratish
const jobRes = await fetch("https://convertpdfgo.com/api/pdf/compress", {
  method: "POST",
  headers: {
    Authorization: `Bearer ${token}`,
    "Content-Type": "application/json"
  },
  body: JSON.stringify({ input_file_id: fileId, compression: "medium" })
});
const { Data: { id: jobId } } = await jobRes.json();

// 4. WebSocket bilan natijani kutish (yoki polling)
const ws = new WebSocket(`wss://convertpdfgo.com/ws?token=${token}`);
ws.onmessage = ({ data }) => {
  const evt = JSON.parse(data);
  if (evt.job_id === jobId && evt.status === "done") {
    // 5. Natijani yuklab olish
    window.location.href =
      `https://convertpdfgo.com/file/${evt.output_file_id}/download`;
  }
};
```

---

## 10. Yordamchi linklar

- **API hujjat (interaktiv):** https://convertpdfgo.com/swagger/index.html
- **OpenAPI spec (codegen uchun):** https://convertpdfgo.com/swagger/doc.json
- **TypeScript types generatsiyasi:**
  ```bash
  npx openapi-typescript https://convertpdfgo.com/swagger/doc.json -o src/api.d.ts
  ```
- **React Query hook'lari:**
  ```bash
  npx orval --input https://convertpdfgo.com/swagger/doc.json
  ```

---

## 11. Tezkor savol-javob

**S: 10 varoqli PDF'ni hammasini aylantirsa bo'ladimi?**
J: Ha, `{"input_file_id":"uuid","angle":90}` — `pages` qoldirib qo'ying yoki `"all"`.

**S: Nechta rasm bir vaqtda PDF'ga aylantirish mumkin?**
J: 50 ta. JPG va PNG qabul qiladi.

**S: Nechta PDF birlashtirish mumkin?**
J: 2 dan 50 gacha. Tartib saqlanadi.

**S: Qaysi operatsiya eng tez?**
J: `inspect` (sync, 50-500ms). Boshqalar 1-10 soniya orasida.

**S: Qaysi operatsiya eng sekin?**
J: OCR oilasi va Office konversiyasi — 5-30 soniya, fayl hajmiga qarab.

**S: Job natija qancha vaqt saqlanadi?**
J: 5 daqiqa. Bu vaqt ichida yuklab olish yoki cloud'ga eksport qilish kerak.

**S: Bir user nechta job parallel yuborishi mumkin?**
J: Backend cheklamaydi, lekin server resurs limit'i bor. Frontend tomondan 3-5 ta parallel job tavsiya etiladi.

**S: WebSocket disconnect bo'lsa nima qilish?**
J: Polling fallback — har 2 soniyada `GET /api/pdf/<op>/<job_id>`.

**S: Refresh token muddati qancha?**
J: 30 kun. Access token 1 soat.

**S: Bitta foydalanuvchi nechta cloud'ga ulanishi mumkin?**
J: Hamma 3 ta (Dropbox + GDrive + OneDrive) bir vaqtning o'zida.

---

> Yangi savol yoki bug bo'lsa, `request_id` (500 javobida bor) bilan murojaat qiling.
