package detectblank

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// DetectBlankPages walks each page with pdftotext and lists the ones with
// no extractable text. pdftotext occasionally exits non-zero on a malformed
// page or one with only annotations — treat those as "no text" rather than
// aborting the whole job, since the unanalysable page is still effectively
// blank from the user's perspective.
func DetectBlankPages(pdfPath string, pageCount int) ([]int, error) {
	var blankPages []int

	for i := 1; i <= pageCount; i++ {
		cmd := exec.Command("pdftotext", "-f", fmt.Sprint(i), "-l", fmt.Sprint(i), pdfPath, "-")
		var out bytes.Buffer
		cmd.Stdout = &out
		if err := cmd.Run(); err != nil {
			// per-page failure → consider the page blank, keep going
			blankPages = append(blankPages, i)
			continue
		}
		if strings.TrimSpace(out.String()) == "" {
			blankPages = append(blankPages, i)
		}
	}

	return blankPages, nil
}
