# convertpdfgo.com — Top 5 SEO Strategy

> **Maqsad:** 6 oy ichida 10 ta asosiy keyword'da Google'ning birinchi sahifasi (top 1-5).
> **Bozor:** Global English-speaking + O'zbekiston/Rossiya/CIS lokal niche.
> **Resurs:** 1 dasturchi (siz), 1 AI yordamchi (men), 0 marketing byudjeti.
> **Boshlanish:** 2026-05-31, hozirgi traffic 0 organic clicks/oy.

---

## 📊 0. Hozirgi holat audit (BUGUN o'tkazamiz)

Bu birinchi bosqich — har qaysi metric'ni o'lchaymiz, baseline yozamiz, keyin har hafta solishtirramiz.

### 0.1. Texnik infratuzilma — hozir mavjud

| Element | Status | Izoh |
|---|---|---|
| HTTPS (TLS 1.3) | ✅ | Caddy + Let's Encrypt avto-renewal |
| HTTP/2 | ✅ | (HTTP/3 o'chirilgan — concurrent request bug edi) |
| Compression | ✅ | gzip + zstd |
| Sitemap.xml | ✅ | `/sitemap.xml` (49 tool + 14 marketing) |
| Robots.txt | ✅ | `/robots.txt`, sitemap qayd qilingan |
| Canonical URL | ✅ | Har sahifada `<link rel="canonical">` |
| Open Graph | ✅ | og:title, og:description, og:image, og:url |
| Twitter Card | ✅ | summary_large_image |
| JSON-LD Schema | ⚠ | Organization + WebSite + FAQPage bor; HowTo, BreadcrumbList, Review yo'q |
| Favicon set | ✅ | SVG + ICO + 192/512 PNG + Apple-touch + manifest |
| Mobile responsive | ✅ | Hozir yaqinda tuzatildi |
| Marketing pages cache | ✅ | revalidate=3600 (hozir qo'shildi) |
| SSR speed | ✅ | ~50ms (cached), ~1s (cold) |

### 0.2. Yo'q bo'lgan texnik narsalar (P0)

- [ ] **Google Search Console** — sayt registratsiya qilinmagan, indekslanmagan
- [ ] **Google Analytics 4** — tracking yo'q
- [ ] **Bing Webmaster Tools** — Bing/Yandex foydalanuvchilari uchun
- [ ] **Yandex Webmaster** — Rossiya/Uzbekistan auditoriyasi uchun KRITIK
- [ ] **HowTo schema** — har tool sahifasiga
- [ ] **BreadcrumbList schema**
- [ ] **Author/Publisher schema** — E-E-A-T uchun
- [ ] **hreflang teglari** — 20 ta locale uchun

### 0.3. Kontent holati

| Sahifa turi | Mavjud | Maqsad |
|---|---|---|
| Tool landing sahifalari | 49 ta | Har biri 800+ so'z bo'lishi kerak (hozir ~200) |
| Blog posts | 3-4 ta | 50+ kerak |
| Comparison maqolalari | 0 ta | 49 ta (vs smallpdf, ilovepdf, adobe) |
| How-to maqolalari | 0 ta | 30+ kerak |
| Locale sahifalari | 0 ta | Har keyword uchun uz/ru lokal'lashtirish |

### 0.4. Hozirgi metric'lar (baseline — 2026-05-31)

```
Foydalanuvchilar: 4 ta (3 sizdan tashqari)
Daily files processed: 0-3
Indekslangan sahifalar: ~0 (Google'da)
Organic search clicks: 0
Domain Rating (Ahrefs): 0
Backlink soni: ~0
LinkedIn followers: 8
```

---

## 🎯 1. Top 10 maqsadli keyword'lar va realistik baho

### 1.1. Eng yuqori ROI keyword'lar (English market)

| # | Keyword | Oylik search | Hozirgi sahifa | Raqib darajasi | Top 5 imkoniyat (oy) |
|---|---|---|---|---|---|
| 1 | "pdf to word free" | 165K | `/pdf-to-word` | Yuqori (smallpdf #1) | 6-9 oy |
| 2 | "compress pdf" | 110K | `/compress` | Yuqori | 6-9 oy |
| 3 | "merge pdf" | 90K | `/merge` | Yuqori | 6-12 oy |
| 4 | "pdf to jpg" | 75K | `/pdf-to-jpg` | O'rta-yuqori | 4-6 oy |
| 5 | "pdf editor online free" | 55K | `/tools` | O'rta | 4-6 oy |
| 6 | "ocr pdf free" | 40K | `/ocr` | O'rta | 3-6 oy |
| 7 | "convert pdf to excel" | 30K | `/pdf-to-excel` | O'rta | 3-6 oy |
| 8 | "**chat with pdf ai free**" | 22K | `/chat` | **Past** ⭐ | 2-3 oy |
| 9 | "**ai resume builder free no signup**" | 18K | `/cv-builder` | **Past** ⭐ | 2-3 oy |
| 10 | "**ats checker free online**" | 12K | `/ats-check` | **Past** ⭐ | 2-3 oy |

⭐ — Bu **biz ustun bo'la oladigan** kategoriya: smallpdf/ilovepdf'da yo'q. AI niche'da raqib kam.

### 1.2. Lokal market — past raqobat, yuqori konversiya

| Keyword | Oylik (UZ) | Imkoniyat |
|---|---|---|
| "pdf word ga aylantirish" | 5K | Top 3 — 1-2 oy |
| "pdf birlashtirish" | 3K | Top 3 — 1-2 oy |
| "pdf siqish online" | 2K | Top 3 — 1-2 oy |
| "конвертер pdf word онлайн" | 8K | Top 5 — 2-3 oy |
| "сжать pdf онлайн бесплатно" | 12K | Top 5 — 2-3 oy |

**Strategiya:** Lokalliga ko'proq fokus → tezroq natija → Domain Authority oshadi → keyin global keyword'lar.

---

## 📅 2. 6 oylik bosqichli reja

### **Faza 1: Texnik baza (1-2 hafta)**

#### Hafta 1
- [ ] **Google Search Console** ulash, sitemap yuborish
- [ ] **Google Analytics 4** o'rnatish
- [ ] **Yandex Webmaster** + **Bing Webmaster** ham ulash
- [ ] **PageSpeed Insights** baseline: har asosiy sahifa uchun (49 tool + 14 marketing)
- [ ] **GTmetrix** A daraja olish
- [ ] **Mobile-Friendly Test** — 100% pass
- [ ] **Core Web Vitals** — yashil (LCP <2.5s, FID <100ms, CLS <0.1)
- [ ] **HTTPS Mixed Content** tekshiruv
- [ ] **404/redirect xatolar** — Screaming Frog crawl

#### Hafta 2
- [ ] **HowTo schema** har tool uchun
- [ ] **BreadcrumbList schema**
- [ ] **Article schema** blog posts uchun
- [ ] **Organization schema** kengaytirish — sameAs (LinkedIn, GitHub, Telegram)
- [ ] **hreflang** 20 ta locale uchun
- [ ] **Image alt text** har bir rasm
- [ ] **Internal linking** — har tool boshqa 3-5 ta tool'ga link

#### O'lchov metrikalari
```
PageSpeed mobile  ≥ 85
PageSpeed desktop ≥ 95
LCP ≤ 2.0s
CLS ≤ 0.05
GSC indexed pages ≥ 60
```

---

### **Faza 2: On-page kontent (3-6 hafta)**

#### Har bir tool sahifasi kengaytiriladi (~30 daqiqa/sahifa × 49 = 25 soat)

**Tool sahifasi template (har bittasi 800-1200 so'z):**

```markdown
# [Tool Name] — [Action verb] [object] [benefit clause]

H2: How to [task] in 3 steps
H2: Why [tool] matters for [audience]
H2: [Tool] vs [Competitor] — feature comparison
H2: Frequently asked questions about [tool]
H2: Related tools

Content blocks:
- 200-word intro (with target keyword in first 100 chars)
- Step-by-step instructions (numbered, with screenshots)
- Comparison table (us vs smallpdf vs ilovepdf vs Adobe)
- 5 FAQ (matches "People also ask" in Google)
- CTA
- Internal links to 5 related tools
```

#### Birinchi 10 ta sahifa (high-ROI prioriteti):
1. `/cv-builder` — kam raqobat, yuqori niche
2. `/ats-check` — recruitment niche
3. `/chat` — AI trend
4. `/pdf-to-word`
5. `/compress`
6. `/merge`
7. `/pdf-to-jpg`
8. `/pdf-to-excel`
9. `/ocr`
10. `/jpg-to-pdf`

#### O'lchov:
- Har sahifa Ahrefs/SEMrush "On-page score" ≥ 85
- Har sahifa keyword density: 1-2% asosiy keyword, 0.5% LSI keyword
- Readability (Flesch): ≥ 60 (oson o'qish)
- Word count: 800+ minimum

---

### **Faza 3: Blog kontent (1-3 oy, haftada 2 post)**

#### Birinchi 24 ta blog post (12 hafta × 2/hafta):

**How-to maqolalari (yuqori intent):**
1. "How to compress PDF to 100KB without losing quality (2026 guide)"
2. "How to convert PDF to Word without watermarks — 3 free methods"
3. "How to merge PDF files on iPhone, Android, Mac, and Windows"
4. "How to remove password from PDF when you forgot it"
5. "How to OCR a scanned PDF for free — step by step"
6. "How to chat with your PDF using AI — Gemini vs ChatGPT compared"
7. "How to build an AI-friendly resume that passes ATS in 2026"
8. "How to translate a PDF to 20 languages while keeping formatting"

**Comparison maqolalari (lower funnel):**
9. "convertpdfgo vs smallpdf — head-to-head feature comparison"
10. "convertpdfgo vs ilovepdf — pricing, privacy, speed tested"
11. "convertpdfgo vs Adobe Acrobat — when free actually wins"
12. "Best free PDF tools in 2026 — tested and ranked"
13. "Best AI Resume Builder in 2026 — 7 platforms compared"
14. "Best free OCR online in 2026 — accuracy benchmark"

**Niche/long-tail (raqib kam, konversiya yuqori):**
15. "PDF to Word for academics — preserving citations and equations"
16. "How developers automate PDF workflows with our REST API"
17. "Why your CV gets rejected by ATS (and how to fix it in 5 min)"
18. "Privacy-first PDF tools — why your files shouldn't go to Adobe Cloud"
19. "Open source PDF tools vs convertpdfgo — when each makes sense"

**Lokal (O'zbekiston bozori):**
20. "PDF ni Word ga qanday o'tkazish — eng oson 3 usul"
21. "O'zbek tilida CV/Resume yaratish — AI bilan 5 daqiqada"
22. "Talabalar uchun bepul PDF asboblari ro'yxati"
23. "Биз нима учун convertpdfgo яратдик — Тошкентдан йўл хикояси" (uz Cyrillic)

**Locale lokalizatsiya (24-chi):**
24. Har post English'da yozilgach uz + ru + tr (top 4 lokal market) ga tarjima

#### O'lchov:
- Har post: 1500-2500 so'z
- Internal links: minimum 5 ta tool sahifasi
- External links: 2 ta authoritative manba (DR 50+)
- Tasvirlar: 3-5 ta, har biri WebP + alt text
- Update frequency: har 6 oyda bir post update qilamiz

---

### **Faza 4: Backlink (2-6 oy davomida)**

#### Bepul katalog'lar (1-hafta, 50 ta link)

| Sayt | DR | Vaqt | Status |
|---|---|---|---|
| ProductHunt — launch | 91 | 1 hafta tayyorlanish | Maker yo'q? Telegram'da topish |
| AlternativeTo (vs smallpdf) | 84 | 30 min | Boshlash |
| G2 | 92 | 1 soat (profile + 3 review) | Asoschi sifatida |
| Capterra | 86 | 1 soat | Asoschi sifatida |
| SaaSHub | 56 | 30 min | Profile yaratish |
| BetaList | 67 | 30 min | Submit |
| Indiehackers | 75 | 1 soat | Profile + post |
| Dev.to | 76 | 2 soat | "How we built..." maqola |
| Hashnode | 64 | 2 soat | Texnik maqola |
| Medium | 96 | 2 soat | Story + canonical |
| StackShare | 79 | 30 min | Tech stack profile |

#### O'zbekiston/Rossiya katalog'lari (lokal)
- IT Park UZ — rasmiy ro'yxat
- spot.uz, uztech.uz — press release
- Habr — Rus tilida texnik maqola (DR 86)
- vc.ru — startup hikoyasi
- Telegram tech kanallari: @startupsuz, @itparkuz, @uzdev — post

#### Guest post + outreach (oyiga 2-3 ta)
- TechCrunch's "Show TC" — agar growth bo'lsa
- Sifted, EU-TechStars — Yevropa fokus
- Dev.to weekly digest — har hafta yangi maqola
- O'zbek tech bloglari — yangi siyosatlarni yozish

#### Reddit (oyiga 5 ta natural post)
- r/SideProject
- r/SaaS
- r/InternetIsBeautiful
- r/learnprogramming (sample API uchun)
- r/CV/Resume (CV builder uchun)
- r/Uzbekistan (lokal sotuvchilar)

#### HackerNews
- Show HN — Tuesday 09:00 UTC eng yaxshi vaqt
- Front page'ga chiqsa = +10-50 DR ekvivalenti
- Faqat 1 marta urinish — mukammal tayyorgarlik kerak

#### O'lchov:
- Backlink soni: 50 (2 oy) → 150 (4 oy) → 300+ (6 oy)
- Referring Domains: 30 (2 oy) → 100 (4 oy) → 200+ (6 oy)
- Domain Rating (Ahrefs): 0 → 20 (2 oy) → 35 (6 oy)
- Anchor text diversity: 30% brand, 30% naked URL, 40% generic + keyword

---

### **Faza 5: O'lchash va iteratsiya (har hafta abadiy)**

#### Har dushanba (har hafta 30 daqiqa)
- GSC: impressions / clicks / CTR / position dashboard
- GA4: real foydalanuvchilar / page views / bounce
- Ahrefs: yangi backlink'lar
- PageSpeed: random 3 ta sahifa
- Top 10 keyword position kuzatish

#### Har oyning 1-ida (1 soat)
- 10 ta asosiy keyword pozitsiya jadvali (sparkline)
- Eng yaxshi/yomon 5 ta sahifa analiz
- Indekslangan sahifa soni — to'liq audit
- Crawl errors — Screaming Frog
- New competitor analysis (kim yangi chiqdi?)

#### Har choragi (har 3 oy)
- Content audit — eng yomon 10 ta sahifa qayta yozish
- Keyword strategy yangilash
- Backlink toxic audit (Penguin'dan saqlanish)
- Schema markup audit
- Site architecture review

---

## 🎯 3. Har tool uchun individual SEO audit reja

49 ta tool'ni 5 ta kategoriyaga ajratamiz, har birini alohida sahifa optimizatsiya qilamiz.

### 3.1. PDF Manipulation (15 ta tool)
merge, split, compress, remove-pages, extract-pages, rotate, crop, n-up, inspect, detect-blank-pages, edit-metadata, extract-text, add-page-numbers, header-footer, add-qr-code

### 3.2. PDF Conversion (10 ta tool)
pdf-to-word, word-to-pdf, pdf-to-excel, excel-to-pdf, pdf-to-ppt, ppt-to-pdf, pdf-to-jpg, jpg-to-pdf, pdf-to-html, html-to-pdf, url-to-pdf

### 3.3. PDF Security (3 ta tool)
sign, protect, unlock

### 3.4. AI-Powered (10 ta tool)
chat-with-pdf, chat-with-word, chat-with-excel, chat-with-ppt, ocr, scan-to-pdf, extract-text-ocr, image-to-text, detect-language, structured-ocr

### 3.5. AI Career (8 ta tool)
cv-builder, cv-from-pdf, cv-voice, ats-check, resume-translate, cover-letter, resume-to-docx, chat-with-cv

### Audit checklist har tool uchun

```
Tool: /[slug]
─────────────────────────────────────────
[ ] PageSpeed mobile  ≥ 85
[ ] PageSpeed desktop ≥ 95
[ ] LCP ≤ 2.0s
[ ] CLS ≤ 0.05
[ ] Title 50-60 belgi, keyword birinchi
[ ] Meta description 140-160 belgi, CTR-friendly
[ ] H1 1 ta, keyword bilan
[ ] H2-H3 struktura mantiqiy
[ ] URL slug qisqa, keyword bilan
[ ] Image alt text har biri
[ ] Internal links 5+ ta
[ ] External links 1-2 ta authoritative
[ ] Schema: HowTo + BreadcrumbList + FAQ + SoftwareApplication
[ ] Content 800+ so'z
[ ] FAQ qismi (5 ta savol)
[ ] Comparison jadvali (vs 3 raqib)
[ ] CTA tugmasi above-the-fold
[ ] Mobile responsive (tekshirildi)
[ ] Google indeksida (GSC tasdiqlangan)
[ ] Konversiya tracking (GA4 event)
[ ] First impression 1-oy ichida ≥ 100
```

---

## 📈 4. Maqsad metrikalari (6 oyda)

| Metric | 0-oy | 1-oy | 2-oy | 3-oy | 6-oy |
|---|---|---|---|---|---|
| Indekslangan sahifa | 0 | 50 | 100 | 150 | 200+ |
| Organic clicks/oy | 0 | 50 | 500 | 2K | 20K+ |
| Top 10 keyword (Google) | 0 | 2 | 8 | 20 | 50+ |
| Top 5 keyword | 0 | 0 | 2 | 5 | 15+ |
| Top 1-3 keyword | 0 | 0 | 0 | 1-2 | 5-10 |
| Backlinks | 0 | 20 | 50 | 100 | 300+ |
| Domain Rating | 0 | 5 | 12 | 18 | 30+ |
| LinkedIn followers | 8 | 50 | 150 | 400 | 1K+ |
| Daily active users | 0-1 | 20 | 100 | 500 | 2K+ |
| Monthly users | 4 | 200 | 1K | 5K | 50K+ |

---

## 🛠 5. Ishlatadigan vositalar (hammasi bepul yoki freemium)

### Bepul (must-have)
- **Google Search Console** — indekslanish + keyword'lar
- **Google Analytics 4** — trafik analitika
- **Yandex Webmaster** — Rossiya/O'zbekiston market
- **Bing Webmaster** — qo'shimcha qamrov
- **PageSpeed Insights** — tezlik
- **Google Trends** — keyword trend'lari
- **Google Keyword Planner** — search hajmi (ads account kerak)
- **Screaming Frog SEO Spider** (free 500 URL)
- **Schema Markup Validator** — schema.org rasmiy
- **GTmetrix** — tezlik audit
- **Mobile-Friendly Test** — Google
- **Wayback Machine** — raqib evolutsiya tarixi

### Freemium (kerak bo'lganda 1-2 oylik trial)
- **Ahrefs Webmaster Tools** — bepul (faqat o'z saytim) — DR + backlink monitoring
- **SEMrush** (7 kun bepul) — keyword research bir martalik
- **Ubersuggest** (3 query/kun bepul) — quick lookup
- **Answer The Public** — savol-keyword'lar
- **Keyword.io** — long-tail
- **Surfer SEO** ($59/oy, content optimization, 1 oylik trial)

### O'zimiz qurib chiqishimiz mumkin (open source)
- **Lighthouse CLI** — avtomatik audit
- **Plausible Analytics** (yoki self-hosted) — GDPR-friendly tracking
- **Umami** (open source) — analytics

---

## 🚀 6. Birinchi 7 kun uchun aniq qadamlar

### 1-kun (BUGUN)
- [ ] **PageSpeed audit** har asosiy sahifaga (man yordamlashaman)
- [ ] **Google Search Console** ro'yxatdan o'tish
- [ ] HTML verification kodi → men sayt'ga joylashtirib beraman
- [ ] Sitemap submit
- [ ] Baseline screenshot (impressions / clicks 0)

### 2-kun
- [ ] Google Analytics 4 ulash
- [ ] Yandex Webmaster ulash
- [ ] Bing Webmaster ulash
- [ ] gtag.js men sayt'ga qo'shaman

### 3-kun
- [ ] **HowTo schema** Top 10 tool'ga
- [ ] **BreadcrumbList schema** hammasiga
- [ ] **Article schema** blog post'larga
- [ ] Tasvir alt text'lari audit

### 4-kun
- [ ] hreflang teglari 20 locale uchun
- [ ] Internal linking — har sahifa boshqa 5 ta'ga
- [ ] Tasvir optimizatsiya (WebP, lazy load)

### 5-kun
- [ ] Birinchi 5 ta tool sahifasini 800+ so'zga kengaytirish (Top 10 ROI)
- [ ] FAQ bo'limini har sahifaga qo'shish
- [ ] Comparison jadvallar

### 6-kun
- [ ] Birinchi 2 ta blog post: "Compress PDF to 100KB" + "PDF to Word free"
- [ ] LinkedIn post — yangi blog'lar haqida
- [ ] Telegram kanalda post

### 7-kun
- [ ] Hafta yakuni: nima yutdik?
- [ ] GSC impressions yo'qmi? (yo'q bo'lsa OK, hali yangi)
- [ ] Indekslangan sahifalar
- [ ] PageSpeed yaxshilanishi
- [ ] Keyingi hafta uchun plan

---

## 📊 7. Har hafta progress dashboard (sodda jadval)

```
Hafta №___ (sanasi)

Texnik:
  PageSpeed avg:     __ / 100
  Core Web Vitals:   __ / 3 yashil
  Indekslangan:      __ sahifa
  Crawl errors:      __ ta

Kontent:
  Yangi tool sahifa:  __ ta
  Yangi blog post:    __ ta
  Yangi til tarjima:  __ ta

Trafik:
  GSC impressions:   __
  GSC clicks:        __
  GSC CTR:           __ %
  GSC avg position:  __
  GA4 sessions:      __
  GA4 unique users:  __

Backlink:
  Yangi linklar:      __ ta
  Referring domain:   __
  DR (Ahrefs):       __

Top 10 keyword pozitsiya:
  [Keyword 1]:       __ pos (o'zgarish: ___)
  [Keyword 2]:       __ pos (o'zgarish: ___)
  ...

Maqsadga yaqinlik:  __ %
```

---

## ⚠️ 8. Diqqat qilish kerak narsalar

### YOQ qilish kerak (penalty'ga olib keladi)
- ❌ Keyword stuffing — 3%+ density
- ❌ Cloaking — turli xil kontent bot va user uchun
- ❌ PBN (private blog network) — qora backlink
- ❌ Fiverr'da link sotib olish
- ❌ Doorway pages
- ❌ Sayt'ga "Spinbot" tarjimalari yopishtirib qoldirish
- ❌ Duplicate content (boshqa sayt'lardan ko'chirish)

### Ehtiyot bo'lish kerak
- ⚠️ AI yozgan kontentni 100% qabul qilmaslik — har biri qo'l bilan tahrirlash
- ⚠️ User-generated reviews — soxta yozish penalty
- ⚠️ Cloud-tier kompaniyalardan bepul backlink — past sifat
- ⚠️ "SEO services" sotuvchilari — 90% scam

### YAXSHI strategiya
- ✅ Foydalanuvchilar haqida o'ylash — Google ham foydalanuvchini ko'radi
- ✅ E-E-A-T (Experience, Expertise, Authority, Trust)
- ✅ Long-form content > short
- ✅ Original research / data > rewrite
- ✅ Real user reviews
- ✅ Texnik blog > marketing blog

---

## 🎬 9. Birinchi qadam — HOZIRGI VAZIFA

Bu MD faylni o'qib bo'lgach, **birinchi 3 ta vazifa**:

1. **PageSpeed audit** — `https://pagespeed.web.dev/analysis?url=https://convertpdfgo.com`
   - Mobile + Desktop tab'larini ochib screenshot oling
   - LCP, FID, CLS qiymatlarini menga ayting
   - Men ko'rib chiqib, qaerda muammo bor — aytaman va tuzataman

2. **Google Search Console**
   - https://search.google.com/search-console ga kiring
   - `+ Add property` → `https://convertpdfgo.com`
   - "HTML tag" usulini tanlang
   - Olingan `<meta name="google-site-verification" content="...">` kodini menga yuboring
   - Men `<head>`'ga joylashtiraman, siz "Verify" bosasiz
   - Keyin Sitemaps → `sitemap.xml` qo'shing

3. **Birinchi tool sahifani noldan optimize qilish**
   - `/cv-builder` ni tanlaymiz (sababi: kam raqib, top 5 osonroq)
   - Hozirgi kontent: ~200 so'z
   - Maqsad: 1200 so'z, 5 FAQ, comparison jadval, HowTo schema
   - Men kontentni yozaman, siz tahrirlab tasdiqlaysiz
   - GSC'da 1 hafta keyin tekshiramiz: indekslanyaptimi?

Tayyor bo'lsangiz, **birinchi qadamdan** boshlaymiz. Qaysi biridan? 🎯
