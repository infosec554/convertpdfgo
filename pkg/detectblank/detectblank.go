package detectblank

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// DetectBlankPages pdftotext yordamida sahifalardagi matnlarni olib, bo‘sh sahifalarni aniqlaydi
func DetectBlankPages(pdfPath string, pageCount int) ([]int, error) {
	var blankPages []int

	for i := 1; i <= pageCount; i++ {
		// pdftotext -f <page> -l <page> <file> - | text
		cmd := exec.Command("pdftotext", "-f", fmt.Sprint(i), "-l", fmt.Sprint(i), pdfPath, "-")
		var out bytes.Buffer
		cmd.Stdout = &out
		err := cmd.Run()
		if err != nil {
			return nil, fmt.Errorf("failed to extract page %d text: %w", i, err)
		}

		text := strings.TrimSpace(out.String())
		if text == "" {
			blankPages = append(blankPages, i)
		}
	}

	return blankPages, nil
}
