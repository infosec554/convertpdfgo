package addbackground

import (
	"fmt"
	"os"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
)

// AddBackgroundParams holds input/output paths and background image options.
type AddBackgroundParams struct {
	InputPath           string
	OutputPath          string
	BackgroundImagePath string
	Opacity             float64
	Position            string // 'center', 'top-left', 'bottom-right', etc.
	PageRange           string // "all", "1-3" va hokazo
}

// AddBackgroundImage adds a background image to the PDF file.
func AddBackgroundImage(params AddBackgroundParams) error {
	conf := model.NewDefaultConfiguration()

	// Opacity va position ni watermark sifatida belgilash
	wmConf := fmt.Sprintf("opacity:%f, pos:%s", params.Opacity, params.Position)

	// Background uchun watermark yaratish:
	wm, err := pdfcpu.ParseImageWatermarkDetails(params.BackgroundImagePath, wmConf, true, model.NewDefaultConfiguration().Unit)
	if err != nil {
		return fmt.Errorf("failed to parse background watermark: %w", err)
	}

	// Orqa fon rasmi qo'shish
	err = api.AddWatermarksFile(params.InputPath, params.OutputPath, []string{params.PageRange}, wm, conf)
	if err != nil {
		return fmt.Errorf("failed to add background image: %w", err)
	}

	// Natija fayli yaratilganligini tekshirish
	if _, err := os.Stat(params.OutputPath); os.IsNotExist(err) {
		return fmt.Errorf("output file not created")
	}

	return nil
}
