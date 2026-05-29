package api

import (
	"github.com/gin-gonic/gin"

	"convertpdfgo/api/handler"
)

// registerPDFRoutes mounts /api/pdf/* — every conversion / analysis
// / OCR endpoint. Optional auth: guests can run jobs within the
// 20-MB tier; registered users get 30 MB.
//
// Endpoints are paired (POST creates+enqueues, GET fetches status),
// grouped by intent (page ops, transforms, security, ...). Adding a
// new conversion is a one-line edit inside the matching sub-section.
func registerPDFRoutes(r *gin.Engine, h handler.Handler) {
	pdf := r.Group("/api/pdf")

	// --- Page operations (rearrange / shrink an existing PDF) -----------
	pdf.POST("/thumbnails", h.PDFThumbnails)
	pdf.POST("/merge", h.CreateMergeJob)
	pdf.GET("/merge/:id", h.GetMergeJob)
	pdf.POST("/split", h.CreateSplitJob)
	pdf.GET("/split/:id", h.GetSplitJob)
	pdf.POST("/removepage", h.CreateRemovePagesJob)
	pdf.GET("/removepage/:id", h.GetRemovePagesJob)
	pdf.POST("/extract", h.CreateExtractJob)
	pdf.GET("/extract/:id", h.GetExtractJob)
	pdf.POST("/compress", h.CreateCompressJob)
	pdf.GET("/compress/:id", h.GetCompressJob)

	// --- Visual transforms (geometry, decorations) ----------------------
	pdf.POST("/rotate", h.CreateRotateJob)
	pdf.GET("/rotate/:id", h.GetRotateJob)
	pdf.POST("/crop", h.CreateCropJob)
	pdf.GET("/crop/:id", h.GetCropJob)
	pdf.POST("/add-page-numbers", h.CreateAddPageNumbersJob)
	pdf.GET("/add-page-numbers/:id", h.GetAddPageNumbersJob)
	pdf.POST("/header-footer", h.AddHeaderFooter)
	pdf.GET("/header-footer/:id", h.GetHeaderFooterJob)
	pdf.POST("/qr-code", h.CreateQRCodeJob)
	pdf.GET("/qr-code/:id", h.GetQRCodeJob)

	// --- Security (encrypt / decrypt) -----------------------------------
	pdf.POST("/protect", h.CreateProtectJob)
	pdf.GET("/protect/:id", h.GetProtectJob)
	pdf.POST("/unlock", h.CreateUnlockJob)
	pdf.GET("/unlock/:id", h.GetUnlockJob)

	// --- Inspection / analysis ------------------------------------------
	pdf.POST("/inspect", h.CreateInspectJob)
	pdf.GET("/inspect/:id", h.GetInspectJob)
	pdf.POST("/detect-blank", h.CreateDetectBlankPagesJob)
	pdf.GET("/detect-blank/:id", h.GetDetectBlankPagesJob)

	// --- Image ↔ PDF ----------------------------------------------------
	pdf.POST("/jpg-to-pdf", h.CreateJPGToPDF)
	pdf.GET("/jpg-to-pdf/:id", h.GetJPGToPDFJob)
	pdf.POST("/pdf-to-jpg", h.CreatePDFToJPG)
	pdf.GET("/pdf-to-jpg/:id", h.GetPDFToJPG)

	// --- Office ↔ PDF (Gotenberg-backed forward direction) ---------------
	pdf.POST("/word-to-pdf", h.CreateWordToPDF)
	pdf.GET("/word-to-pdf/:id", h.GetWordToPDFJob)
	pdf.POST("/pdf-to-word", h.CreatePDFToWordJob)
	pdf.GET("/pdf-to-word/:id", h.GetPDFToWordJob)
	pdf.POST("/excel-to-pdf", h.CreateExcelToPDF)
	pdf.GET("/excel-to-pdf/:id", h.GetExcelToPDFJob)
	pdf.POST("/ppt-to-pdf", h.CreatePowerPointToPDF)
	pdf.GET("/ppt-to-pdf/:id", h.GetPowerPointToPDFJob)
	pdf.POST("/html-to-pdf", h.CreateHTMLToPDF)
	pdf.GET("/html-to-pdf/:id", h.GetHTMLToPDFJob)

	// --- PDF → Office/HTML (LibreOffice direct; Gotenberg can't) -------
	// Gotenberg's /forms/libreoffice/convert only goes Office→PDF, so
	// reverse direction is in-process via pkg/libreoffice +
	// service/pdf_to_{html,ppt,excel}.go.
	pdf.POST("/pdf-to-html", h.CreatePDFToHTMLJob)
	pdf.GET("/pdf-to-html/:id", h.GetPDFToHTMLJob)
	pdf.POST("/pdf-to-ppt", h.CreatePDFToPPTJob)
	pdf.GET("/pdf-to-ppt/:id", h.GetPDFToPPTJob)
	pdf.POST("/pdf-to-excel", h.CreatePDFToExcelJob)
	pdf.GET("/pdf-to-excel/:id", h.GetPDFToExcelJob)

	// --- OCR family (ocrmypdf + tesseract) ------------------------------
	// Tesseract data packs (eng/uzb/uzb_cyrl/rus/kir/osd) ship inside
	// the worker image — see Dockerfile.
	pdf.POST("/ocr", h.CreateOCRJob)
	pdf.GET("/ocr/:id", h.GetOCRJob)
	pdf.POST("/scan-to-pdf", h.CreateScanToPDFJob)
	pdf.GET("/scan-to-pdf/:id", h.GetScanToPDFJob)
	pdf.POST("/ocr-extract-text", h.CreateOCRExtractTextJob)
	pdf.GET("/ocr-extract-text/:id", h.GetOCRExtractTextJob)
	pdf.POST("/image-to-text", h.CreateImageToTextJob)
	pdf.GET("/image-to-text/:id", h.GetImageToTextJob)
	pdf.POST("/ocr-detect-lang", h.CreateOCRDetectLangJob)
	pdf.GET("/ocr-detect-lang/:id", h.GetOCRDetectLangJob)
	pdf.POST("/ocr-structured", h.CreateOCRStructuredJob)
	pdf.GET("/ocr-structured/:id", h.GetOCRStructuredJob)

	// --- Phase 1 features (PDF_FEATURES.md) -----------------------------
	// watermark, extract-text, n-up, url-to-pdf, metadata edit.
	pdf.POST("/watermark", h.CreateWatermarkJob)
	pdf.GET("/watermark/:id", h.GetWatermarkJob)
	pdf.POST("/extract-text", h.CreateExtractTextJob)
	pdf.GET("/extract-text/:id", h.GetExtractTextJob)
	pdf.POST("/nup", h.CreateNUpJob)
	pdf.GET("/nup/:id", h.GetNUpJob)
	pdf.POST("/url-to-pdf", h.CreateURLToPDFJob)
	pdf.GET("/url-to-pdf/:id", h.GetURLToPDFJob)
	pdf.POST("/metadata", h.CreateMetadataJob)
	pdf.GET("/metadata/:id", h.GetMetadataJob)

	// --- Sharing (public link with a token) -----------------------------
	pdf.POST("/share", h.CreateSharedLink)
	pdf.GET("/share/:token", h.GetSharedLink)
}
