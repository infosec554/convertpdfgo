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

	if params.HeaderText != "" {
		wm, err := pdfcpu.ParseTextWatermarkDetails(params.HeaderText, "pos:top center, font:Helvetica, points:12", true, model.NewDefaultConfiguration().Unit)
		if err != nil {
			return fmt.Errorf("failed to parse header watermark: %w", err)
		}
		err = api.AddWatermarksFile(params.InputPath, params.OutputPath, []string{params.PageRange}, wm, conf)
		if err != nil {
			return fmt.Errorf("failed to add header: %w", err)
		}
		params.InputPath = params.OutputPath
	}

	if params.FooterText != "" {
		wm, err := pdfcpu.ParseTextWatermarkDetails(params.FooterText, "pos:bottom center, font:Helvetica, points:12", true, model.NewDefaultConfiguration().Unit)
		if err != nil {
			return fmt.Errorf("failed to parse footer watermark: %w", err)
		}
		err = api.AddWatermarksFile(params.InputPath, params.OutputPath, []string{params.PageRange}, wm, conf)
		if err != nil {
			return fmt.Errorf("failed to add footer: %w", err)
		}
	}

	if _, err := os.Stat(params.OutputPath); os.IsNotExist(err) {
		return fmt.Errorf("output file not created")
	}

	return nil
}
