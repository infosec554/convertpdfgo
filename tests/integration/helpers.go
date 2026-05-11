// Package integration drives the HTTP API end-to-end. Each test:
//
//  1. uploads its own fixtures (so the 5-min privacy cleaner can't eat them
//     mid-run regardless of how long the sweep takes);
//  2. POSTs the conversion;
//  3. polls until status=done|failed;
//  4. downloads the output via /file/{id}/download;
//  5. validates real bytes (PDF magic, page count, MIME-aware checks).
//
// Configuration:
//
//	BASE_URL   — defaults to http://127.0.0.1:8080
//	SKIP_OFFICE — set non-empty to skip LibreOffice-bound tests (slow, ~30s each)
//	SKIP_URL   — set non-empty to skip the url-to-pdf test (depends on egress)
package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Client is a thin HTTP wrapper that fails the test on transport errors.
type Client struct {
	BaseURL string
	HTTP    *http.Client
}

func NewClient(t *testing.T) *Client {
	t.Helper()
	base := os.Getenv("BASE_URL")
	if base == "" {
		base = "http://127.0.0.1:8080"
	}
	return &Client{BaseURL: base, HTTP: &http.Client{Timeout: 2 * time.Minute}}
}

// Envelope mirrors the API's outer response shape.
type Envelope struct {
	StatusCode  int             `json:"StatusCode"`
	Description string          `json:"Description"`
	Data        json.RawMessage `json:"Data"`
}

// JobStatus is the union of fields different endpoints put on a job row.
// pdf-to-jpg uses ZipFileID, split uses OutputFileIDs, the rest output_file_id.
type JobStatus struct {
	ID            string   `json:"id"`
	Status        string   `json:"status"`
	OutputFileID  *string  `json:"output_file_id"`
	OutputFileIDs []string `json:"output_file_ids"`
	ZipFileID     *string  `json:"zip_file_id"`
}

// Output returns whatever ID this endpoint exposed for the produced file.
// Empty string means "analytic endpoint" (inspect, detect-blank).
func (j JobStatus) Output() string {
	switch {
	case j.OutputFileID != nil && *j.OutputFileID != "":
		return *j.OutputFileID
	case j.ZipFileID != nil && *j.ZipFileID != "":
		return *j.ZipFileID
	case len(j.OutputFileIDs) > 0:
		return j.OutputFileIDs[0]
	}
	return ""
}

// UploadFile streams a fixture to /file/upload and returns its file_id.
func (c *Client) UploadFile(t *testing.T, path string) string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("upload: open %s: %v", path, err)
	}
	defer f.Close()

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	part, err := mw.CreateFormFile("file", filepath.Base(path))
	if err != nil {
		t.Fatalf("upload: create form: %v", err)
	}
	if _, err := io.Copy(part, f); err != nil {
		t.Fatalf("upload: copy: %v", err)
	}
	mw.Close()

	req, _ := http.NewRequest(http.MethodPost, c.BaseURL+"/file/upload", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	resp, err := c.HTTP.Do(req)
	if err != nil {
		t.Fatalf("upload http: %v", err)
	}
	defer resp.Body.Close()

	var env Envelope
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		t.Fatalf("upload decode: %v", err)
	}
	if env.StatusCode != http.StatusCreated && env.StatusCode != http.StatusOK {
		t.Fatalf("upload status=%d body=%s", env.StatusCode, string(env.Data))
	}
	var d struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(env.Data, &d); err != nil {
		t.Fatalf("upload data: %v (%s)", err, string(env.Data))
	}
	if d.ID == "" {
		t.Fatalf("upload returned empty id: %s", string(env.Data))
	}
	return d.ID
}

// PostJob marshals body, hits the endpoint, returns the created job ID.
// Accepts 200 (synchronous endpoints like inspect) and 202 (every async
// job creation). After WEAKNESSES.md #7/9 these are the only correct
// codes — drift here would surface as a test failure.
func (c *Client) PostJob(t *testing.T, path string, body any) string {
	t.Helper()
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("post: marshal: %v", err)
	}
	req, _ := http.NewRequest(http.MethodPost, c.BaseURL+path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		t.Fatalf("post %s: %v", path, err)
	}
	defer resp.Body.Close()

	var env Envelope
	raw, _ := io.ReadAll(resp.Body)
	if err := json.Unmarshal(raw, &env); err != nil {
		t.Fatalf("post %s decode: %v body=%s", path, err, string(raw))
	}
	if env.StatusCode != http.StatusAccepted && env.StatusCode != http.StatusOK {
		t.Fatalf("post %s status=%d (want 200 or 202) data=%s", path, env.StatusCode, string(env.Data))
	}
	var d struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(env.Data, &d); err != nil || d.ID == "" {
		t.Fatalf("post %s no id: %s", path, string(env.Data))
	}
	return d.ID
}

// PollDone polls /<path>/<jobID> until status=done or status=failed or timeout.
// Returns the final JobStatus; fails the test on timeout / failed status.
func (c *Client) PollDone(t *testing.T, path, jobID string, timeout time.Duration) JobStatus {
	t.Helper()
	deadline := time.Now().Add(timeout)
	tick := time.NewTicker(2 * time.Second)
	defer tick.Stop()

	var last JobStatus
	for {
		resp, err := c.HTTP.Get(fmt.Sprintf("%s%s/%s", c.BaseURL, path, jobID))
		if err != nil {
			t.Fatalf("poll http: %v", err)
		}
		raw, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		var env Envelope
		if err := json.Unmarshal(raw, &env); err == nil {
			if env.StatusCode == http.StatusOK {
				var js JobStatus
				if err := json.Unmarshal(env.Data, &js); err == nil {
					last = js
					switch js.Status {
					case "done":
						return js
					case "failed":
						t.Fatalf("%s/%s failed", path, jobID)
					}
				}
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("%s/%s timeout after %s — last status=%q", path, jobID, timeout, last.Status)
		}
		<-tick.C
	}
}

// Download pulls the raw bytes from /file/{id}/download.
func (c *Client) Download(t *testing.T, fileID string) []byte {
	t.Helper()
	resp, err := c.HTTP.Get(fmt.Sprintf("%s/file/%s/download", c.BaseURL, fileID))
	if err != nil {
		t.Fatalf("download http: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		t.Fatalf("download %s status=%d body=%s", fileID, resp.StatusCode, string(body))
	}
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("download read: %v", err)
	}
	return b
}

// --- assertions -----------------------------------------------------------

// AssertPDF panics if b doesn't start with %PDF or is smaller than minSize.
func AssertPDF(t *testing.T, b []byte, minSize int) {
	t.Helper()
	if len(b) < minSize {
		t.Fatalf("PDF too small: %d < %d", len(b), minSize)
	}
	if len(b) < 4 || string(b[:4]) != "%PDF" {
		head := b
		if len(head) > 32 {
			head = head[:32]
		}
		t.Fatalf("not a PDF, head=%q", string(head))
	}
}

// AssertText fails if b doesn't contain `needle`.
func AssertText(t *testing.T, b []byte, needle string) {
	t.Helper()
	if !bytes.Contains(b, []byte(needle)) {
		t.Fatalf("expected %q in output (first 200 bytes: %q)", needle, head(b, 200))
	}
}

// AssertZip fails if b doesn't look like a ZIP archive (PK\x03\x04 magic).
func AssertZip(t *testing.T, b []byte) {
	t.Helper()
	if len(b) < 4 || !bytes.Equal(b[:4], []byte{'P', 'K', 0x03, 0x04}) {
		t.Fatalf("not a ZIP, head=%q", head(b, 16))
	}
}

func head(b []byte, n int) []byte {
	if len(b) > n {
		return b[:n]
	}
	return b
}

// --- env gates ------------------------------------------------------------

// SkipIfShort honours `go test -short`.
func SkipIfShort(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in -short mode")
	}
}

// SkipIfOffice skips when SKIP_OFFICE is set (LibreOffice conversions are slow).
func SkipIfOffice(t *testing.T) {
	if os.Getenv("SKIP_OFFICE") != "" {
		t.Skip("skipping office conversion: SKIP_OFFICE set")
	}
}

// SkipIfURL skips when SKIP_URL is set (url-to-pdf depends on egress).
func SkipIfURL(t *testing.T) {
	if os.Getenv("SKIP_URL") != "" {
		t.Skip("skipping url-to-pdf: SKIP_URL set")
	}
}
