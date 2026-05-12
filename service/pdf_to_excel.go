// PDF → Excel.
//
// Unlike pdf_to_word / pdf_to_html / pdf_to_ppt, this one *can't* lean
// on LibreOffice — its PDF importer only feeds Writer or Impress, and
// Calc has no equivalent. (`libreoffice --convert-to xlsx` from a PDF
// dies with "no export filter found".)
//
// We take a text-mode path instead: shell out to `pdftotext -layout`
// to preserve column whitespace, split the output into pages on the
// form-feed character (\f), then write each page as a sheet via
// excelize. Each text line becomes a row; tabs and runs of spaces
// produce separate cells so most table-shaped PDFs come out looking
// roughly right. True table-extraction (tabula/camelot quality) needs
// a different tool — tracked separately.
package service

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/xuri/excelize/v2"

	"convertpdfgo/api/models"
	"convertpdfgo/pkg/logger"
	"convertpdfgo/storage"
)

type PDFToExcelService interface {
	Create(ctx context.Context, req models.PDFToExcelRequest, userID *string) (string, error)
	Process(ctx context.Context, jobID string) error
	GetByID(ctx context.Context, id string) (*models.PDFToExcelJob, error)
}

type pdfToExcelService struct {
	stg storage.IStorage
	log logger.ILogger
}

func NewPDFToExcelService(stg storage.IStorage, log logger.ILogger) PDFToExcelService {
	return &pdfToExcelService{stg: stg, log: log}
}

func (s *pdfToExcelService) Create(ctx context.Context, req models.PDFToExcelRequest, userID *string) (string, error) {
	job := &models.PDFToExcelJob{
		ID:          uuid.NewString(),
		UserID:      userID,
		InputFileID: req.InputFileID,
		Status:      "pending",
		CreatedAt:   time.Now(),
	}
	if err := s.stg.PDFToExcel().Create(ctx, job); err != nil {
		return "", err
	}
	return job.ID, nil
}

func (s *pdfToExcelService) Process(ctx context.Context, jobID string) error {
	job, err := s.stg.PDFToExcel().GetByID(ctx, jobID)
	if err != nil {
		return fmt.Errorf("get job: %w", err)
	}
	file, err := s.stg.File().GetByID(ctx, job.InputFileID)
	if err != nil {
		s.markFailed(ctx, job)
		return fmt.Errorf("get input file: %w", err)
	}

	// `pdftotext -layout` keeps relative horizontal positioning so a
	// table-shaped page survives the extraction. Output goes to stdout
	// ("-" sink); we read the bytes directly.
	out, err := exec.CommandContext(ctx, "pdftotext", "-layout", file.FilePath, "-").Output()
	if err != nil {
		s.markFailed(ctx, job)
		return fmt.Errorf("pdftotext: %w", err)
	}

	outDir := "storage/pdf_to_excel"
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		s.markFailed(ctx, job)
		return fmt.Errorf("mkdir: %w", err)
	}
	outputFileID := uuid.NewString()
	outputPath := filepath.Join(outDir, outputFileID+".xlsx")

	if err := writeTextToXLSX(out, outputPath); err != nil {
		s.markFailed(ctx, job)
		return fmt.Errorf("xlsx: %w", err)
	}

	fi, err := os.Stat(outputPath)
	if err != nil {
		s.markFailed(ctx, job)
		return fmt.Errorf("stat: %w", err)
	}
	if _, err := s.stg.File().Save(ctx, models.File{
		ID:         outputFileID,
		UserID:     job.UserID,
		FileName:   filepath.Base(outputPath),
		FilePath:   outputPath,
		FileType:   "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		FileSize:   fi.Size(),
		UploadedAt: time.Now(),
	}); err != nil {
		s.markFailed(ctx, job)
		return fmt.Errorf("save output: %w", err)
	}
	job.OutputFileID = &outputFileID
	job.Status = "done"
	return s.stg.PDFToExcel().Update(ctx, job)
}

func (s *pdfToExcelService) markFailed(ctx context.Context, job *models.PDFToExcelJob) {
	job.Status = "failed"
	if err := s.stg.PDFToExcel().Update(ctx, job); err != nil {
		s.log.Error("pdf_to_excel: mark failed", logger.Error(err))
	}
}

func (s *pdfToExcelService) GetByID(ctx context.Context, id string) (*models.PDFToExcelJob, error) {
	return s.stg.PDFToExcel().GetByID(ctx, id)
}

// splitOnRuns splits a line on runs of 2+ whitespace chars (\t or
// multiple spaces). pdftotext -layout uses gaps to separate columns,
// so this yields one cell per visual column.
var multiSpace = regexp.MustCompile(`\s{2,}|\t`)

// writeTextToXLSX turns pdftotext output into a multi-sheet workbook
// (one sheet per PDF page). Form-feed (\f) is pdftotext's per-page
// separator.
func writeTextToXLSX(raw []byte, outPath string) error {
	pages := strings.Split(string(raw), "\f")

	f := excelize.NewFile()
	defer f.Close()

	// excelize creates Sheet1 by default; we rename it for the first
	// page and create fresh sheets for the rest.
	for i, page := range pages {
		// pdftotext often ends with an empty trailing page; skip.
		if i == len(pages)-1 && strings.TrimSpace(page) == "" {
			continue
		}
		sheetName := fmt.Sprintf("Page %d", i+1)
		if i == 0 {
			if err := f.SetSheetName("Sheet1", sheetName); err != nil {
				return fmt.Errorf("rename sheet: %w", err)
			}
		} else {
			if _, err := f.NewSheet(sheetName); err != nil {
				return fmt.Errorf("new sheet: %w", err)
			}
		}
		for rowIdx, line := range strings.Split(page, "\n") {
			if line == "" {
				continue
			}
			cells := multiSpace.Split(strings.TrimRight(line, " "), -1)
			for colIdx, cell := range cells {
				addr, err := excelize.CoordinatesToCellName(colIdx+1, rowIdx+1)
				if err != nil {
					return fmt.Errorf("cell coords: %w", err)
				}
				if err := f.SetCellValue(sheetName, addr, cell); err != nil {
					return fmt.Errorf("set cell: %w", err)
				}
			}
		}
	}

	if err := f.SaveAs(outPath); err != nil {
		return fmt.Errorf("save xlsx: %w", err)
	}
	return nil
}
