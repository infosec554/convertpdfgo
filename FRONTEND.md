# convertpdfgo — Frontend uchun to'liq qo'llanma

> Mijoz/frontend dasturchi uchun **har bir endpoint** uchun: request shape, validation, javob, curl + JS misol va edge case'lar.

**Base URL:** `https://convertpdfgo.com`
**Swagger UI:** https://convertpdfgo.com/swagger/index.html
**OpenAPI JSON:** https://convertpdfgo.com/swagger/doc.json

---

## Mundarija

1. [Asosiy tushunchalar](#1-asosiy-tushunchalar)
2. [Autentifikatsiya (OAuth)](#2-autentifikatsiya)
3. [Profil — /me/*](#3-profil)
4. [Fayl boshqaruvi](#4-fayl-boshqaruvi)
5. [Cloud Export — Save to Dropbox/GDrive](#5-cloud-export)
6. [PDF operatsiyalar (35 ta)](#6-pdf-operatsiyalar)
7. [WebSocket — real-time](#7-websocket)
8. [Public / Infra endpoint'lar](#8-public--infra)
9. [Aloqa (contact form)](#9-aloqa-formasi)
10. [End-to-End to'liq misol](#10-end-to-end-misol)
11. [FAQ](#11-faq)

---

## 1. Asosiy tushunchalar

### 1.1 Foydalanuvchi turlari

| Tur | Yuklash limiti | Auth kerakmi |
|---|---|---|
| **Mehmon** (guest) | 30 MB | Yo'q |
| **Ro'yxatdan o'tgan** | 50 MB | Ha (JWT Bearer) |

30 MB+ fayl mehmonda → HTTP **401**:
```json
{
  "StatusCode": 401,
  "Description": "Unauthorized",
  "Data": {
    "limit_bytes": 31457280,
    "file_size": 34567890,
    "sign_in_paths": ["/auth/google", "/auth/github"]
  }
}
```

Frontend bu javobni ko'rsa, "Katta fayl uchun Google yoki GitHub orqali kiring" tugmasini ko'rsatishi kerak. 50 MB+ esa hatto auth bilan ham `400 "File exceeds your 50 MB account limit"`.

### 1.2 Javob format

Hamma API javobi:
```json
{
  "StatusCode": <int>,
  "Description": "OK" | "Bad Request" | "Unauthorized" | "Internal Server Error",
  "Data": <object | array | string | null>
}
```

### 1.3 Validation xatolari — frontend uchun tayyor

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

JSON key = JSON tag (`input_file_id`, snake_case). Frontend bevosita form input'iga bog'lashi mumkin.

### 1.4 5 daqiqa auto-cleanup

Hamma fayllar (input va output) **5 daqiqada o'chiriladi**. Frontend:
- Natija tayyor bo'lishi bilan darhol download tugmasini ko'rsating
- "5 daqiqada o'chiriladi" ogohlantiruvi qo'shing
- Yoki cloud'ga avtomatik eksport (Dropbox/GDrive)

### 1.5 HTTP status → harakat jadvali

| HTTP | Sabab | Frontend nima qiladi |
|---|---|---|
| 200 | OK | Natijani ishlat |
| 201 | Yaratildi | Yangi resurs id'sini saqla |
| 202 | Job navbatga qo'yildi | Polling boshla yoki WS kutib tur |
| 307 / 302 | OAuth redirect | `window.location` ga yo'naltir |
| 400 | Validation | `Data.errors` map'idan field xatolarni ko'rsat |
| 401 | Token yo'q/yaroqsiz | OAuth'ga yo'naltir |
| 401 + `sign_in_paths` | Guest 30 MB dan katta fayl | "Google/GitHub bilan kiring" CTA |
| 403 | Ruxsat yo'q | "Sizda ruxsat yo'q" |
| 404 | Topilmadi yoki ACL | "Mavjud emas" |
| 422 | Yomon PDF (corrupt) | "Fayl buzuq" |
| 429 | Rate-limit | "Birozdan keyin" |
| 500 + `request_id` | Server xato | Support'ga `request_id` |

---

## 2. Autentifikatsiya

**FAQAT OAuth.** Email+parol yo'q.

### 2.1 `GET /auth/google` — Google bilan kirish

**Auth:** Yo'q
**Format:** Brauzer redirect

```
window.location.href = "https://convertpdfgo.com/auth/google";
```

Backend → 307 Google'ga → user tasdiqlaydi → callback → access/refresh token JSON.

### 2.2 `GET /auth/github` — GitHub bilan kirish

Bir xil pattern:
```
window.location.href = "https://convertpdfgo.com/auth/github";
```

### 2.3 Callback javoblari

`/auth/google/callback` va `/auth/github/callback` HTTP 200 + JSON qaytaradi:

```json
{
  "StatusCode": 200,
  "Description": "OK",
  "Data": {
    "access_token":  "eyJhbGciOiJI...",
    "refresh_token": "eyJhbGciOiJI..."
  }
}
```

**Token muddati:**
- access: 1 soat
- refresh: 30 kun

### 2.4 `POST /auth/refresh` — tokenni yangilash

**Auth:** Yo'q (refresh token body'da)

**Request:**
```json
{"refresh_token": "eyJ..."}
```

**curl:**
```bash
curl -X POST https://convertpdfgo.com/auth/refresh \
  -H 'Content-Type: application/json' \
  -d '{"refresh_token":"eyJ..."}'
```

**Javob (200):**
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

Eski refresh token revoke bo'ladi (rotation). Yangi juftlik ishlatiladi.

**Xato (401):** "invalid refresh token" yoki "refresh token revoked".

### 2.5 `POST /auth/logout` — chiqish

**Auth:** ✅ Bearer kerak

**curl:**
```bash
curl -X POST https://convertpdfgo.com/auth/logout \
  -H "Authorization: Bearer $ACCESS"
```

**Javob (200):**
```json
{"StatusCode":200,"Description":"OK","Data":null}
```

Token darhol revoke bo'ladi — keyingi har qanday so'rov 401 `token revoked` qaytaradi.

### 2.6 `GET /auth/invite/:token` — admin invite preview

**Auth:** Yo'q (token o'zi himoya)

**Javob (200):**
```json
{
  "Data": {
    "email": "ad@convertpdfgo.com",
    "role": "admin"
  }
}
```
**Xato (404):** "invite expired or invalid"

### 2.7 `POST /auth/accept-invite` — invite qabul qilish

**Auth:** Yo'q

**Request:**
```json
{
  "token": "<invite_token>",
  "password": "minimal_8_chars"
}
```

**Javob (200):** tokens qaytadi (huddi login).

---

## 3. Profil

Hammasi **JWT talab qiladi**. Group: `/me/*`.

### 3.1 `GET /me` — o'z profilim

**curl:**
```bash
curl https://convertpdfgo.com/me -H "Authorization: Bearer $T"
```

**Javob (200):**
```json
{
  "StatusCode": 200,
  "Data": {
    "id": "fbd15a33-4daa-42d2-a949-367e166313aa",
    "name": "Zarif",
    "email": "zarif@gmail.com",
    "status": "active",
    "role": "user",
    "created_at": "2026-05-22T21:18:03.698337Z",
    "created_by": ""
  }
}
```

### 3.2 `PUT /me` — ismni o'zgartirish

**Request:**
```json
{"name": "Yangi Ism"}
```

**Validation:** `name` required, min 2 chars.

**Javob (200):**
```json
{"Data": {"id": "...", "name": "Yangi Ism"}}
```

**Xato (400):**
```json
{"Data": {"errors": {"name": "Name is required"}}}
```

### 3.3 `GET /me/auth-history` — login tarixi

**Javob (200):**
```json
{
  "Data": [
    {
      "id": "...",
      "user_id": "...",
      "email": "z@gmail.com",
      "event": "oauth_login",       // yoki "logout"
      "provider": "google",
      "ip": "188.113.197.69",
      "user_agent": "Mozilla/5.0 ...",
      "created_at": "2026-05-22T..."
    }
  ]
}
```

### 3.4 `GET /me/stats` — PDF konversiya statistikam

**Javob (200):**
```json
{
  "Data": {
    "merged": 0,
    "splitted": 0,
    "removed_pages": 0,
    "compressed": 5,
    "extracted": 0,
    "organized": 0,
    "jpg_to_pdf": 0,
    "pdf_to_jpg": 12,
    "pdf_to_word": 0,
    "rotated": 2,
    "cropped": 0,
    "added_page_numbers": 0,
    "unlocked": 0,
    "protected": 0,
    "watermarked": 0,
    "total_files": 19,
    "used_storage_mb": 0
  }
}
```

---

## 4. Fayl boshqaruvi

### 4.1 `POST /file/upload` — fayl yuklash

**Auth:** Optional. Bor bo'lsa fayl user'ga biriktiriladi (50 MB limit). Yo'q bo'lsa guest (30 MB limit).

**Format:** `multipart/form-data`, field nomi `file`.

**curl:**
```bash
curl -X POST https://convertpdfgo.com/file/upload \
  -H "Authorization: Bearer $T" \
  -F "file=@/path/to/file.pdf"
```

**JS:**
```js
const fd = new FormData();
fd.append("file", fileBlob);  // <input type="file">.files[0]

const res = await fetch("https://convertpdfgo.com/file/upload", {
  method: "POST",
  headers: token ? { Authorization: `Bearer ${token}` } : {},
  body: fd
});
const { Data: { id } } = await res.json();
```

**Yangi yuklash javobi (201):**
```json
{"StatusCode":201,"Data":{"id":"6c251904-0f93-4d6e-90ac-d13f2ebd64aa"}}
```

**Dedup javobi (200):** Bir xil content yuklasangiz, eski id qaytadi:
```json
{"StatusCode":200,"Data":{"id":"6c251904-...","dedup":true}}
```

> **Dedup user-scoped.** Mehmon va auth user ALOHIDA — bir xil content yuklashsa, alohida row'larga ega. Privacy himoyalangan.

**Xatolar:**
- 400 `"file is required"` — multipart field yo'q
- 400 `"request Content-Type isn't multipart/form-data"` — Content-Type noto'g'ri
- 401 + `sign_in_paths` — guest 30 MB dan oshib ketdi (OAuth'ga yo'naltir)
- 400 `"File exceeds your 50 MB account limit"` — auth user 50 MB dan oshib ketdi
- 500 — diskdagi yozish xatosi

### 4.2 `GET /file/:id` — metadata olish

**Auth:** Optional. Guest fayl bo'lsa hamma ko'radi. Owned fayl bo'lsa faqat owner yoki admin.

**curl:**
```bash
curl https://convertpdfgo.com/file/<id> -H "Authorization: Bearer $T"
```

**Javob (200):**
```json
{
  "StatusCode": 200,
  "Data": {
    "id": "6c251904-...",
    "user_id": "fbd15a33-..." | null,
    "file_name": "test.pdf",
    "file_path": "uploads/785e71ad-....pdf",
    "file_type": ".pdf",
    "file_size": 13264,
    "sha256": "5163068...",
    "uploaded_at": "2026-05-23T..."
  }
}
```

**Xato (404):** "file not found" — fayl yo'q yoki sizniki emas.

### 4.3 `GET /file/:id/download` — binary content

**Auth:** Optional (xuddi /file/:id kabi ACL).

**Headers (javob):**
```
Content-Disposition: attachment; filename="test.pdf"
Content-Type: .pdf
ETag: "<sha256>"
Cache-Control: private, max-age=3600, must-revalidate
Accept-Ranges: bytes
```

**JS — yangi tab'da ochish:**
```js
window.location.href = `https://convertpdfgo.com/file/${id}/download`;
```

**JS — blob qilib olish:**
```js
const res = await fetch(`https://convertpdfgo.com/file/${id}/download`, {
  headers: { Authorization: `Bearer ${token}` }
});
const blob = await res.blob();
const url = URL.createObjectURL(blob);
window.open(url);
```

**Range request** (qisman yuklash uchun):
```bash
curl https://convertpdfgo.com/file/<id>/download \
  -H "Range: bytes=0-1023"
```
→ HTTP 206 Partial Content.

**Xato (404):**
- "file not found" — id yo'q
- "the file was already cleaned up" — 5 daqiqa o'tdi

### 4.4 `DELETE /file/:id` — fayl o'chirish

**Auth:** Optional (ACL).

**curl:**
```bash
curl -X DELETE https://convertpdfgo.com/file/<id> \
  -H "Authorization: Bearer $T"
```

**Javob (200):**
```json
{"StatusCode":200,"Data":{"id":"<id>"}}
```

Diskdan va DB'dan o'chadi.

### 4.5 `GET /file/list` — mening fayllarim

**Auth:** ✅ Required.

**curl:**
```bash
curl https://convertpdfgo.com/file/list -H "Authorization: Bearer $T"
```

**Javob (200):**
```json
{
  "StatusCode": 200,
  "Data": [
    {
      "id": "...",
      "file_name": "...",
      "file_size": 13264,
      "file_type": ".pdf",
      "uploaded_at": "..."
    }
  ]
}
```

Bo'sh ro'yxat → `"Data": null`.

---

## 5. Cloud Export

### 5.1 `GET /api/storage/providers` — provider'lar ro'yxati

**Auth:** Optional.

**Javob (200):**
```json
{
  "Data": [
    {"name":"dropbox","display_name":"Dropbox","configured":true,"connected":false},
    {"name":"gdrive", "display_name":"Google Drive","configured":true,"connected":true},
    {"name":"onedrive","display_name":"OneDrive","configured":false,"connected":false}
  ]
}
```

| Field | Ma'no |
|---|---|
| `configured: true` | Backend bu provider uchun client ID/secret bilan sozlangan |
| `configured: false` | Server'da yoqilmagan — UI'da yashir |
| `connected: true` | Hozirgi user shu cloud akkauntiga ulangan |
| `connected: false` | Ulanmagan — "Connect" tugmasini ko'rsat |

### 5.2 `GET /api/storage/:provider/connect` — cloud OAuth boshlash

**Auth:** ✅ Required.

**Provider'lar:** `dropbox`, `gdrive`, `onedrive`.

**Javob (302):** brauzer Dropbox/Google sahifasiga yo'naltiriladi.

**JS:**
```js
window.location.href = `https://convertpdfgo.com/api/storage/dropbox/connect`;
```

Backend ichida JWT'ni state cookie'ga saqlaydi, callback'da o'qiydi.

### 5.3 `GET /oauth/:provider/callback` — cloud OAuth qaytishi

**Avtomatik.** Provider tomondan chaqiriladi. Backend token saqlaydi va user'ni `/me` yoki `/files` ga qaytaradi.

### 5.4 `POST /file/:id/export` — faylni cloud'ga eksport

**Auth:** ✅ Required.

**Request:**
```json
{
  "provider": "dropbox",          // yoki "gdrive"
  "path": "/convertpdfgo/result.pdf"   // ixtiyoriy, default root
}
```

**Javob (200):**
```json
{
  "Data": {
    "provider": "dropbox",
    "path": "/convertpdfgo/result.pdf",
    "size": 13264
  }
}
```

**Xato (403):** "not connected to dropbox" → avval `/api/storage/dropbox/connect` qiling.

### 5.5 `DELETE /api/storage/:provider` — cloud aloqani uzish

**Auth:** ✅ Required.

**curl:**
```bash
curl -X DELETE https://convertpdfgo.com/api/storage/dropbox \
  -H "Authorization: Bearer $T"
```

**Javob (200):** `{"Data": {"provider":"dropbox","disconnected":true}}`

---

## 6. PDF Operatsiyalar

**Hammasi async** (faqat `/inspect` sync). Pattern bir xil:

1. `POST /api/pdf/<op>` → 202 + `{id: jobId}`
2. `GET /api/pdf/<op>/<jobId>` poll yoki WS event kut
3. Status `done` bo'lganda `output_file_id` ni yuklab ol

### 6.1 Job hayot sikli

```
pending → processing → done       (success)
                    └→ failed     (worker xatosi)
```

**Pollig misol:**
```js
async function waitForJob(op, jobId, token) {
  while (true) {
    const r = await fetch(`https://convertpdfgo.com/api/pdf/${op}/${jobId}`, {
      headers: { Authorization: `Bearer ${token}` }
    });
    const { Data } = await r.json();
    if (Data.status === "done") return Data;
    if (Data.status === "failed") throw new Error(Data.error || "job failed");
    await new Promise(r => setTimeout(r, 1500));
  }
}
```

---

### 6.2 Sahifa operatsiyalari (5 ta)

#### **`/api/pdf/merge`** — bir nechta PDF birlashtirish

| Field | Type | Required | Limit |
|---|---|---|---|
| `input_file_ids` | uuid[] | ✅ | 2-50 PDF |

**Request:**
```json
{"input_file_ids": ["uuid1","uuid2","uuid3"]}
```

**curl:**
```bash
curl -X POST https://convertpdfgo.com/api/pdf/merge \
  -H "Authorization: Bearer $T" \
  -H "Content-Type: application/json" \
  -d '{"input_file_ids":["id1","id2"]}'
```

**202:** `{"Data":{"id":"job-uuid","status":"pending"}}`

**Done javobi:** `{"Data":{"id":"...","status":"done","output_file_id":"merged-uuid",...}}`

---

#### **`/api/pdf/split`** — PDF'ni bo'lish

| Field | Type | Required | Format |
|---|---|---|---|
| `input_file_id` | uuid | ✅ | |
| `split_ranges` | string | ✅ | `"1-3,4-5"` yoki `"1,3,5"` |

**Output:** `output_file_ids: [uuid, ...]` — har bir bo'lak alohida fayl.

**Misol — 10 sahifani 3 ta faylga:**
```json
{"input_file_id":"uuid","split_ranges":"1-3,4-7,8-10"}
```

---

#### **`/api/pdf/removepage`** — sahifa o'chirish

```json
{"input_file_id":"uuid","pages_to_remove":"1,4-6"}
```

**Output:** `output_file_id` (yangi PDF).

---

#### **`/api/pdf/extract`** — sahifa ajratib olish

⚠️ Field nomi `page_ranges` (removepage'da `pages_to_remove` edi).

```json
{"input_file_id":"uuid","page_ranges":"1-3,5,7-9"}
```

---

#### **`/api/pdf/compress`** — siqish

| Field | Required | Qiymatlar |
|---|---|---|
| `input_file_id` | ✅ | |
| `compression` | ✅ | `low` / `medium` / `high` |

| Daraja | Backend | Natija |
|---|---|---|
| `low` | Ghostscript `/screen` | ~30-50% kichik, eng yuqori sifat |
| `medium` | `/ebook` | ~50-70%, normal (tavsiya) |
| `high` | `/printer` | ~70%+, past sifat |

---

### 6.3 Vizual o'zgartirishlar (6 ta)

#### **`/api/pdf/rotate`** — aylantirish

| Field | Type | Required | Izoh |
|---|---|---|---|
| `input_file_id` | uuid | ✅ | |
| `angle` | int | ✅ | 90 / 180 / 270 |
| `pages` | string | ❌ | `"all"` (default) yoki `"1-3,5"` |

**Hamma sahifa:**
```json
{"input_file_id":"uuid","angle":90}
```
(yoki `"pages":"all"`)

**Faqat ba'zi sahifalar:**
```json
{"input_file_id":"uuid","angle":180,"pages":"3,7"}
```

---

#### **`/api/pdf/crop`** — kesish

| Field | Type | Required |
|---|---|---|
| `input_file_id` | uuid | ✅ |
| `top` / `bottom` / `left` / `right` | int | ❌ (pikselda) |
| `pages` | string | ❌ |
| `box` | string | ❌ (`mediabox`/`cropbox`) |

---

#### **`/api/pdf/add-page-numbers`**

| Field | Type | Required | Izoh |
|---|---|---|---|
| `input_file_id` | uuid | ✅ | |
| `first_number` | int ≥1 | ✅ | |
| `page_range` | string | ✅ | `"all"` yoki `"1-5"` |
| `position` | string | ✅ | `bottom-right` va h.k. |
| `color` | string | ✅ | `"black"`, hex |
| `font_size` | int ≥1 | ✅ | px (10-14) |

**Misol:**
```json
{
  "input_file_id":"uuid",
  "first_number":1,
  "page_range":"all",
  "position":"bottom-right",
  "color":"black",
  "font_size":12
}
```

---

#### **`/api/pdf/header-footer`**

| Field | Required | Default |
|---|---|---|
| `input_file_id` | ✅ | |
| `header_text` | ❌ | "" |
| `footer_text` | ❌ | "" |
| `font_size` | ❌ | 12 |
| `font_color` | ❌ | "black" |
| `position` | ❌ | "center" |

---

#### **`/api/pdf/qr-code`** — QR kod qo'shish

| Field | Required |
|---|---|
| `input_file_id` | ✅ |
| `qr_content` | ✅ |
| `position` | ✅ |
| `size` | ✅ (px) |

```json
{
  "input_file_id":"uuid",
  "qr_content":"https://example.com",
  "position":"top-right",
  "size":80
}
```

---

#### **`/api/pdf/watermark`** — vodyanoy znak

| Field | Type | Izoh |
|---|---|---|
| `input_file_id` | uuid | ✅ |
| `mode` | string | ✅ `text` yoki `image` |
| `text` | string | mode=text uchun |
| `image_file_id` | uuid | mode=image uchun (oldin upload qiling) |
| `position` | string | pdfcpu: `c`/`tl`/`tr`/`bl`/`br` |
| `rotation` | int | -90 dan 90 |
| `opacity` | float | 0.0-1.0 (default 0.4) |
| `pages` | string | `"all"` yoki range |

**Text watermark:**
```json
{
  "input_file_id":"uuid",
  "mode":"text",
  "text":"DRAFT",
  "position":"c",
  "rotation":-45,
  "opacity":0.3
}
```

**Image watermark:**
```json
{
  "input_file_id":"uuid",
  "mode":"image",
  "image_file_id":"<oldin yuklangan logo PNG id>",
  "position":"br",
  "opacity":0.5
}
```

---

### 6.4 Xavfsizlik (2 ta)

#### **`/api/pdf/protect`** — parol qo'yish

```json
{"input_file_id":"uuid","password":"secret123"}
```

**Output:** himoyalangan PDF.

#### **`/api/pdf/unlock`** — parolni olib tashlash

```json
{"input_file_id":"uuid","password":"secret123"}
```

Agar PDF haqiqatda himoyalangan bo'lmasa → `failed` "not encrypted".

---

### 6.5 Tahlil (2 ta)

#### **`/api/pdf/inspect`** ⚡ SYNC

⚠️ Yagona sync operatsiya. POST darhol 200 qaytaradi.

**Request:** ⚠️ Field nomi `file_id` (boshqa op'larda `input_file_id`).
```json
{"file_id":"uuid"}
```

**Javob (200):**
```json
{
  "Data": {
    "id": "...",
    "file_id": "...",
    "page_count": 12,
    "title": "...",
    "author": "...",
    "subject": "...",
    "keywords": "...",
    "status": "done"
  }
}
```

**Yomon PDF (422):**
```json
{"StatusCode":422,"Data":"invalid or malformed PDF: ..."}
```

#### **`/api/pdf/detect-blank`** — bo'sh sahifalarni topish

```json
{"input_file_id":"uuid"}
```

**Output:** `{"blank_pages":[3,7,12]}`

---

### 6.6 Rasm ↔ PDF (2 ta)

#### **`/api/pdf/jpg-to-pdf`** — rasmlardan PDF

| Field | Type | Limit |
|---|---|---|
| `input_file_ids` | uuid[] | 1-50 (JPG/PNG) |

**Output:** yagona PDF (har rasm = 1 sahifa) + ixtiyoriy ZIP arxiv.

#### **`/api/pdf/pdf-to-jpg`**

```json
{"input_file_id":"uuid"}
```

**Output:**
```json
{
  "output_file_ids": ["jpg1","jpg2","..."],
  "zip_file_id": "zip-uuid"
}
```

10 sahifali PDF → 10 ta JPG + 1 ta ZIP.

---

### 6.7 Office → PDF (5 ta)

Hammasi bir xil shape (Gotenberg / LibreOffice ishlatadi):

| Operatsiya | Qabul qiladi |
|---|---|
| `/api/pdf/word-to-pdf` | `.doc`, `.docx` |
| `/api/pdf/excel-to-pdf` | `.xls`, `.xlsx` |
| `/api/pdf/ppt-to-pdf` | `.ppt`, `.pptx` |

**Request:**
```json
{"input_file_id":"uuid"}
```

#### **`/api/pdf/html-to-pdf`** — HTML matn

```json
{"html_content":"<html><body><h1>Salom</h1></body></html>"}
```
CSS to'liq qo'llab-quvvatlanadi.

#### **`/api/pdf/url-to-pdf`** — URL'dan PDF

```json
{
  "url": "https://example.com",
  "page_size": "A4",         // ixtiyoriy: A3, A4, A5, B4, B5
  "landscape": false,        // ixtiyoriy
  "wait_for": "2s"           // ixtiyoriy: render kutish
}
```

---

### 6.8 PDF → Office (4 ta)

Hammasi bir xil shape:
```json
{"input_file_id":"uuid"}
```

| Operatsiya | Output |
|---|---|
| `/api/pdf/pdf-to-word` | `.docx` |
| `/api/pdf/pdf-to-ppt` | `.pptx` (1 page = 1 slayd) |
| `/api/pdf/pdf-to-excel` | `.xlsx` (layout-aware) |
| `/api/pdf/pdf-to-html` | `.html` |

---

### 6.9 OCR oilasi (6 ta)

**Tesseract til kodlari:**

| Kod | Til |
|---|---|
| `eng` | Inglizcha (default) |
| `rus` | Ruscha |
| `uzb` | O'zbekcha (lotin) |
| `uzb_cyrl` | O'zbekcha (kirill) |
| `kir` | Qirg'izcha |
| `eng+rus` | Bir nechta (`+` bilan) |

#### **`/api/pdf/ocr`** — searchable PDF yaratish

```json
{
  "input_file_id":"uuid",
  "lang":"eng+rus",
  "force_ocr": false
}
```
**Output:** matnli qatlam qo'shilgan PDF.

#### **`/api/pdf/scan-to-pdf`** — skanlardan OCR PDF

```json
{
  "input_file_ids": ["jpg1","jpg2"],   // max 50
  "lang": "uzb"
}
```

⚠️ Juda kichik rasm (<3px) → `failed`.

#### **`/api/pdf/ocr-extract-text`** — .txt fayl

```json
{"input_file_id":"uuid","lang":"eng"}
```

#### **`/api/pdf/image-to-text`** — bitta rasmdan .txt

```json
{"input_file_id":"uuid","lang":"eng"}
```

#### **`/api/pdf/ocr-detect-lang`** — tilni aniqlash

```json
{"input_file_id":"uuid"}
```
**Output:** `{"detected_lang":"eng","confidence":0.97}`

⚠️ Juda kam matnli PDF → `failed` "Too few characters".

#### **`/api/pdf/ocr-structured`** — OCR + tuzilma

```json
{"input_file_id":"uuid","lang":"eng"}
```
**Output:** strukturalashgan format (HOCR/JSON).

---

### 6.10 Boshqa (4 ta)

#### **`/api/pdf/extract-text`** — OCR'siz matn

```json
{"input_file_id":"uuid","format":"txt"}   // yoki "json"
```

#### **`/api/pdf/nup`** — bir varaqqa N sahifa

| Field | Required | Qiymatlar |
|---|---|---|
| `input_file_id` | ✅ | |
| `n` | ✅ | 2, 4, 6, 9 |
| `orientation` | ❌ | `portrait` / `landscape` |
| `border` | ❌ | bool (default false) |

**Misol:** 10 sahifali PDF + n=4 → 3 varaq (4+4+2).

#### **`/api/pdf/metadata`** — metadata o'zgartirish

```json
{
  "input_file_id":"uuid",
  "title":"Yangi nom",
  "author":"Men",
  "subject":"...",
  "keywords":["pdf","convert"],
  "creator":"convertpdfgo"
}
```

⚠️ Update mode. Bo'sh field bersangiz `failed` "no metadata fields provided". O'qish uchun `/api/pdf/inspect`.

#### **`/api/pdf/share`** — public link yaratish

⚠️ Field `file_id` (boshqalarda `input_file_id`).

```json
{"file_id":"uuid","ttl_minutes":60}
```

**Output (201):**
```json
{"Data":{"token":"abc123..."}}
```

**Public download:**
```
https://convertpdfgo.com/api/pdf/share/<token>
```

Default TTL: 24 soat. Token expire → 404.

---

## 7. WebSocket

### 7.1 `/ws` — user WebSocket

**Auth:** `?token=<access_token>` query param (brauzer WS'da Authorization header qo'llamaydi).

**Misol:**
```js
const ws = new WebSocket(`wss://convertpdfgo.com/ws?token=${accessToken}`);

ws.onopen = () => console.log("WS ulandi");
ws.onmessage = ({ data }) => {
  const evt = JSON.parse(data);
  if (evt.type === "job.update") {
    console.log(`Job ${evt.job_id}: ${evt.status}`);
    if (evt.status === "done") {
      downloadFile(evt.output_file_id);
    }
  }
};
ws.onclose = () => console.log("WS uzildi — polling'ga o'tish");
ws.onerror = (e) => console.error(e);
```

**Event format:**
```json
{
  "type": "job.update",
  "job_id": "...",
  "op": "compress",
  "status": "done",          // pending | processing | done | failed
  "output_file_id": "..."    // status=done bo'lsa
}
```

### 7.2 `/ws/admin` — admin (admin role kerak)

Non-admin token → 403. Admin event'lar (yangi user, contact, suspicious login) shu yerga keladi.

---

## 8. Public / Infra

### 8.1 `GET /healthz`

**Auth:** Yo'q.

**Javob (200):**
```json
{"status":"ok"}
```

Load balancer / monitoring uchun.

### 8.2 `GET /readyz`

**Auth:** Yo'q.

**Javob (200):**
```json
{"status":"ready"}
```

DB ulanish ishlasa "ready" qaytadi. Aks holda 503.

### 8.3 `GET /metrics`

**Auth:** Yo'q.

Prometheus format. Frontend uchun ahamiyatsiz.

### 8.4 `GET /swagger/index.html`

Interaktiv API hujjat. Frontend dasturchi shu yerda har endpoint'ni "Try it out" qila oladi.

### 8.5 `GET /swagger/doc.json`

OpenAPI 2.0 spec. Codegen uchun:
```bash
npx openapi-typescript https://convertpdfgo.com/swagger/doc.json -o api.d.ts
```

### 8.6 `GET /api/stats/public`

**Auth:** Yo'q. Sayt landing'iga "Bizning xizmat shuncha foydalanildi" ko'rsatish uchun.

**Javob (200):**
```json
{
  "Data": {
    "total_users": 5,
    "total_files_uploaded": 0,
    "total_files_processed": 0,
    "total_jobs": 0
  }
}
```

---

## 9. Aloqa formasi

### 9.1 `POST /api/contact`

**Auth:** Yo'q (mehmon ham yubora oladi).

**Request:**
```json
{
  "name": "Zarif",
  "email": "z@gmail.com",
  "subject": "Savol",            // ixtiyoriy
  "body": "Salom, savol bor..."  // min 5, max 8000
}
```

**Validation:**
- `name`: min 2, max 255
- `email`: valid email
- `subject`: max 255 (ixtiyoriy)
- `body`: min 5, max 8000

**Javob (201):**
```json
{"Data":{"id":"contact-uuid"}}
```

Admin keyin email orqali javob beradi (Brevo orqali).

---

## 10. End-to-End to'liq misol

PDF'ni Google bilan login bo'lib siqish va yuklab olish:

```js
// ============================================================
// 1) LOGIN — yangi tab Google sahifasi
// ============================================================
window.location.href = "https://convertpdfgo.com/auth/google";
// → user OAuth → callback → JSON token
const { access_token, refresh_token } = await fetchCallbackResult();

// ============================================================
// 2) FAYL YUKLASH
// ============================================================
const fd = new FormData();
fd.append("file", pdfFile);   // <input type="file">
const upRes = await fetch("https://convertpdfgo.com/file/upload", {
  method: "POST",
  headers: { Authorization: `Bearer ${access_token}` },
  body: fd
});
const { Data: { id: fileId } } = await upRes.json();
console.log("File uploaded:", fileId);

// ============================================================
// 3) JOB YARATISH — compress
// ============================================================
const jobRes = await fetch("https://convertpdfgo.com/api/pdf/compress", {
  method: "POST",
  headers: {
    Authorization: `Bearer ${access_token}`,
    "Content-Type": "application/json"
  },
  body: JSON.stringify({
    input_file_id: fileId,
    compression: "medium"
  })
});
const { Data: { id: jobId } } = await jobRes.json();
console.log("Job queued:", jobId);

// ============================================================
// 4) NATIJANI KUTISH — WebSocket
// ============================================================
const ws = new WebSocket(`wss://convertpdfgo.com/ws?token=${access_token}`);
ws.onmessage = ({ data }) => {
  const evt = JSON.parse(data);
  if (evt.job_id === jobId) {
    if (evt.status === "done") {
      // ====================================================
      // 5) NATIJANI YUKLAB OLISH
      // ====================================================
      window.location.href =
        `https://convertpdfgo.com/file/${evt.output_file_id}/download`;
      ws.close();
    } else if (evt.status === "failed") {
      alert("Xato: " + evt.error);
      ws.close();
    }
  }
};

// ============================================================
// FALLBACK — WS ishlamasa polling
// ============================================================
ws.onerror = async () => {
  while (true) {
    const r = await fetch(
      `https://convertpdfgo.com/api/pdf/compress/${jobId}`,
      { headers: { Authorization: `Bearer ${access_token}` } }
    );
    const { Data } = await r.json();
    if (Data.status === "done") {
      window.location.href =
        `https://convertpdfgo.com/file/${Data.output_file_id}/download`;
      break;
    }
    if (Data.status === "failed") {
      alert("Xato: " + Data.error);
      break;
    }
    await new Promise(r => setTimeout(r, 1500));
  }
};
```

---

## 11. FAQ

**S: 10 varoqli PDF'ni hammasini aylantirsa bo'ladimi?**
J: Ha. `{"input_file_id":"uuid","angle":90}` — `pages` qoldirib qo'ying yoki `"all"`.

**S: Faqat 3 va 7-sahifani aylantirsa?**
J: `{"input_file_id":"uuid","angle":90,"pages":"3,7"}`

**S: Nechta rasmni bir vaqtda PDF'ga aylantirish mumkin?**
J: 50 ta. JPG va PNG qabul qiladi.

**S: Nechta PDF birlashtirish mumkin?**
J: 2-50. Tartib saqlanadi.

**S: Qaysi operatsiya eng tez?**
J: `/inspect` (sync, 50-500ms). Boshqalar 1-10s.

**S: Eng sekin?**
J: OCR + Office konversiyasi — 5-30s.

**S: Job natija qancha vaqt saqlanadi?**
J: 5 daqiqa. Darhol yuklab olish kerak.

**S: Bir user nechta job parallel yubora oladi?**
J: Backend cheklamaydi. Frontend tomondan 3-5 parallel tavsiya.

**S: WS uzilsa nima qilish?**
J: Polling fallback (yuqorida 10-bo'limda misol).

**S: Refresh token qancha?**
J: 30 kun. Access 1 soat.

**S: Mehmon sifatida qaysi operatsiyalar mumkin?**
J: HAMMASI — faqat 30 MB dan katta fayl yuklay olmaydi. 30-50 MB orasidagi fayl uchun OAuth kerak. 50 MB+ esa umuman qabul qilinmaydi.

**S: Dedup boshqa user'ning faylini ko'rsatadimi?**
J: Yo'q. User-scoped — har user o'z slice'i ichida dedup.

**S: PDF qaytarilgandan keyin uni Dropbox'ga avto yuborib bo'ladimi?**
J: Ha — `POST /file/<id>/export` bilan.

**S: Field nomi qaysi joyda `file_id` qaysi joyda `input_file_id`?**
J:
- `file_id`: `/api/pdf/inspect`, `/api/pdf/share`, `/file/<id>/...`
- `input_file_id`: boshqa hamma PDF op'lar
- Validation error JSON kalitda esa `input_file_id` (snake_case) qaytadi

**S: CORS sozlangan?**
J: Ha. Hamma domen'dan chaqirish mumkin.

**S: HTTPS majburiymi?**
J: Ha. `http://` so'rovlar HTTPS'ga 308 redirect bo'ladi.

**S: Bitta user nechta cloud'ga ulanishi mumkin?**
J: 3 ta (Dropbox + GDrive + OneDrive) bir vaqtda.

---

## 12. Status: Live va Ishlayotgan

✅ HTTPS: `https://convertpdfgo.com` (Let's Encrypt)
✅ OAuth: Google + GitHub
✅ 35 PDF op
✅ WebSocket real-time
✅ Cloud export (Dropbox + GDrive)
✅ Auto 5-min cleanup
✅ Snake_case validation errors
✅ User-scoped dedup (privacy)
✅ HTTP→HTTPS auto-redirect

---

> **Bag yoki yangi savol?** 500 javobida `request_id` bor — uni support'ga yuboring.
