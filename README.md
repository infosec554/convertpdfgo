# 📎 convertpdfgo — Authentication & PDF Processing API

Bu loyiha `Golang`da yozilgan **monolit autentifikatsiya va PDF xizmatlari tizimi** bo‘lib, JWT orqali foydalanuvchi boshqaruvi, fayllarni yuklash, tahrirlash, konvertatsiya qilish, birlashtirish, ajratish va boshqa ko‘plab PDF amallarini bajarishni ta'minlaydi.

---

## 🚀 Boshlang'ich sozlamalar

- Swagger: [`http://localhost:8080/swagger/index.html`](http://localhost:8080/swagger/index.html)
- Port: `8080`

---

## 🔐 Authentication & Authorization Endpoints

| Endpoint            | Method | Tavsif                            |
|---------------------|--------|----------------------------------|
| `/signup`           | POST   | Ro‘yxatdan o‘tish (OTP token bilan) |
| `/login`            | POST   | Email va parol orqali tizimga kirish |
| `/refresh-token`    | POST   | Refresh token orqali yangilanish |
| `/me`               | GET    | JWT orqali o‘z profilini olish   |

---

## 🔐 OTP (bir martalik parol) Endpoints

| Endpoint          | Method | Tavsif                          |
|-------------------|--------|--------------------------------|
| `/otp/send`       | POST   | Emailga OTP yuborish          |
| `/otp/confirm`    | POST   | OTP ni tasdiqlash             |

---

## 🛡️ Admin Panel (Auth + Admin middleware talab qilinadi)

### 🔎 Loglar
| Endpoint               | Method | Tavsif                      |
|------------------------|--------|-----------------------------|
| `/admin/logs/:id`      | GET    | PDF job log'larini ko‘rish  |

### 👥 Rollar
| Endpoint         | Method | Tavsif                        |
|------------------|--------|-------------------------------|
| `/role`          | POST   | Rol yaratish                 |
| `/role/:id`      | PUT    | Rolni yangilash              |
| `/role`          | GET    | Barcha rollarni olish        |

### 🧑‍💼 SysUser (System foydalanuvchilari)
| Endpoint         | Method | Tavsif                        |
|------------------|--------|-------------------------------|
| `/sysuser`       | POST   | Admin, Moderator, Marketer yaratish |

---

## 📊 Statistika

| Endpoint           | Method | Tavsif                      |
|--------------------|--------|-----------------------------|
| `/stats/user`      | GET    | Foydalanuvchi statistikasi  |

---

## 📁 Fayllar (Token talab qilinadi)

| Endpoint              | Method | Tavsif                       |
|-----------------------|--------|------------------------------|
| `/file/upload`        | POST   | Fayl yuklash                |
| `/file/:id`           | GET    | Faylni olish                |
| `/file/:id`           | DELETE | Faylni o‘chirish            |
| `/file/list`          | GET    | Foydalanuvchining fayllari  |
| `/file/cleanup`       | GET    | Eski fayllarni tozalash (Admin) |

---

## 📚 PDF Xizmatlari (Token optional)

| Xizmat                      | POST                          | GET                                  |
|-----------------------------|-------------------------------|--------------------------------------|
| PDF birlashtirish (merge)   | `/api/pdf/merge`              | `/api/pdf/merge/:id`                 |
| PDF ajratish (split)        | `/api/pdf/split`              | `/api/pdf/split/:id`                 |
| Sahifani o‘chirish          | `/api/pdf/removepage`         | `/api/pdf/removepage/:id`           |
| Sahifalarni ajratib olish   | `/api/pdf/extract`            | `/api/pdf/extract/:id`              |
| PDF siqish                  | `/api/pdf/compress`           | `/api/pdf/compress/:id`             |
| JPG → PDF                  | `/api/pdf/jpg-to-pdf`         | `/api/pdf/jpg-to-pdf/:id`           |
| PDF → JPG                  | `/api/pdf/pdf-to-jpg`         | `/api/pdf/pdf-to-jpg/:id`           |
| Aylantirish (Rotate)        | `/api/pdf/rotate`             | `/api/pdf/rotate/:id`               |
| Crop qilish                | `/api/pdf/crop`               | `/api/pdf/crop/:id`                 |
| Qulfdan chiqarish (Unlock)  | `/api/pdf/unlock`             | `/api/pdf/unlock/:id`               |
| Himoya qilish (Protect)     | `/api/pdf/protect`            | `/api/pdf/protect/:id`              |
| Sahifalarga raqam qo‘shish  | `/api/pdf/add-page-numbers`   | `/api/pdf/add-page-numbers/:id`     |
| Header/Footer qo‘shish      | `/api/pdf/header-footer`      | `/api/pdf/header-footer/:id`        |
| Ulashish linki              | `/api/pdf/share`              | `/api/pdf/share/:token`             |
| PDF → Word                 | `/api/pdf/pdf-to-word`        | `/api/pdf/pdf-to-word/:id`          |
| Word → PDF                 | `/api/pdf/word-to-pdf`        | `/api/pdf/word-to-pdf/:id`          |
| Excel → PDF                | `/api/pdf/excel-to-pdf`       | `/api/pdf/excel-to-pdf/:id`         |
| PowerPoint → PDF           | `/api/pdf/ppt-to-pdf`         | `/api/pdf/ppt-to-pdf/:id`           |
| Watermark (matn) qo‘shish   | `/api/pdf/watermark`          | `/api/pdf/watermark/:id`            |

---

## ⚙️ Texnologiyalar

- Gin HTTP Framework
- PostgreSQL + Redis
- JWT & OTP Authentication
- pdfcpu, gofpdf, Gotenberg API
- Swagger Docs (`/swagger/index.html`)

---

## 🧪 Test qilish

Swagger orqali barcha endpointlarni test qilish mumkin:  
[`http://localhost:8080/swagger/index.html`](http://localhost:8080/swagger/index.html)

---

## 🧑‍💻 Developer uchun

**Foydalanish:**

```bash
make run
# yoki
go run main.go
