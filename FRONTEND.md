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

## 12. Frontend implementation qo'llanmasi — har funksiya uchun

> Har bir endpoint uchun: qaysi UI komponent, default qiymat, validation, UX maslahatlar.

### 12.1 Umumiy patternlar (barcha sahifalarga)

#### A) Axios/fetch interceptor — token refresh

```js
axios.interceptors.response.use(
  res => res,
  async err => {
    if (err.response?.status === 401 && err.response?.data?.Data === null) {
      // Access token expired — refresh & retry
      const refresh = localStorage.getItem("refresh_token");
      const { data } = await axios.post("/auth/refresh", { refresh_token: refresh });
      localStorage.setItem("access_token", data.Data.access_token);
      localStorage.setItem("refresh_token", data.Data.refresh_token);
      err.config.headers.Authorization = `Bearer ${data.Data.access_token}`;
      return axios.request(err.config);
    }
    throw err;
  }
);
```

#### B) Error toast — `Data.errors` map

```js
function showApiError(res) {
  const data = res.data?.Data;
  if (data?.errors) {
    // Validation: highlight form fields
    for (const [field, msg] of Object.entries(data.errors)) {
      setFieldError(field, msg);
    }
  } else if (res.status === 401 && data?.sign_in_paths) {
    // Guest hit file size limit
    showSignInModal(data.sign_in_paths);
  } else if (res.status === 422) {
    toast.error("Fayl buzuq yoki PDF emas");
  } else if (res.status === 429) {
    toast.error("Juda ko'p so'rov yubordingiz, birozdan keyin");
  } else if (res.status >= 500 && data?.request_id) {
    toast.error(`Server xatosi. Yordam uchun: ${data.request_id}`);
  } else {
    toast.error("Noma'lum xato");
  }
}
```

#### C) Job polling helper

```js
async function pollJob(op, jobId, opts = {}) {
  const { interval = 1500, timeout = 60000, onProgress } = opts;
  const start = Date.now();
  while (Date.now() - start < timeout) {
    const { data } = await axios.get(`/api/pdf/${op}/${jobId}`);
    onProgress?.(data.Data.status);
    if (data.Data.status === "done") return data.Data;
    if (data.Data.status === "failed") throw new Error(data.Data.error || "Job failed");
    await new Promise(r => setTimeout(r, interval));
  }
  throw new Error("Timeout");
}
```

#### D) WebSocket bilan job tugashini kutish (polling o'rniga)

```js
function waitForJobWS(jobId, token) {
  return new Promise((resolve, reject) => {
    const ws = new WebSocket(`wss://convertpdfgo.com/ws?token=${token}`);
    const timeout = setTimeout(() => { ws.close(); reject(new Error("Timeout")); }, 60000);
    ws.onmessage = ({ data }) => {
      const e = JSON.parse(data);
      if (e.job_id !== jobId) return;
      if (e.status === "done") { clearTimeout(timeout); ws.close(); resolve(e); }
      if (e.status === "failed") { clearTimeout(timeout); ws.close(); reject(new Error(e.error)); }
    };
    ws.onerror = () => reject(new Error("WS error"));
  });
}
```

#### E) Drag-drop file uploader

```jsx
function FileDropzone({ onUpload, accept = ".pdf", multiple = false, maxSize = 30 * 1024 * 1024 }) {
  const [dragging, setDragging] = useState(false);
  return (
    <div
      onDragOver={e => { e.preventDefault(); setDragging(true); }}
      onDragLeave={() => setDragging(false)}
      onDrop={e => {
        e.preventDefault();
        setDragging(false);
        const files = Array.from(e.dataTransfer.files);
        // Client-side size check (skip server roundtrip)
        const oversized = files.find(f => f.size > maxSize);
        if (oversized) {
          showSignInModal(["/auth/google", "/auth/github"]);
          return;
        }
        onUpload(multiple ? files : files[0]);
      }}
      className={dragging ? "border-blue-500" : "border-gray-300"}
    >
      Faylni shu yerga tashlang yoki bosing
    </div>
  );
}
```

#### F) "5 daqiqada o'chiriladi" countdown

Natija fayl tayyor bo'lganda:
```jsx
function ExpiryCountdown({ uploadedAt }) {
  const [remaining, setRemaining] = useState(300);
  useEffect(() => {
    const id = setInterval(() => {
      const elapsed = (Date.now() - new Date(uploadedAt)) / 1000;
      setRemaining(Math.max(0, 300 - elapsed));
    }, 1000);
    return () => clearInterval(id);
  }, [uploadedAt]);
  if (remaining < 60) return <span className="text-red-500">⚠️ {Math.ceil(remaining)}s da o'chadi</span>;
  return <span>⏱ {Math.floor(remaining / 60)}m da o'chadi</span>;
}
```

---

### 12.2 Auth sahifa (login/register UI)

**UI:** Bitta sahifa, ikkita katta tugma:
```
┌─────────────────────────────┐
│   convertpdfgo               │
│                             │
│   [G] Google bilan kirish    │
│   [⌐] GitHub bilan kirish    │
│                             │
│   Tezda boshlash uchun:      │
│   - 30 MB gacha mehmon       │
│   - 50 MB gacha auth         │
└─────────────────────────────┘
```

**Tugma onClick:**
```js
window.location.href = "https://convertpdfgo.com/auth/google";
```

**Callback page** (`/auth/callback` yoki sayt root):
URL hash/query'dan token olib, localStorage'ga saqlang, asosiy sahifaga yo'naltir.

```js
// Callback handlerda:
const params = new URLSearchParams(window.location.search);
// Backend JSON qaytaradi (HTML emas). Token'larni darhol storage'ga:
const res = await fetch("/auth/google/callback" + window.location.search);
const { Data } = await res.json();
localStorage.setItem("access_token", Data.access_token);
localStorage.setItem("refresh_token", Data.refresh_token);
window.location.href = "/";
```

---

### 12.3 Profil sahifasi (`/me`)

**Layout:**
- Yuqorida: avatar + ism + email + provider belgisi
- Bo'limlar: Profil | Statistika | Login tarixi | Cloud aloqalar

**`GET /me` qachon chaqirish:**
- App load (token bor bo'lsa)
- Login'dan keyin
- "Mening sahifam" sahifasiga kirgan paytda

**`PUT /me` ism o'zgartirish:**
```jsx
<input type="text" value={name} minLength={2} required />
<button onClick={() => axios.put("/me", { name })}>Saqlash</button>
```

Validation: client-side `minLength=2`, server qaytaradi `{"errors":{"name":"..."}}` agar muammo bo'lsa.

**`GET /me/auth-history`** — jadval ko'rinishda:
```
| Sana            | Provider | IP             | Holat       |
| 22 May 22:18    | google   | 188.113.197.69 | login_ok    |
| 22 May 22:30    | -        | 188.113.197.69 | logout      |
```

Faqat oxirgi 20 ta yoki paginatsiya bilan.

**`GET /me/stats`** — dashboard widget:
```
┌─────────────────────────┐
│ Jami konvertatsiya: 19  │
│ ──────────────────────  │
│ 📦 Compressed:      5   │
│ 🖼  PDF→JPG:        12  │
│ 🔄 Rotated:         2   │
│ Disk: 0 MB              │
└─────────────────────────┘
```

---

### 12.4 Cloud Export — Settings sahifa

**UI** (`/api/storage/providers` natijasi asosida):
```
┌────────────────────────────────────────────┐
│ Bulutga saqlash                            │
│                                            │
│ 📦 Dropbox                                 │
│    ✓ Ulangan       [Disconnect]            │
│                                            │
│ ☁  Google Drive                            │
│    Ulanmagan       [Connect]               │
│                                            │
│ OneDrive — hozircha mavjud emas            │
└────────────────────────────────────────────┘
```

**Connect tugmasi:**
```js
window.open(`https://convertpdfgo.com/api/storage/${provider}/connect?token=${accessToken}`, "_blank");
```

Yangi tab'da OAuth flow → user ruxsat beradi → callback → tab yopiladi.

**Asosiy oqimga qo'shilgandan keyin** har bir job result oynasida:
```
[ Download ]  [ Save to Dropbox ]  [ Save to Google Drive ]
```

`Save to X` tugmasi:
```js
await axios.post(`/file/${outputFileId}/export`, { provider: "dropbox" });
toast.success("Dropbox'ga yuborildi");
```

---

### 12.5 Fayl operatsiyalari uchun UI patternlar

Hamma PDF op uchun **umumiy struktura**:

```
1. [ Drag-drop / file picker ]   ← input fayl tanlash
2. [ Operatsiyaga oid sozlamalar ] ← op-specific controls
3. [ Convert tugmasi ]
4. [ Progress bar yoki spinner ] ← polling/WS bilan
5. [ Natija: download + cloud export tugmalari ]
6. [ "5 daqiqada o'chadi" countdown ]
```

---

### 12.6 Har PDF operatsiya uchun aniq UI specs

#### 📑 merge — PDF birlashtirish

**UI:**
- Multi-file picker (`<input multiple accept=".pdf">`)
- Yuklangan fayllar ro'yxati + **drag-and-drop tartib o'zgartirish**
- Har faylda "X" tugmasi (o'chirish)
- Disable Submit agar < 2 fayl yoki > 50 fayl
- Ko'rsatish: "Jami: 5 PDF, 12 MB"

**Validation:**
- Min 2 fayl
- Max 50 fayl
- Har fayl ≤ 30/50 MB

**Tip:** Drag-drop bilan tartib o'zgartirish katta UX yutuq. `react-dnd` yoki `react-beautiful-dnd` ishlatish mumkin.

---

#### ✂️ split — PDF bo'lish

**UI:**
- Single file picker
- Avval `/api/pdf/inspect` chaqirib `page_count` olish → user ko'rishi mumkin
- Range input + visual page preview (ixtiyoriy)
- Ikkita rejim:
  - "Range bilan: 1-3, 5, 7-9" (text input)
  - "Har sahifani alohida fayl qil" (checkbox → backend'ga `"1,2,3,...,N"` yuborish)

**Validation:** range format `^[\d,\-\s]+$` (raqamlar, vergul, defis, bo'sh joy).

**Natija:** Bir nechta `output_file_ids`. Foydalanuvchiga:
- "Hammasini ZIP'da yuklab olish" tugmasi (frontend ZIP yasaydi)
- Yoki har birini alohida download

---

#### 🗑 removepage — sahifa o'chirish

**UI:**
- File picker
- Avval inspect → page_count olib visual page grid (thumbnail bilan ko'rinsa yana yaxshi)
- User sahifalarni klik bilan tanlaydi
- Tanlanganlar `pages_to_remove` ga aylanadi

**Tip:** Visual sahifa preview ko'rsatish uchun `pdf-to-jpg` bilan thumbnail yaratish mumkin. Yoki yengilroq — `pdf.js` bilan brauzerda render.

---

#### 📤 extract — kerakli sahifalarni ajratish

Xuddi `removepage` kabi — faqat user "qoldiriladigan"larni tanlaydi.

**UI farqi:** Tanlanganlar yashil (qoladi), tanlanmaganlar kulrang.

---

#### 📦 compress — siqish

**UI:**
- File picker
- Radio button:
  ```
  ( ) Low      ~30-50% kichik, eng yuqori sifat
  (•) Medium   ~50-70% kichik, normal sifat (tavsiya)
  ( ) High     ~70%+ kichik, past sifat (faqat skan uchun)
  ```
- "Convert" tugmasi
- Natijada: "Hajm: 5 MB → 1.8 MB (64% kichik)"

**Tip:** Default `medium`. Estimated savings ko'rsatish (statistik bo'lsin).

---

#### 🔄 rotate — aylantirish

**UI:**
- File picker → inspect → page_count
- Burchak tanlash:
  ```
  [ ↻ 90° ]  [ ↺ 270° ]  [ ⤴ 180° ]
  ```
- Sahifa range (default: hammasi):
  - Radio: ( ) Hamma sahifalar  ( ) Tanlangan: [input "1-3,5"]

**Default qiymatlar:**
- `angle: 90`
- `pages: "all"`

**UX:** "Visual preview" — sahifani aylantirib ko'rsatish (frontend tomondan `transform: rotate(90deg)`).

---

#### ✂️ crop — kesish

**UI:**
- File picker
- Visual crop tool — sahifani ko'rsatib, foydalanuvchi qirralarni sudraydi
- Yoki oddiy variant — 4 ta input: top, right, bottom, left (pikselda)
- Page range selector

**Tip:** "Auto-crop" tugmasi (oq joylarni avtomatik aniqlash) qo'shimcha funksiya bo'lishi mumkin.

---

#### 🔢 add-page-numbers

**UI:**
```
Birinchi raqam:    [ 1 ]
Sahifalar:         [ Hamma ▾ ] [ 1-10 ]
Pozitsiya:         [ Pastki o'ng ▾ ]
                   ( top-left | top-center | top-right
                   | bottom-left | bottom-center | bottom-right )
Rang:              [⬛ qora] [color picker]
Font o'lchami:     [ 12 ] px
```

**Default'lar:**
- first_number: 1
- page_range: "all"
- position: "bottom-right"
- color: "black"
- font_size: 12

**Tip:** Mini preview ko'rsatish — bitta sahifa ko'rinishida raqam joyini ko'rsatish.

---

#### 📋 header-footer

**UI:**
```
Yuqori matn:    [____________________]
Pastki matn:    [____________________]
Font o'lchami:  [ 12 ]
Rang:           [⬛ ▾]
Joylashuv:      ( ) Chap (•) O'rta ( ) O'ng
```

**Default'lar:**
- font_size: 12
- font_color: "black"
- position: "center"

Bo'sh field'lar — server tomondan default ishlatiladi.

---

#### 🔲 qr-code — QR kod qo'shish

**UI:**
```
QR kontent:     [____________________]
                (URL, matn, telefon)
Pozitsiya:      [Yuqori o'ng ▾]
O'lcham (px):   [ 80 ] slider 30-200
```

**Preview:** Frontend tomondan `qrcode.js` bilan live preview ko'rsatish — user backend'ga yuborishdan oldin natijani ko'radi.

**Default'lar:**
- size: 80
- position: "top-right"

---

#### 💧 watermark

**UI — Mode tab:**
```
( Text )  ( Image )
```

**Text mode:**
```
Matn:           [DRAFT_____________]
Pozitsiya:      [Markaz ▾]
                ( c | tl | tr | bl | br )
Burilish:       [-45°] slider -90...90
Shaffoflik:     [0.4] slider 0...1
Sahifalar:      [ Hamma ▾ ]
```

**Image mode:**
```
Logo fayl:      [ Yuklash ]   ← oldin PNG/JPG yuklang
Pozitsiya:      [Pastki o'ng ▾]
Shaffoflik:     [0.5]
```

**Tip:** Visual preview obyekt — text/image'ni real PDF ustida ko'rsatish.

---

#### 🔒 protect — parol qo'yish

**UI:**
```
Parol:                  [ ●●●●●●●●●● ] [👁]
Parolni qaytaring:      [ ●●●●●●●●●● ]
```

**Client-side validation:**
- Min 6 ta belgi
- Ikkala parol mos kelishi

Backend `password` ni qabul qiladi, lekin **parol mosligini frontend tekshirishi** kerak.

**Warning:** "Parolingizni saqlang — yo'qotsangiz, biz tiklamaymiz".

---

#### 🔓 unlock — parolni olib tashlash

**UI:**
```
Parol:    [ ●●●●●●●●●● ] [👁]
```

**Error handling:**
- 202 → polling → `status: "failed"` + error contains "this file is not encrypted" → toast "Bu PDF himoyalangan emas"
- error contains "wrong password" → toast "Parol noto'g'ri"

---

#### 🔍 inspect ⚡ SYNC

**UI:**
- File picker
- "Tahlil qilish" tugmasi
- **Polling YO'Q** — javob darhol keladi
- Natija:
  ```
  Sahifalar: 12
  Sarlavha: ...
  Muallif: ...
  Mavzu: ...
  Kalit so'zlar: ...
  ```

**Error 422 → toast "PDF buzuq yoki PDF formati emas".

**Tip:** Boshqa PDF operatsiyalarda yuklashdan keyin avtomatik inspect chaqirish — user page_count ni darhol ko'rsin (range tanlash uchun).

---

#### ⬜ detect-blank — bo'sh sahifalarni topish

**UI:**
- File picker
- Natija ro'yxat ko'rinishda:
  ```
  Bo'sh sahifalar: 3, 7, 12
  [ O'chirish ]   ← darhol /removepage chaqiradi
  ```

**Tip:** "Bo'shlarni avtomatik o'chirish" tugmasi = `removepage` ga `blank_pages.join(",")` yuborish.

---

#### 🖼 jpg-to-pdf — rasmlardan PDF

**UI:**
- Multi-file picker `accept="image/jpeg, image/png"`
- Yuklangan rasmlar thumbnail grid (drag-and-drop tartib)
- Rasm sonini ko'rsatish ("12 rasm, 8 MB")
- Disable agar > 50

**Tip:** Brauzer ichida thumbnail yaratish (FileReader + `<img>`), yuklashdan oldin user tartibni o'zgartira oladi.

---

#### 🖼 pdf-to-jpg

**UI:**
- File picker
- "Aylantirish" tugmasi
- Natija: thumbnail grid + ikkita tugma:
  - "Hammasini ZIP'da yuklash" (`zip_file_id`)
  - "Faqat tanlanganlarni yuklash" (selected `output_file_ids`)

---

#### 📄 word-to-pdf, excel-to-pdf, ppt-to-pdf

**UI har biriga bir xil:**
- File picker (accept .doc/.docx, .xls/.xlsx, .ppt/.pptx)
- "Aylantirish" tugmasi
- Result download

**Frontend MIME tekshirish:**
```js
const allowed = {
  "word-to-pdf": [".doc", ".docx"],
  "excel-to-pdf": [".xls", ".xlsx"],
  "ppt-to-pdf": [".ppt", ".pptx"]
};
```

---

#### 🌐 html-to-pdf

**UI:**
- Code editor (`<textarea>` yoki Monaco) — HTML matn kiritish
- Yoki HTML fayl picker → FileReader bilan o'qib yuborish
- Live preview (right pane)

**Tip:** Default template (`<!DOCTYPE html>...`) avtomatik o'rab qo'yish — user faqat content yozsin.

---

#### 🔗 url-to-pdf

**UI:**
```
URL:              [ https://_______________________ ]
Sahifa o'lchami:  [ A4 ▾ ]   (A3, A4, A5, B4, B5)
Yo'nalish:        ( • ) Portret  ( ) Landscape
Kutish:           [ 2 ] soniya  (JS ko'p saytlar uchun)
```

**Validation:** URL format `^https?://...`.

**Warning:** "Auth talab qiluvchi saytlar ishlamaydi" — frontend ogohlantiradi.

---

#### 📄 pdf-to-word, pdf-to-ppt, pdf-to-excel, pdf-to-html

**UI bir xil:** File picker + "Aylantirish" tugmasi.

**Natija:** Output fayl extension'iga qarab tugma matni:
- pdf-to-word → "Word fayl yuklash (.docx)"
- pdf-to-ppt → "PowerPoint yuklash (.pptx)"
- pdf-to-excel → "Excel yuklash (.xlsx)"
- pdf-to-html → "HTML yuklash"

---

#### 🔤 OCR oilasi (6 ta) — umumiy UI

**Til tanlash dropdown:**
```
Til:  [ Inglizcha (eng) ▾ ]
  - Inglizcha (eng)
  - Ruscha (rus)
  - O'zbekcha lotin (uzb)
  - O'zbekcha kirill (uzb_cyrl)
  - Qirg'izcha (kir)
  - + Boshqa til qo'shish (multi-select uchun "eng+rus")
```

**Per-op UI:**

| Op | Input | Output ko'rinish |
|---|---|---|
| `/ocr` | PDF | Yangi searchable PDF (download) |
| `/scan-to-pdf` | Multi-image (max 50) | Searchable PDF |
| `/ocr-extract-text` | PDF | .txt fayl + matn preview |
| `/image-to-text` | Bitta rasm | .txt + preview |
| `/ocr-detect-lang` | PDF/rasm | "Til: eng (97% ishonch)" |
| `/ocr-structured` | PDF | HOCR/JSON fayl |

**Force OCR checkbox** (`/ocr` uchun):
```
☐ Mavjud matn qatlamini ham qayta OCR qil
   (default: faqat skan sahifalar)
```

**Edge case'lar:**
- Juda kichik rasm → `failed` → toast "Rasm juda kichik, kamida 3px kerak"
- Bo'sh PDF (faqat oq sahifalar) → `failed` → toast "Sahifada matn yo'q"

---

#### 📝 extract-text — OCR'siz matn

**UI:**
```
Format:  ( • ) TXT (oddiy matn)  ( ) JSON (sahifa bo'yicha)
```

**Default:** txt.

**Natijani frontend'da preview qilish** — txt'ni matn maydonida ko'rsatish.

---

#### 📊 nup — bir varaqqa N sahifa

**UI:**
```
Bir varaqqa nechta sahifa:
  ( ) 2-up   ( • ) 4-up   ( ) 6-up   ( ) 9-up

Yo'nalish:    ( • ) Portret  ( ) Landscape
Chegara:      ☑ Sahifalar orasiga chegara qo'yish
```

**Default:** n=4, orientation=portrait, border=false.

**Tip:** Visual misol ko'rsatish — har n qiymati uchun kichik diagramma.

---

#### 🏷 metadata — metadata o'zgartirish

**UI:**
```
Avval mavjud metadata ko'rsatish (inspect orqali):
─────────────────────────────────
Sarlavha:  [ Eski sarlavha______ ]
Muallif:   [ Eski muallif_______ ]
Mavzu:     [ ___________________ ]
Kalit so'zlar: [tag1, tag2, ...]
─────────────────────────────────
[ Saqlash ]
```

**MUHIM:** Bo'sh maydon bersangiz, backend `failed` qaytaradi ("no metadata fields provided"). Frontend kamida bitta field to'ldirilganini tekshirishi kerak.

**Tip:** "Bo'shatish" tugmasi — hamma field'ni bo'sh `null` qilib yuborish (lekin server qo'llab-quvvatlaydimi? — sinab ko'ring).

---

#### 🔗 share — public link yaratish

**UI:**
```
Bu faylni public qilib ulashing:

TTL (qancha vaqt ishlasin):
  ( ) 1 soat
  ( • ) 24 soat (default)
  ( ) 7 kun
  ( ) Maxsus: [ 60 ] daqiqa

[ Link yaratish ]

Natija:
https://convertpdfgo.com/api/pdf/share/abc123...
[ Nusxa olish ] [ QR code ko'rish ]
```

**Field nomi:** `file_id` (boshqalarda `input_file_id`!).

**UX:** Link nusxa olingach toast "Link nusxa olindi". QR code generation — `qrcode.js` bilan brauzerda.

---

### 12.7 Aloqa formasi UI

**Validatsiya:**

| Field | Constraint |
|---|---|
| name | 2-255 belgi |
| email | valid email |
| subject | 0-255 (ixtiyoriy) |
| body | 5-8000 belgi |

**UI:**
```
Ism:     [ _________________ ] *
Email:   [ _________________ ] *
Mavzu:   [ _________________ ]
Xabar:
┌──────────────────────────────┐
│                              │
│                              │  500/8000
│                              │
└──────────────────────────────┘
                          [ Yuborish ]
```

**Auth bo'lsa:** ism va email avtomatik to'ldirilsin (`/me` dan).

**Natija (201):** toast "Xabaringiz qabul qilindi. Email orqali javob beramiz."

---

### 12.8 WebSocket connection lifecycle

**Best practice — global WS manager:**

```js
class JobWatcher {
  constructor(token) {
    this.token = token;
    this.handlers = new Map();   // jobId → callback
    this.connect();
  }

  connect() {
    this.ws = new WebSocket(`wss://convertpdfgo.com/ws?token=${this.token}`);
    this.ws.onmessage = ({ data }) => {
      const e = JSON.parse(data);
      const cb = this.handlers.get(e.job_id);
      if (cb) cb(e);
    };
    this.ws.onclose = () => setTimeout(() => this.connect(), 2000);   // auto-reconnect
  }

  watch(jobId, cb) {
    this.handlers.set(jobId, cb);
    return () => this.handlers.delete(jobId);   // unwatch
  }
}

// Foydalanish:
const watcher = new JobWatcher(token);
const unwatch = watcher.watch(jobId, (event) => {
  if (event.status === "done") {
    showDownloadButton(event.output_file_id);
    unwatch();
  }
});
```

---

### 12.9 Tezroq UX patternlar

#### Inspect dan oldin avtomatik chaqirish

Foydalanuvchi PDF yuklagandan keyin **darhol** inspect chaqiring va saqlang:

```js
const fileId = await uploadFile(pdfBlob);
const { Data: meta } = await axios.post("/api/pdf/inspect", { file_id: fileId });
// Endi meta.page_count user'ga "10 sahifa" deb ko'rsatish mumkin
// Va boshqa op'larning page range input'iga limit qo'yish
setPageRange({ max: meta.page_count });
```

#### Page range input — visual validator

Foydalanuvchi "1-3,5,15" deb yozsa, agar PDF 10 sahifa bo'lsa, 15 noto'g'ri. Frontend tekshirsin:

```js
function validateRange(input, maxPage) {
  const parts = input.split(",").map(s => s.trim());
  for (const p of parts) {
    if (p.includes("-")) {
      const [a, b] = p.split("-").map(Number);
      if (a < 1 || b > maxPage || a > b) return false;
    } else {
      const n = Number(p);
      if (!Number.isInteger(n) || n < 1 || n > maxPage) return false;
    }
  }
  return true;
}
```

#### Cloud export tugmasini conditional ko'rsatish

```jsx
{providers.filter(p => p.connected).map(p => (
  <button onClick={() => exportTo(p.name)}>
    Save to {p.display_name}
  </button>
))}
```

#### Drag-drop yagona endpoint uchun emas, butun sahifa uchun

Foydalanuvchi istalgan joyga fayl tashlasa, app modal ochib operatsiyani tanlatsin.

```js
document.addEventListener("drop", (e) => {
  e.preventDefault();
  const file = e.dataTransfer.files[0];
  if (file.type === "application/pdf") {
    showOperationPicker(file);   // "Nima qilamiz: Compress? Rotate? OCR?"
  }
});
```

---

### 12.10 Kontekstli ogohlantirishlar

Har joyda foydalanuvchini ogohlantirish:

| Kontekst | Toast / Tooltip |
|---|---|
| Guest 28 MB fayl yuklamoqchi | "Kirish bilan 50 MB gacha yuklay olasiz" |
| OCR 10+ sahifa | "Bu jarayon 30+ soniya olishi mumkin" |
| compress=high | "Yuqori siqish sifatni kamaytiradi, faqat skan/email uchun" |
| Same file yuklash | "Bu fayl avval yuklangan, qayta convert qilamiz" |
| Job 30 soniyadan ko'p | "Sekin ishlayapti, sabr qiling..." |
| Result 5 daqiqa qoldi | "Faylingiz tez orada o'chiriladi, hozir saqlang" |

---

### 12.11 Loading states — har op uchun progress

**Spinner emas, progress message:**

```
Job pending...      "Navbatda..."
Job processing...   "Ishlanyapti..."
Job done            ✓ "Tayyor!"
Job failed          ✗ "Xato: ..."
```

**Long-running** uchun progress bar (vaqt asosida emas, indeterminate):
```jsx
{status === "processing" && (
  <div>
    <CircularProgress />
    <p>{op === "ocr" ? "OCR ishlanmoqda... (~30s)" : "Aylantirilmoqda..."}</p>
  </div>
)}
```

---

### 12.12 Mobile-first tafsiyalar

- Drag-drop YO'Q mobile'da → "Fayl tanlash" tugmasini ko'rsating
- Cloud export tugmalari kichik bo'lsin
- PDF preview mobile'da memory-heavy → faqat thumbnail
- Page range input — "Hamma sahifa" radio button default

---

## 13. Status: Live va Ishlayotgan

✅ HTTPS: `https://convertpdfgo.com` (Let's Encrypt)
✅ OAuth: Google + GitHub
✅ 35 PDF op
✅ WebSocket real-time
✅ Cloud export (Dropbox + GDrive)
✅ Auto 5-min cleanup
✅ Snake_case validation errors
✅ User-scoped dedup (privacy)
✅ HTTP→HTTPS auto-redirect
✅ Upload limit: 30 MB guest / 50 MB auth
✅ 401 da `sign_in_paths` OAuth tugmalari uchun

---

> **Bag yoki yangi savol?** 500 javobida `request_id` bor — uni support'ga yuboring.
