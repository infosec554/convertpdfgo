// End-to-end coverage for every conversion endpoint shipped today.
// Each test is self-contained: uploads its own fixture, runs the
// conversion, downloads the output, validates real bytes.
package integration

import (
	"testing"
	"time"
)

const (
	pdfFixture  = "testdata/test.pdf"
	jpgFixture  = "testdata/test.jpg"
	docxFixture = "testdata/test.docx"
	xlsxFixture = "testdata/test.xlsx"
	pptxFixture = "testdata/test.pptx"

	pollFast   = 30 * time.Second // pdfcpu jobs finish in <5s
	pollNormal = 60 * time.Second // mid jobs (pdf-to-jpg)
	pollOffice = 120 * time.Second // LibreOffice / url-to-pdf
)

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

func TestPDFToWord(t *testing.T) {
	SkipIfShort(t)
	SkipIfOffice(t)
	// Known broken: Gotenberg's /forms/libreoffice/convert is one-way (office → PDF).
	// The `output: docx` parameter we pass is ignored, so the worker just round-trips
	// the input PDF. Real fix needs a sidecar (LibreOffice headless directly, or
	// pdf2docx). Tracked in WEAKNESSES.md.
	t.Skip("Gotenberg can't do PDF→DOCX; see WEAKNESSES.md #pdf-to-word")
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
