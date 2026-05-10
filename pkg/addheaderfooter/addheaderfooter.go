package addheaderfooter

import (
	"fmt"
	"os"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
)

type AddHeaderFooterParams struct {
	InputPath  string
	OutputPath string
	HeaderText string
	FooterText string
	PageRange  string // "all", "1-3" va hokazo
}

func AddHeaderFooterPDF(params AddHeaderFooterParams) error {
	conf := model.NewDefaultConfiguration()

	// pdfcpu's anchor codes: tl|tc|tr|c|bl|bc|br. The "top center" / "bottom
	// center" English form was rejected by ParseTextWatermarkDetails — switch
	// to the abbreviation pdfcpu actually accepts.
	if params.HeaderText != "" {
		wm, err := pdfcpu.ParseTextWatermarkDetails(params.HeaderText, "pos:tc, font:Helvetica, points:12", true, model.NewDefaultConfiguration().Unit)
		if err != nil {
			return fmt.Errorf("failed to parse header watermark: %w", err)
		}
		err = api.AddWatermarksFile(params.InputPath, params.OutputPath, pageSelection(params.PageRange), wm, conf)
		if err != nil {
			return fmt.Errorf("failed to add header: %w", err)
		}
		params.InputPath = params.OutputPath
	}

	if params.FooterText != "" {
		wm, err := pdfcpu.ParseTextWatermarkDetails(params.FooterText, "pos:bc, font:Helvetica, points:12", true, model.NewDefaultConfiguration().Unit)
		if err != nil {
			return fmt.Errorf("failed to parse footer watermark: %w", err)
		}
		err = api.AddWatermarksFile(params.InputPath, params.OutputPath, pageSelection(params.PageRange), wm, conf)
		if err != nil {
			return fmt.Errorf("failed to add footer: %w", err)
		}
	}

	if _, err := os.Stat(params.OutputPath); os.IsNotExist(err) {
		return fmt.Errorf("output file not created")
	}

	return nil
}

// pageSelection turns a user-facing page string into pdfcpu's []string
// form. nil == "every page" — passing []string{"all"} would have pdfcpu
// Atoi-fail trying to parse the literal word "all".
func pageSelection(s string) []string {
	if s == "" || s == "all" {
		return nil
	}
	return []string{s}
}
