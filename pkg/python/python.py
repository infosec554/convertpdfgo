import (
	"os/exec"
)

inputPDF := "storage/pdf_to_word/input.pdf"
outputDocx := "storage/pdf_to_word/output.docx"

cmd := exec.Command("python3", "convert.py", inputPDF, outputDocx)
err := cmd.Run()
if err != nil {
    log.Fatal("Konvertatsiya xatoligi:", err)
}
