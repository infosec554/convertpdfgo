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
)

type Client interface {
	PDFToWord(ctx context.Context, srcPath, dstPath string) (int64, error)
	WordToPDF(ctx context.Context, srcPath, dstPath string) (int64, error)
	ExcelToPDF(ctx context.Context, srcPath, dstPath string) (int64, error)
	PowerPointToPDF(ctx context.Context, srcPath, dstPath string) (int64, error)
	HTMLToPDF(ctx context.Context, srcPath, dstPath string) (int64, error)
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
		if _, cerr := io.Copy(part, src); cerr != nil {
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

	n, err := io.Copy(dst, resp.Body)
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
