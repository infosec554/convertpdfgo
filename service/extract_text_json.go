package service

import (
	"encoding/json"
	"os"
	"strings"
)

// writePagesJSON splits pdftotext output on the form-feed character (0x0c)
// — pdftotext emits one form feed between pages — and writes a JSON array
// of {"page": N, "text": "..."} objects to outPath.
func writePagesJSON(outPath string, raw []byte) error {
	pages := strings.Split(string(raw), "\f")
	type page struct {
		Page int    `json:"page"`
		Text string `json:"text"`
	}
	out := make([]page, 0, len(pages))
	for i, p := range pages {
		if i == len(pages)-1 && strings.TrimSpace(p) == "" {
			break
		}
		out = append(out, page{Page: i + 1, Text: p})
	}
	f, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}
