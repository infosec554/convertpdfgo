package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hibiken/asynq"

	"convertpdfgo/pkg/logger"
	"convertpdfgo/pkg/metrics"
	"convertpdfgo/service"
)

// Handlers wires queue tasks to the application's services.
type Handlers struct {
	services service.IServiceManager
	log      logger.ILogger
}

func NewHandlers(services service.IServiceManager, log logger.ILogger) *Handlers {
	return &Handlers{services: services, log: log}
}

// Register attaches every task type to its handler on the given mux.
func (h *Handlers) Register(mux *asynq.ServeMux) {
	mux.HandleFunc(TaskMerge, h.runByID("merge", func(ctx context.Context, id string) error { return h.services.Merge().Process(ctx, id) }))
	mux.HandleFunc(TaskSplit, h.runByID("split", func(ctx context.Context, id string) error { return h.services.Split().Process(ctx, id) }))
	mux.HandleFunc(TaskCompress, h.runByID("compress", func(ctx context.Context, id string) error { return h.services.Compress().Process(ctx, id) }))
	mux.HandleFunc(TaskExtractPage, h.runByID("extract_page", func(ctx context.Context, id string) error { return h.services.ExtractPage().Process(ctx, id) }))
	mux.HandleFunc(TaskRemovePage, h.runByID("remove_page", func(ctx context.Context, id string) error { return h.services.RemovePage().Process(ctx, id) }))
	mux.HandleFunc(TaskRotate, h.runByID("rotate", func(ctx context.Context, id string) error { return h.services.Rotate().Process(ctx, id) }))
	mux.HandleFunc(TaskCrop, h.runByID("crop", func(ctx context.Context, id string) error { return h.services.Crop().Process(ctx, id) }))
	mux.HandleFunc(TaskProtect, h.runByID("protect", func(ctx context.Context, id string) error { return h.services.Protect().Process(ctx, id) }))
	mux.HandleFunc(TaskAddPageNumber, h.runByID("add_page_number", func(ctx context.Context, id string) error { return h.services.AddPageNumber().Process(ctx, id) }))
	mux.HandleFunc(TaskHeaderFooter, h.runByID("header_footer", func(ctx context.Context, id string) error { return h.services.AddHeaderFooter().Process(ctx, id) }))
	mux.HandleFunc(TaskJPGToPDF, h.runByID("jpg_to_pdf", func(ctx context.Context, id string) error { return h.services.JPGToPDF().Process(ctx, id) }))
	mux.HandleFunc(TaskPDFToJPG, h.runByID("pdf_to_jpg", func(ctx context.Context, id string) error { return h.services.PDFToJPG().Process(ctx, id) }))
	mux.HandleFunc(TaskWordToPDF, h.runByID("word_to_pdf", func(ctx context.Context, id string) error { return h.services.WordToPDF().Process(ctx, id) }))
	mux.HandleFunc(TaskPDFToWord, h.runByID("pdf_to_word", func(ctx context.Context, id string) error { return h.services.PDFToWord().Process(ctx, id) }))
	mux.HandleFunc(TaskExcelToPDF, h.runByID("excel_to_pdf", func(ctx context.Context, id string) error { return h.services.ExcelToPDF().Process(ctx, id) }))
	mux.HandleFunc(TaskPowerPointToPDF, h.runByID("ppt_to_pdf", func(ctx context.Context, id string) error { return h.services.PowerPointToPDF().Process(ctx, id) }))
	mux.HandleFunc(TaskHTMLToPDF, h.runByID("html_to_pdf", func(ctx context.Context, id string) error { return h.services.HTMLToPDF().Process(ctx, id) }))
	mux.HandleFunc(TaskQRCode, h.runByID("qr_code", func(ctx context.Context, id string) error { return h.services.QRCode().Process(ctx, id) }))
	mux.HandleFunc(TaskDetectBlank, h.runByID("detect_blank", func(ctx context.Context, id string) error { return h.services.DetectBlank().Process(ctx, id) }))

	// Phase 1 features.
	mux.HandleFunc(TaskWatermark, h.runByID("watermark", func(ctx context.Context, id string) error { return h.services.Watermark().Process(ctx, id) }))
	mux.HandleFunc(TaskExtractText, h.runByID("extract_text", func(ctx context.Context, id string) error { return h.services.ExtractText().Process(ctx, id) }))
	mux.HandleFunc(TaskNUp, h.runByID("nup", func(ctx context.Context, id string) error { return h.services.NUp().Process(ctx, id) }))
	mux.HandleFunc(TaskURLToPDF, h.runByID("url_to_pdf", func(ctx context.Context, id string) error { return h.services.URLToPDF().Process(ctx, id) }))
	mux.HandleFunc(TaskMetadata, h.runByID("metadata", func(ctx context.Context, id string) error { return h.services.Metadata().Process(ctx, id) }))

	// Unlock needs the password from the task payload — handled separately.
	mux.HandleFunc(TaskUnlock, h.handleUnlock)
}

// runByID returns a handler that decodes a SimpleJobPayload and invokes fn(ctx, jobID).
// All structurally identical processing tasks reuse this wrapper. Records
// duration + outcome to Prometheus.
func (h *Handlers) runByID(name string, fn func(context.Context, string) error) asynq.HandlerFunc {
	return func(ctx context.Context, t *asynq.Task) error {
		var p SimpleJobPayload
		if err := json.Unmarshal(t.Payload(), &p); err != nil {
			return fmt.Errorf("unmarshal %s payload: %w: %w", name, err, asynq.SkipRetry)
		}
		done := metrics.ObserveJob("pdf:"+name, time.Now())
		err := fn(ctx, p.JobID)
		done(err)
		if err != nil {
			h.log.Error("worker: "+name+" failed", logger.String("job_id", p.JobID), logger.Error(err))
			return err
		}
		return nil
	}
}

func (h *Handlers) handleUnlock(ctx context.Context, t *asynq.Task) error {
	var p UnlockPayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return fmt.Errorf("unmarshal unlock payload: %w: %w", err, asynq.SkipRetry)
	}
	done := metrics.ObserveJob("pdf:unlock", time.Now())
	err := h.services.Unlock().Process(ctx, p.JobID, p.Password)
	done(err)
	if err != nil {
		h.log.Error("worker: unlock failed", logger.String("job_id", p.JobID), logger.Error(err))
		return err
	}
	return nil
}
