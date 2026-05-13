package gotenberg

import (
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"convertpdfgo/pkg/bufpool"
)

type Client interface {
	PDFToWord(ctx context.Context, srcPath, dstPath string) (int64, error)
	WordToPDF(ctx context.Context, srcPath, dstPath string) (int64, error)
	ExcelToPDF(ctx context.Context, srcPath, dstPath string) (int64, error)
	PowerPointToPDF(ctx context.Context, srcPath, dstPath string) (int64, error)
	HTMLToPDF(ctx context.Context, srcPath, dstPath string) (int64, error)
	URLToPDF(ctx context.Context, dstPath string, opts URLToPDFOptions) (int64, error)
}

// URLToPDFOptions configures /forms/chromium/convert/url. Empty values fall
// back to Gotenberg defaults (A4 portrait, no wait).
type URLToPDFOptions struct {
	URL       string
	PageSize  string // "A4" | "Letter" | "Legal" | "" (default)
	Landscape bool
	WaitFor   string // "<n>s|m" → waitDelay, otherwise CSS selector → waitForExpression
}

type gotenbergClient struct {
	baseURL    string
	httpClient *http.Client
}

func New(url string) Client {
	return &gotenbergClient{
		baseURL: url,
		httpClient: &http.Client{
			Timeout: 2 * time.Minute,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 20,
				MaxConnsPerHost:     50,
				IdleConnTimeout:     90 * time.Second,
				TLSHandshakeTimeout: 10 * time.Second,
				ForceAttemptHTTP2:   true,
			},
		},
	}
}

// convert streams srcPath to Gotenberg endpoint and writes the response to dstPath.
// Both upload and download use io.Copy (no full file in RAM).
func (g *gotenbergClient) convert(
	ctx context.Context,
	endpoint, srcPath, dstPath, formFileName string,
	fields map[string]string,
) (int64, error) {
	if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
		return 0, fmt.Errorf("mkdir output dir: %w", err)
	}

	pr, pw := io.Pipe()
	mw := multipart.NewWriter(pw)

	go func() {
		var err error
		defer func() {
			if err != nil {
				_ = pw.CloseWithError(err)
				return
			}
			if cerr := mw.Close(); cerr != nil {
				_ = pw.CloseWithError(cerr)
				return
			}
			_ = pw.Close()
		}()

		src, oerr := os.Open(srcPath)
		if oerr != nil {
			err = fmt.Errorf("open src: %w", oerr)
			return
		}
		defer src.Close()

		part, perr := mw.CreateFormFile("files", formFileName)
		if perr != nil {
			err = fmt.Errorf("create form file: %w", perr)
			return
		}
		// 1 MiB CopyBuffer keeps syscalls down on big office files
		// (50 MB+ docx/xlsx). Slab comes from a sync.Pool — no per-job alloc.
		upBuf := bufpool.GetBytes()
		_, cerr := io.CopyBuffer(part, src, upBuf)
		bufpool.PutBytes(upBuf)
		if cerr != nil {
			err = fmt.Errorf("copy src: %w", cerr)
			return
		}
		for k, v := range fields {
			if werr := mw.WriteField(k, v); werr != nil {
				err = fmt.Errorf("write field %s: %w", k, werr)
				return
			}
		}
	}()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, g.baseURL+endpoint, pr)
	if err != nil {
		return 0, fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("gotenberg request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return 0, fmt.Errorf("gotenberg status %d: %s", resp.StatusCode, string(body))
	}

	dst, err := os.Create(dstPath)
	if err != nil {
		return 0, fmt.Errorf("create dst: %w", err)
	}
	defer dst.Close()

	dnBuf := bufpool.GetBytes()
	n, err := io.CopyBuffer(dst, resp.Body, dnBuf)
	bufpool.PutBytes(dnBuf)
	if err != nil {
		_ = os.Remove(dstPath)
		return 0, fmt.Errorf("write dst: %w", err)
	}
	return n, nil
}

func (g *gotenbergClient) PDFToWord(ctx context.Context, srcPath, dstPath string) (int64, error) {
	return g.convert(ctx, "/forms/libreoffice/convert", srcPath, dstPath, filepath.Base(srcPath), map[string]string{
		"output": "docx",
	})
}

func (g *gotenbergClient) WordToPDF(ctx context.Context, srcPath, dstPath string) (int64, error) {
	return g.convert(ctx, "/forms/libreoffice/convert", srcPath, dstPath, filepath.Base(srcPath), map[string]string{
		"waitTimeout": "30s",
	})
}

func (g *gotenbergClient) ExcelToPDF(ctx context.Context, srcPath, dstPath string) (int64, error) {
	return g.convert(ctx, "/forms/libreoffice/convert", srcPath, dstPath, filepath.Base(srcPath), map[string]string{
		"waitTimeout": "30s",
	})
}

func (g *gotenbergClient) PowerPointToPDF(ctx context.Context, srcPath, dstPath string) (int64, error) {
	return g.convert(ctx, "/forms/libreoffice/convert", srcPath, dstPath, filepath.Base(srcPath), map[string]string{
		"waitTimeout": "30s",
	})
}

func (g *gotenbergClient) HTMLToPDF(ctx context.Context, srcPath, dstPath string) (int64, error) {
	return g.convert(ctx, "/forms/chromium/convert/html", srcPath, dstPath, "index.html", nil)
}

// URLToPDF renders a public URL via Gotenberg's chromium converter. No file
// is uploaded — only multipart form fields. Page size maps to paperWidth/
// paperHeight in inches per Gotenberg's contract.
func (g *gotenbergClient) URLToPDF(ctx context.Context, dstPath string, opts URLToPDFOptions) (int64, error) {
	if opts.URL == "" {
		return 0, fmt.Errorf("url required")
	}
	if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
		return 0, fmt.Errorf("mkdir output dir: %w", err)
	}

	pr, pw := io.Pipe()
	mw := multipart.NewWriter(pw)

	go func() {
		var err error
		defer func() {
			if err != nil {
				_ = pw.CloseWithError(err)
				return
			}
			if cerr := mw.Close(); cerr != nil {
				_ = pw.CloseWithError(cerr)
				return
			}
			_ = pw.Close()
		}()

		fields := map[string]string{"url": opts.URL}
		if w, h, ok := paperSize(opts.PageSize); ok {
			fields["paperWidth"] = w
			fields["paperHeight"] = h
		}
		if opts.Landscape {
			fields["landscape"] = "true"
		}
		if opts.WaitFor != "" {
			if isDuration(opts.WaitFor) {
				fields["waitDelay"] = opts.WaitFor
			} else {
				fields["waitForExpression"] = "document.querySelector('" + opts.WaitFor + "') !== null"
			}
		}
		for k, v := range fields {
			if werr := mw.WriteField(k, v); werr != nil {
				err = fmt.Errorf("write field %s: %w", k, werr)
				return
			}
		}
	}()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, g.baseURL+"/forms/chromium/convert/url", pr)
	if err != nil {
		return 0, fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("gotenberg request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return 0, fmt.Errorf("gotenberg status %d: %s", resp.StatusCode, string(body))
	}

	dst, err := os.Create(dstPath)
	if err != nil {
		return 0, fmt.Errorf("create dst: %w", err)
	}
	defer dst.Close()

	dnBuf := bufpool.GetBytes()
	n, err := io.CopyBuffer(dst, resp.Body, dnBuf)
	bufpool.PutBytes(dnBuf)
	if err != nil {
		_ = os.Remove(dstPath)
		return 0, fmt.Errorf("write dst: %w", err)
	}
	return n, nil
}

// paperSize maps a friendly page-size name to Gotenberg's paperWidth/paperHeight
// in inches. Unknown names fall through to Gotenberg defaults.
func paperSize(name string) (string, string, bool) {
	switch name {
	case "A4":
		return "8.27", "11.7", true
	case "Letter":
		return "8.5", "11", true
	case "Legal":
		return "8.5", "14", true
	default:
		return "", "", false
	}
}

// isDuration reports whether s looks like "5s", "1.5s", "30m" etc.
func isDuration(s string) bool {
	if len(s) < 2 {
		return false
	}
	last := s[len(s)-1]
	if last != 's' && last != 'm' {
		return false
	}
	for _, c := range s[:len(s)-1] {
		if (c < '0' || c > '9') && c != '.' {
			return false
		}
	}
	return true
}
