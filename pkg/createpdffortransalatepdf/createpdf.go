package createpdffortransalatepdf

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	"github.com/go-resty/resty/v2"
	"github.com/jung-kurt/gofpdf"
	"rsc.io/pdf"
)

func CreatePDF(text string, outputPath string) error {
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.AddPage()
	pdf.SetFont("Arial", "", 12)
	pdf.MultiCell(190, 10, text, "", "", false)
	return pdf.OutputFileAndClose(outputPath)
}

func TranslateGoogleAPI(text, targetLang string) (string, error) {
	apiKey := os.Getenv("GOOGLE_TRANSLATE_API_KEY")
	if apiKey == "" {
		return "", fmt.Errorf("GOOGLE_TRANSLATE_API_KEY is not set")
	}

	client := resty.New()
	reqBody := map[string]interface{}{
		"q":      text,
		"target": targetLang,
		"format": "text",
	}

	type googleResp struct {
		Data struct {
			Translations []struct {
				TranslatedText string `json:"translatedText"`
			} `json:"translations"`
		} `json:"data"`
	}

	var result googleResp

	_, err := client.R().
		SetHeader("Content-Type", "application/json").
		SetQueryParam("key", apiKey).
		SetBody(reqBody).
		SetResult(&result).
		Post("https://translation.googleapis.com/language/translate/v2")

	if err != nil {
		return "", err
	}

	if len(result.Data.Translations) == 0 {
		return "", fmt.Errorf("no translations received")
	}

	return result.Data.Translations[0].TranslatedText, nil
}
func ExtractTextFromPDF(filePath string) (string, error) {
	r, err := pdf.Open(filePath)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	for i := 1; i <= r.NumPage(); i++ {
		p := r.Page(i)
		if p.V.IsNull() {
			continue
		}
	
		content := p.Content()
	
		for _, txt := range content.Text {
			buf.WriteString(txt.S)
			buf.WriteString(" ")
		}
	}
	return strings.TrimSpace(buf.String()), nil
}
