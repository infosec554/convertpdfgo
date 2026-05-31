package addheaderfooter

import (
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
)

func relaxedPDFConf() *model.Configuration {
	c := model.NewDefaultConfiguration()
	c.ValidationMode = model.ValidationRelaxed
	return c
}
