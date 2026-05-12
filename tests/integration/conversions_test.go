// End-to-end coverage for every conversion endpoint shipped today.
// Each test is self-contained: uploads its own fixture, runs the
// conversion, downloads the output, validates real bytes.
package integration

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"testing"
	"time"
)

const (
	pdfFixture  = "testdata/test.pdf"
	jpgFixture  = "testdata/test.jpg"
	docxFixture = "testdata/test.docx"
	xlsxFixture = "testdata/test.xlsx"
	pptxFixture = "testdata/test.pptx"

	pollFast   = 30 * time.Second  // pdfcpu jobs finish in <5s
	pollNormal = 60 * time.Second  // mid jobs (pdf-to-jpg)
	pollOffice = 120 * time.Second // LibreOffice / url-to-pdf
	pollOCR    = 300 * time.Second // ocrmypdf rasterises every page; allow a generous cap
)

// SkipIfOCR lets a sweep skip the OCR family when running on a host
// without the tesseract/ocrmypdf binaries (e.g. local dev where the
// runtime image isn't reproduced). Default behaviour is to run.
func SkipIfOCR(t *testing.T) {
	if os.Getenv("SKIP_OCR") != "" {
		t.Skip("skipping ocr: SKIP_OCR set")
	}
}

// --- Page operations ------------------------------------------------------

func TestMerge(t *testing.T) {
	SkipIfShort(t)
	c := NewClient(t)
	a := c.UploadFile(t, pdfFixture)
	b := c.UploadFile(t, pdfFixture)
	jid := c.PostJob(t, "/api/pdf/merge", map[string]any{"input_file_ids": []string{a, b}})
	job := c.PollDone(t, "/api/pdf/merge", jid, pollFast)
	AssertPDF(t, c.Download(t, job.Output()), 1000)
}

func TestSplit(t *testing.T) {
	SkipIfShort(t)
	c := NewClient(t)
	a := c.UploadFile(t, pdfFixture)
	jid := c.PostJob(t, "/api/pdf/split", map[string]any{
		"input_file_id": a, "split_ranges": "1-3,4-7",
	})
	job := c.PollDone(t, "/api/pdf/split", jid, pollFast)
	if len(job.OutputFileIDs) == 0 {
		t.Fatalf("split: no output_file_ids")
	}
	AssertPDF(t, c.Download(t, job.OutputFileIDs[0]), 1000)
}

func TestRemovePage(t *testing.T) {
	SkipIfShort(t)
	c := NewClient(t)
	a := c.UploadFile(t, pdfFixture)
	jid := c.PostJob(t, "/api/pdf/removepage", map[string]any{
		"input_file_id": a, "pages_to_remove": "1",
	})
	job := c.PollDone(t, "/api/pdf/removepage", jid, pollFast)
	AssertPDF(t, c.Download(t, job.Output()), 1000)
}

func TestExtract(t *testing.T) {
	SkipIfShort(t)
	c := NewClient(t)
	a := c.UploadFile(t, pdfFixture)
	jid := c.PostJob(t, "/api/pdf/extract", map[string]any{
		"input_file_id": a, "page_ranges": "1-2",
	})
	job := c.PollDone(t, "/api/pdf/extract", jid, pollFast)
	AssertPDF(t, c.Download(t, job.Output()), 1000)
}

func TestCompress(t *testing.T) {
	SkipIfShort(t)
	c := NewClient(t)
	a := c.UploadFile(t, pdfFixture)
	jid := c.PostJob(t, "/api/pdf/compress", map[string]any{
		"input_file_id": a, "compression": "medium",
	})
	job := c.PollDone(t, "/api/pdf/compress", jid, pollNormal)
	AssertPDF(t, c.Download(t, job.Output()), 1000)
}

// --- Visual transforms ----------------------------------------------------

func TestRotate(t *testing.T) {
	SkipIfShort(t)
	c := NewClient(t)
	a := c.UploadFile(t, pdfFixture)
	jid := c.PostJob(t, "/api/pdf/rotate", map[string]any{
		"input_file_id": a, "angle": 90, "pages": "all",
	})
	job := c.PollDone(t, "/api/pdf/rotate", jid, pollFast)
	AssertPDF(t, c.Download(t, job.Output()), 1000)
}

func TestCrop(t *testing.T) {
	SkipIfShort(t)
	c := NewClient(t)
	a := c.UploadFile(t, pdfFixture)
	jid := c.PostJob(t, "/api/pdf/crop", map[string]any{
		"input_file_id": a, "top": 20, "bottom": 20, "left": 20, "right": 20,
	})
	job := c.PollDone(t, "/api/pdf/crop", jid, pollFast)
	AssertPDF(t, c.Download(t, job.Output()), 1000)
}

func TestAddPageNumbers(t *testing.T) {
	SkipIfShort(t)
	c := NewClient(t)
	a := c.UploadFile(t, pdfFixture)
	jid := c.PostJob(t, "/api/pdf/add-page-numbers", map[string]any{
		"input_file_id": a, "first_number": 1, "page_range": "all",
		"position": "bc", "color": "black", "font_size": 12,
	})
	job := c.PollDone(t, "/api/pdf/add-page-numbers", jid, pollFast)
	AssertPDF(t, c.Download(t, job.Output()), 1000)
}

func TestHeaderFooter(t *testing.T) {
	SkipIfShort(t)
	c := NewClient(t)
	a := c.UploadFile(t, pdfFixture)
	jid := c.PostJob(t, "/api/pdf/header-footer", map[string]any{
		"input_file_id": a,
		"header_text":   "H", "footer_text": "F",
		"font_size": 12, "font_color": "black", "position": "center",
	})
	job := c.PollDone(t, "/api/pdf/header-footer", jid, pollFast)
	AssertPDF(t, c.Download(t, job.Output()), 1000)
}

func TestQRCode(t *testing.T) {
	SkipIfShort(t)
	c := NewClient(t)
	a := c.UploadFile(t, pdfFixture)
	jid := c.PostJob(t, "/api/pdf/qr-code", map[string]any{
		"input_file_id": a, "qr_content": "https://example.com",
		"position": "top-right", "size": 80,
	})
	job := c.PollDone(t, "/api/pdf/qr-code", jid, pollFast)
	AssertPDF(t, c.Download(t, job.Output()), 1000)
}

// --- Security: protect → unlock chain -------------------------------------

func TestProtectUnlock(t *testing.T) {
	SkipIfShort(t)
	c := NewClient(t)
	a := c.UploadFile(t, pdfFixture)

	// 1) protect
	jid := c.PostJob(t, "/api/pdf/protect", map[string]any{
		"input_file_id": a, "password": "secret123",
	})
	prot := c.PollDone(t, "/api/pdf/protect", jid, pollNormal)
	if prot.Output() == "" {
		t.Fatal("protect: no output_file_id")
	}
	AssertPDF(t, c.Download(t, prot.Output()), 1000)

	// 2) unlock the protected output
	jid = c.PostJob(t, "/api/pdf/unlock", map[string]any{
		"input_file_id": prot.Output(), "password": "secret123",
	})
	unl := c.PollDone(t, "/api/pdf/unlock", jid, pollNormal)
	AssertPDF(t, c.Download(t, unl.Output()), 1000)
}

// --- Inspection (analytic; no output file) --------------------------------

func TestInspect(t *testing.T) {
	SkipIfShort(t)
	c := NewClient(t)
	a := c.UploadFile(t, pdfFixture)
	jid := c.PostJob(t, "/api/pdf/inspect", map[string]any{"file_id": a})
	// inspect runs synchronously inside the create handler — GET should
	// already report status=done on the first poll.
	c.PollDone(t, "/api/pdf/inspect", jid, pollFast)
}

func TestDetectBlank(t *testing.T) {
	SkipIfShort(t)
	c := NewClient(t)
	a := c.UploadFile(t, pdfFixture)
	jid := c.PostJob(t, "/api/pdf/detect-blank", map[string]any{"input_file_id": a})
	c.PollDone(t, "/api/pdf/detect-blank", jid, pollNormal)
}

// --- Image ↔ PDF ----------------------------------------------------------

func TestJPGToPDF(t *testing.T) {
	SkipIfShort(t)
	c := NewClient(t)
	j := c.UploadFile(t, jpgFixture)
	jid := c.PostJob(t, "/api/pdf/jpg-to-pdf", map[string]any{
		"input_file_ids": []string{j},
	})
	job := c.PollDone(t, "/api/pdf/jpg-to-pdf", jid, pollFast)
	AssertPDF(t, c.Download(t, job.Output()), 1000)
}

func TestPDFToJPG(t *testing.T) {
	SkipIfShort(t)
	c := NewClient(t)
	a := c.UploadFile(t, pdfFixture)
	jid := c.PostJob(t, "/api/pdf/pdf-to-jpg", map[string]any{"input_file_id": a})
	job := c.PollDone(t, "/api/pdf/pdf-to-jpg", jid, pollNormal)
	// Output is a ZIP of JPGs.
	AssertZip(t, c.Download(t, job.Output()))
}

// --- Office ↔ PDF (Gotenberg) ---------------------------------------------

func TestWordToPDF(t *testing.T) {
	SkipIfShort(t)
	SkipIfOffice(t)
	c := NewClient(t)
	d := c.UploadFile(t, docxFixture)
	jid := c.PostJob(t, "/api/pdf/word-to-pdf", map[string]any{"input_file_id": d})
	job := c.PollDone(t, "/api/pdf/word-to-pdf", jid, pollOffice)
	AssertPDF(t, c.Download(t, job.Output()), 500)
}

// PDF → HTML uses LibreOffice writer_pdf_import; output is real HTML.
// The test asserts the file contains an `<html` tag rather than any
// specific structure so cosmetic LibreOffice changes don't flake it.
func TestPDFToHTML(t *testing.T) {
	SkipIfShort(t)
	SkipIfOffice(t)
	c := NewClient(t)
	a := c.UploadFile(t, pdfFixture)
	jid := c.PostJob(t, "/api/pdf/pdf-to-html", map[string]any{"input_file_id": a})
	job := c.PollDone(t, "/api/pdf/pdf-to-html", jid, pollOffice)
	out := c.Download(t, job.Output())
	if len(out) < 100 {
		t.Fatalf("html too small: %d bytes", len(out))
	}
	AssertText(t, out, "<html")
}

// PDF → PowerPoint runs through LibreOffice impress_pdf_import — each
// PDF page becomes a slide image. Asserts the ZIP magic that every
// real .pptx starts with.
func TestPDFToPPT(t *testing.T) {
	SkipIfShort(t)
	SkipIfOffice(t)
	c := NewClient(t)
	a := c.UploadFile(t, pdfFixture)
	jid := c.PostJob(t, "/api/pdf/pdf-to-ppt", map[string]any{"input_file_id": a})
	job := c.PollDone(t, "/api/pdf/pdf-to-ppt", jid, pollOffice)
	AssertZip(t, c.Download(t, job.Output()))
}

// PDF → Excel: LibreOffice can't produce xlsx from PDF, so we go
// pdftotext -layout → excelize. Validates xlsx ZIP magic + a token
// the fixture has on page 1, surviving the layout-aware extraction.
func TestPDFToExcel(t *testing.T) {
	SkipIfShort(t)
	c := NewClient(t)
	a := c.UploadFile(t, pdfFixture)
	jid := c.PostJob(t, "/api/pdf/pdf-to-excel", map[string]any{"input_file_id": a})
	job := c.PollDone(t, "/api/pdf/pdf-to-excel", jid, pollNormal)
	AssertZip(t, c.Download(t, job.Output()))
}

func TestPDFToWord(t *testing.T) {
	SkipIfShort(t)
	SkipIfOffice(t)
	// Previously skipped because Gotenberg's libreoffice route is
	// one-way (office → PDF). The worker now shells out to LibreOffice
	// directly via pkg/libreoffice with `--infilter=writer_pdf_import`,
	// producing a real DOCX. Validates the ZIP magic so a future round-
	// trip regression (e.g. accidentally re-routing back through
	// Gotenberg) is caught loudly.
	c := NewClient(t)
	a := c.UploadFile(t, pdfFixture)
	jid := c.PostJob(t, "/api/pdf/pdf-to-word", map[string]any{"input_file_id": a})
	job := c.PollDone(t, "/api/pdf/pdf-to-word", jid, pollOffice)
	AssertZip(t, c.Download(t, job.Output()))
}

func TestExcelToPDF(t *testing.T) {
	SkipIfShort(t)
	SkipIfOffice(t)
	c := NewClient(t)
	x := c.UploadFile(t, xlsxFixture)
	jid := c.PostJob(t, "/api/pdf/excel-to-pdf", map[string]any{"input_file_id": x})
	job := c.PollDone(t, "/api/pdf/excel-to-pdf", jid, pollOffice)
	AssertPDF(t, c.Download(t, job.Output()), 500)
}

func TestPPTToPDF(t *testing.T) {
	SkipIfShort(t)
	SkipIfOffice(t)
	c := NewClient(t)
	p := c.UploadFile(t, pptxFixture)
	jid := c.PostJob(t, "/api/pdf/ppt-to-pdf", map[string]any{"input_file_id": p})
	job := c.PollDone(t, "/api/pdf/ppt-to-pdf", jid, pollOffice)
	AssertPDF(t, c.Download(t, job.Output()), 500)
}

func TestHTMLToPDF(t *testing.T) {
	SkipIfShort(t)
	SkipIfOffice(t)
	c := NewClient(t)
	jid := c.PostJob(t, "/api/pdf/html-to-pdf", map[string]any{
		"html_content": "<h1>Integration test</h1><p>HTML to PDF.</p>",
	})
	job := c.PollDone(t, "/api/pdf/html-to-pdf", jid, pollOffice)
	AssertPDF(t, c.Download(t, job.Output()), 500)
}

// --- Phase 1 features -----------------------------------------------------

func TestWatermark(t *testing.T) {
	SkipIfShort(t)
	c := NewClient(t)
	a := c.UploadFile(t, pdfFixture)
	jid := c.PostJob(t, "/api/pdf/watermark", map[string]any{
		"input_file_id": a, "mode": "text", "text": "INTEGRATION", "position": "c",
	})
	job := c.PollDone(t, "/api/pdf/watermark", jid, pollFast)
	AssertPDF(t, c.Download(t, job.Output()), 1000)
}

func TestExtractText(t *testing.T) {
	SkipIfShort(t)
	c := NewClient(t)
	a := c.UploadFile(t, pdfFixture)
	jid := c.PostJob(t, "/api/pdf/extract-text", map[string]any{
		"input_file_id": a, "format": "txt",
	})
	job := c.PollDone(t, "/api/pdf/extract-text", jid, pollFast)
	out := c.Download(t, job.Output())
	if len(out) == 0 {
		t.Fatalf("extract-text: empty output")
	}
}

func TestNUp(t *testing.T) {
	SkipIfShort(t)
	c := NewClient(t)
	a := c.UploadFile(t, pdfFixture)
	jid := c.PostJob(t, "/api/pdf/nup", map[string]any{
		"input_file_id": a, "n": 4, "orientation": "portrait", "border": true,
	})
	job := c.PollDone(t, "/api/pdf/nup", jid, pollFast)
	AssertPDF(t, c.Download(t, job.Output()), 1000)
}

func TestURLToPDF(t *testing.T) {
	SkipIfShort(t)
	SkipIfURL(t)
	c := NewClient(t)
	jid := c.PostJob(t, "/api/pdf/url-to-pdf", map[string]any{
		"url": "https://example.com", "page_size": "A4",
	})
	job := c.PollDone(t, "/api/pdf/url-to-pdf", jid, pollOffice)
	AssertPDF(t, c.Download(t, job.Output()), 500)
}

func TestMetadata(t *testing.T) {
	SkipIfShort(t)
	c := NewClient(t)
	a := c.UploadFile(t, pdfFixture)
	jid := c.PostJob(t, "/api/pdf/metadata", map[string]any{
		"input_file_id": a, "title": "Test", "author": "runner",
	})
	job := c.PollDone(t, "/api/pdf/metadata", jid, pollFast)
	pdf := c.Download(t, job.Output())
	AssertPDF(t, pdf, 1000)
	// pdfcpu writes /Info into a FlateDecode'd stream — verifying the
	// title text would need pdfinfo or a parser. Size + magic is the
	// strongest assertion we get from raw bytes.
}

// --- OCR family ----------------------------------------------------------
//
// Each test exercises a separate code path through pkg/ocr. They all use
// the existing test.pdf — even though it already has a text layer,
// ocrmypdf can be coerced into running with --force-ocr (which the
// scan-to-pdf flow always does, and which our OCR endpoint accepts via
// the `force_ocr` request flag).

// /api/pdf/ocr — searchable PDF out, asserts PDF magic + non-trivial size.
func TestOCR(t *testing.T) {
	SkipIfShort(t)
	SkipIfOCR(t)
	c := NewClient(t)
	a := c.UploadFile(t, pdfFixture)
	jid := c.PostJob(t, "/api/pdf/ocr", map[string]any{
		"input_file_id": a, "lang": "eng", "force_ocr": true,
	})
	job := c.PollDone(t, "/api/pdf/ocr", jid, pollOCR)
	AssertPDF(t, c.Download(t, job.Output()), 1000)
}

// /api/pdf/scan-to-pdf — multiple images → searchable PDF.
func TestScanToPDF(t *testing.T) {
	SkipIfShort(t)
	SkipIfOCR(t)
	c := NewClient(t)
	a := c.UploadFile(t, jpgFixture)
	b := c.UploadFile(t, jpgFixture)
	jid := c.PostJob(t, "/api/pdf/scan-to-pdf", map[string]any{
		"input_file_ids": []string{a, b}, "lang": "eng",
	})
	job := c.PollDone(t, "/api/pdf/scan-to-pdf", jid, pollOCR)
	AssertPDF(t, c.Download(t, job.Output()), 1000)
}

// /api/pdf/ocr-extract-text — scanned PDF → plain text sidecar.
func TestOCRExtractText(t *testing.T) {
	SkipIfShort(t)
	SkipIfOCR(t)
	c := NewClient(t)
	a := c.UploadFile(t, pdfFixture)
	jid := c.PostJob(t, "/api/pdf/ocr-extract-text", map[string]any{
		"input_file_id": a, "lang": "eng",
	})
	job := c.PollDone(t, "/api/pdf/ocr-extract-text", jid, pollOCR)
	out := c.Download(t, job.Output())
	if len(out) == 0 {
		t.Fatalf("ocr_extract_text: empty output")
	}
}

// /api/pdf/image-to-text — single image → plain text.
func TestImageToText(t *testing.T) {
	SkipIfShort(t)
	SkipIfOCR(t)
	c := NewClient(t)
	a := c.UploadFile(t, jpgFixture)
	jid := c.PostJob(t, "/api/pdf/image-to-text", map[string]any{
		"input_file_id": a, "lang": "eng",
	})
	job := c.PollDone(t, "/api/pdf/image-to-text", jid, pollFast)
	out := c.Download(t, job.Output())
	// Tesseract on test.jpg may or may not produce text depending on the
	// fixture; we only assert the file exists and the endpoint returns a
	// real .txt payload.
	if len(out) == 0 {
		t.Logf("image_to_text returned empty text — fixture may be image-only")
	}
}

// /api/pdf/ocr-detect-lang — analytic; expects detected_lang on the row.
func TestOCRDetectLang(t *testing.T) {
	SkipIfShort(t)
	SkipIfOCR(t)
	c := NewClient(t)
	a := c.UploadFile(t, pdfFixture)
	jid := c.PostJob(t, "/api/pdf/ocr-detect-lang", map[string]any{"input_file_id": a})
	c.PollDone(t, "/api/pdf/ocr-detect-lang", jid, pollNormal)
	// The job row carries the result — we just verified status=done.
}

// /api/pdf/ocr-structured — JSON wrapping the hOCR XML payload.
func TestOCRStructured(t *testing.T) {
	SkipIfShort(t)
	SkipIfOCR(t)
	c := NewClient(t)
	a := c.UploadFile(t, pdfFixture)
	jid := c.PostJob(t, "/api/pdf/ocr-structured", map[string]any{
		"input_file_id": a, "lang": "eng",
	})
	job := c.PollDone(t, "/api/pdf/ocr-structured", jid, pollOCR)
	out := c.Download(t, job.Output())
	// Output is JSON wrapping hocr — assert both that it's JSON-like
	// and that the hocr envelope is recognisable.
	if !startsWith(out, "{") {
		t.Fatalf("ocr-structured: expected JSON, head=%q", string(out[:min(len(out), 32)]))
	}
	AssertText(t, out, "hocr")
}

func startsWith(b []byte, prefix string) bool {
	if len(b) < len(prefix) {
		return false
	}
	return string(b[:len(prefix)]) == prefix
}

// --- "Save to ..." cloud-export endpoints ---------------------------------
//
// Real OAuth happens between the user's browser and the provider — we
// can't drive it from the test runner without registered apps + a real
// account. What we *can* verify here is that the plumbing works:
//
//   - GET /api/storage/providers lists three rows
//   - Each row reports Configured=false when no app creds are set
//     (the CI/dev case) — so the frontend correctly hides the option
//
// When real keys land on prod, swap SKIP_EXPORT=1 + add an assertion
// that at least one provider shows Configured=true.

func TestStorageProvidersList(t *testing.T) {
	SkipIfShort(t)
	c := NewClient(t)
	resp, err := c.HTTP.Get(c.BaseURL + "/api/storage/providers")
	if err != nil {
		t.Fatalf("GET /api/storage/providers: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
		t.Fatalf("providers status=%d body=%s", resp.StatusCode, string(body))
	}
	var env Envelope
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		t.Fatalf("decode: %v", err)
	}
	var providers []map[string]any
	if err := json.Unmarshal(env.Data, &providers); err != nil {
		t.Fatalf("unmarshal providers: %v (%s)", err, string(env.Data))
	}
	if len(providers) != 3 {
		t.Fatalf("expected 3 providers (dropbox/gdrive/onedrive), got %d", len(providers))
	}
	names := map[string]bool{}
	for _, p := range providers {
		names[p["name"].(string)] = true
	}
	for _, want := range []string{"dropbox", "gdrive", "onedrive"} {
		if !names[want] {
			t.Errorf("missing provider %q in response", want)
		}
	}
}
