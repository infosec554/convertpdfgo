package service

import (
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
)

// relaxedPDFConf returns a pdfcpu configuration that tolerates the kind
// of minor spec deviations real-world PDFs carry (binary garbage in name
// refs, oddly-encoded outlines, etc.). Strict mode rejected legitimate
// user uploads with "invalid Name ref detected" errors.
func relaxedPDFConf() *model.Configuration {
	c := model.NewDefaultConfiguration()
	c.ValidationMode = model.ValidationRelaxed
	return c
}
