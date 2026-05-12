package queue

import (
	"encoding/json"
	"time"

	"github.com/hibiken/asynq"
)

// Task type identifiers. Workers register handlers against these strings.
const (
	TaskMerge           = "pdf:merge"
	TaskSplit           = "pdf:split"
	TaskCompress        = "pdf:compress"
	TaskExtractPage     = "pdf:extract_page"
	TaskRemovePage      = "pdf:remove_page"
	TaskRotate          = "pdf:rotate"
	TaskCrop            = "pdf:crop"
	TaskUnlock          = "pdf:unlock"
	TaskProtect         = "pdf:protect"
	TaskAddPageNumber   = "pdf:add_page_number"
	TaskHeaderFooter    = "pdf:header_footer"
	TaskJPGToPDF        = "pdf:jpg_to_pdf"
	TaskPDFToJPG        = "pdf:pdf_to_jpg"
	TaskWordToPDF       = "pdf:word_to_pdf"
	TaskPDFToWord       = "pdf:pdf_to_word"
	TaskPDFToHTML       = "pdf:pdf_to_html"
	TaskPDFToPPT        = "pdf:pdf_to_ppt"
	TaskPDFToExcel      = "pdf:pdf_to_excel"

	TaskOCR            = "pdf:ocr"
	TaskScanToPDF      = "pdf:scan_to_pdf"
	TaskOCRExtractText = "pdf:ocr_extract_text"
	TaskImageToText    = "pdf:image_to_text"
	TaskOCRDetectLang  = "pdf:ocr_detect_lang"
	TaskOCRStructured  = "pdf:ocr_structured"
	TaskExcelToPDF      = "pdf:excel_to_pdf"
	TaskPowerPointToPDF = "pdf:ppt_to_pdf"
	TaskHTMLToPDF       = "pdf:html_to_pdf"
	TaskQRCode          = "pdf:qr_code"
	TaskInspect         = "pdf:inspect"
	TaskDetectBlank     = "pdf:detect_blank"

	// Phase 1 features.
	TaskWatermark   = "pdf:watermark"
	TaskExtractText = "pdf:extract_text"
	TaskNUp         = "pdf:nup"
	TaskURLToPDF    = "pdf:url_to_pdf"
	TaskMetadata    = "pdf:metadata"
)

// MergePayload carries data needed to process a merge job in the worker.
// JobID is the row in merge_jobs; the worker re-loads input file IDs from it.
type MergePayload struct {
	JobID string `json:"job_id"`
}

// SimpleJobPayload is the common shape for jobs that only need a job ID
// (the worker fetches the rest from the database).
type SimpleJobPayload struct {
	JobID string `json:"job_id"`
}

// UnlockPayload carries the password alongside the job ID. The password is
// deliberately not stored in the database row.
type UnlockPayload struct {
	JobID    string `json:"job_id"`
	Password string `json:"password"`
}

// NewUnlockTask builds an asynq.Task for an unlock job that needs the password
// to be passed to the worker without persisting it.
func NewUnlockTask(jobID, password string) (*asynq.Task, []asynq.Option, error) {
	payload, err := json.Marshal(UnlockPayload{JobID: jobID, Password: password})
	if err != nil {
		return nil, nil, err
	}
	opts := append(retryDefaults(), asynq.Queue(QueueDefault))
	return asynq.NewTask(TaskUnlock, payload), opts, nil
}

// retryDefaults returns a sensible retry/backoff policy for processing tasks.
func retryDefaults() []asynq.Option {
	return []asynq.Option{
		asynq.MaxRetry(3),
		asynq.Timeout(10 * time.Minute),
		asynq.Retention(24 * time.Hour),
	}
}

// NewMergeTask builds an asynq.Task for a merge job. Caller picks the queue.
func NewMergeTask(jobID string) (*asynq.Task, []asynq.Option, error) {
	payload, err := json.Marshal(MergePayload{JobID: jobID})
	if err != nil {
		return nil, nil, err
	}
	opts := append(retryDefaults(), asynq.Queue(QueueDefault))
	return asynq.NewTask(TaskMerge, payload), opts, nil
}

// NewSimpleTask builds a generic task carrying only a job ID.
// Use for compress/split/rotate/etc. The handler reads the full job from DB.
func NewSimpleTask(taskType, jobID, queue string) (*asynq.Task, []asynq.Option, error) {
	payload, err := json.Marshal(SimpleJobPayload{JobID: jobID})
	if err != nil {
		return nil, nil, err
	}
	opts := append(retryDefaults(), asynq.Queue(queue))
	return asynq.NewTask(taskType, payload), opts, nil
}
