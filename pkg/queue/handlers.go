package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hibiken/asynq"

	"convertpdfgo/pkg/logger"
	"convertpdfgo/pkg/metrics"
	"convertpdfgo/pkg/notify"
	"convertpdfgo/service"
)

// Handlers wires queue tasks to the application's services.
type Handlers struct {
	services service.IServiceManager
	log      logger.ILogger
	bus      notify.Bus // notify.Noop when realtime disabled
}

func NewHandlers(services service.IServiceManager, log logger.ILogger, bus notify.Bus) *Handlers {
	if bus == nil {
		bus = notify.Noop{}
	}
	return &Handlers{services: services, log: log, bus: bus}
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
	mux.HandleFunc(TaskPDFToHTML, h.runByID("pdf_to_html", func(ctx context.Context, id string) error { return h.services.PDFToHTML().Process(ctx, id) }))
	mux.HandleFunc(TaskPDFToPPT, h.runByID("pdf_to_ppt", func(ctx context.Context, id string) error { return h.services.PDFToPPT().Process(ctx, id) }))
	mux.HandleFunc(TaskPDFToExcel, h.runByID("pdf_to_excel", func(ctx context.Context, id string) error { return h.services.PDFToExcel().Process(ctx, id) }))

	mux.HandleFunc(TaskOCR, h.runByID("ocr", func(ctx context.Context, id string) error { return h.services.OCR().Process(ctx, id) }))
	mux.HandleFunc(TaskScanToPDF, h.runByID("scan_to_pdf", func(ctx context.Context, id string) error { return h.services.ScanToPDF().Process(ctx, id) }))
	mux.HandleFunc(TaskOCRExtractText, h.runByID("ocr_extract_text", func(ctx context.Context, id string) error { return h.services.OCRExtractText().Process(ctx, id) }))
	mux.HandleFunc(TaskImageToText, h.runByID("image_to_text", func(ctx context.Context, id string) error { return h.services.ImageToText().Process(ctx, id) }))
	mux.HandleFunc(TaskOCRDetectLang, h.runByID("ocr_detect_lang", func(ctx context.Context, id string) error { return h.services.OCRDetectLang().Process(ctx, id) }))
	mux.HandleFunc(TaskOCRStructured, h.runByID("ocr_structured", func(ctx context.Context, id string) error { return h.services.OCRStructured().Process(ctx, id) }))
	mux.HandleFunc(TaskExcelToPDF, h.runByID("excel_to_pdf", func(ctx context.Context, id string) error { return h.services.ExcelToPDF().Process(ctx, id) }))
	mux.HandleFunc(TaskPowerPointToPDF, h.runByID("ppt_to_pdf", func(ctx context.Context, id string) error { return h.services.PowerPointToPDF().Process(ctx, id) }))
	mux.HandleFunc(TaskHTMLToPDF, h.runByID("html_to_pdf", func(ctx context.Context, id string) error { return h.services.HTMLToPDF().Process(ctx, id) }))
	mux.HandleFunc(TaskQRCode, h.runByID("qr_code", func(ctx context.Context, id string) error { return h.services.QRCode().Process(ctx, id) }))
	mux.HandleFunc(TaskDetectBlank, h.runByID("detect_blank", func(ctx context.Context, id string) error { return h.services.DetectBlank().Process(ctx, id) }))
	// NOTE: inspect runs synchronously inside handler.CreateInspectJob —
	// no enqueue happens, so no worker handler is needed for TaskInspect.

	// Phase 1 features.
	mux.HandleFunc(TaskWatermark, h.runByID("watermark", func(ctx context.Context, id string) error { return h.services.Watermark().Process(ctx, id) }))
	mux.HandleFunc(TaskExtractText, h.runByID("extract_text", func(ctx context.Context, id string) error { return h.services.ExtractText().Process(ctx, id) }))
	mux.HandleFunc(TaskNUp, h.runByID("nup", func(ctx context.Context, id string) error { return h.services.NUp().Process(ctx, id) }))
	mux.HandleFunc(TaskURLToPDF, h.runByID("url_to_pdf", func(ctx context.Context, id string) error { return h.services.URLToPDF().Process(ctx, id) }))
	mux.HandleFunc(TaskMetadata, h.runByID("metadata", func(ctx context.Context, id string) error { return h.services.Metadata().Process(ctx, id) }))

	// Unlock needs the password from the task payload — handled separately.
	mux.HandleFunc(TaskUnlock, h.handleUnlock)

	// PDF Chat AI indexing — extract → chunk → embed → save. Slow
	// (~10-30s for a 50-page PDF) but otherwise structurally identical
	// to the convert tasks, so runByID gives us the right metrics and
	// notifications for free.
	mux.HandleFunc(TaskChatIndex, h.runByID("chat_index", func(ctx context.Context, id string) error {
		return h.services.Chat().Process(ctx, id)
	}))
}

// runByID returns a handler that decodes a SimpleJobPayload and invokes fn(ctx, jobID).
// All structurally identical processing tasks reuse this wrapper. Records
// duration + outcome to Prometheus, and publishes a WS notification to the
// owning user on every terminal outcome (done | failed). Failed jobs also
// fan out to the admin broadcast channel for monitoring.
func (h *Handlers) runByID(name string, fn func(context.Context, string) error) asynq.HandlerFunc {
	return func(ctx context.Context, t *asynq.Task) error {
		var p SimpleJobPayload
		if err := json.Unmarshal(t.Payload(), &p); err != nil {
			return fmt.Errorf("unmarshal %s payload: %w: %w", name, err, asynq.SkipRetry)
		}
		done := metrics.ObserveJob("pdf:"+name, time.Now())
		err := fn(ctx, p.JobID)
		done(err)
		h.publishJobOutcome(ctx, name, p.JobID, p.UserID, err)
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
	h.publishJobOutcome(ctx, "unlock", p.JobID, p.UserID, err)
	if err != nil {
		h.log.Error("worker: unlock failed", logger.String("job_id", p.JobID), logger.Error(err))
		return err
	}
	return nil
}

// publishJobOutcome emits the JobUpdate event after a task finishes.
// Best-effort: a bus error (Redis down, etc.) is logged but doesn't fail
// the job — the user can still poll the REST endpoint for status. The
// admin fan-out only fires on failures so dashboards aren't flooded
// with done events on a healthy system.
func (h *Handlers) publishJobOutcome(ctx context.Context, jobType, jobID, userID string, jobErr error) {
	status := "done"
	errMsg := ""
	if jobErr != nil {
		status = "failed"
		errMsg = jobErr.Error()
	}
	env, mErr := notify.NewEnvelope(notify.TypeJobUpdate, notify.JobUpdate{
		UserID:  userID,
		JobID:   jobID,
		JobType: jobType,
		Status:  status,
		Error:   errMsg,
	})
	if mErr != nil {
		h.log.Warning("queue: build job-update envelope", logger.Error(mErr))
		return
	}
	if err := h.bus.PublishUser(ctx, userID, env); err != nil {
		h.log.Warning("queue: publish user event", logger.Error(err))
	}
	if jobErr != nil {
		adminEnv, _ := notify.NewEnvelope(notify.TypeJobFailed, notify.JobFailed{
			JobID:   jobID,
			JobType: jobType,
			Error:   errMsg,
		})
		if err := h.bus.PublishAdmin(ctx, adminEnv); err != nil {
			h.log.Warning("queue: publish admin event", logger.Error(err))
		}
	}
}
