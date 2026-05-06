package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"convertpdfgo/api/models"
	"convertpdfgo/pkg/logger"
)

type statsRepo struct {
	db  *pgxpool.Pool
	log logger.ILogger
}

func NewStatsRepo(db *pgxpool.Pool, log logger.ILogger) *statsRepo {
	return &statsRepo{
		db:  db,
		log: log,
	}
}

func (r *statsRepo) GetUserStats(ctx context.Context, userID string) (models.UserStats, error) {
	var stats models.UserStats

	query := `
	SELECT
		(SELECT COUNT(*) FROM merge_jobs WHERE user_id = $1) AS merged,
		(SELECT COUNT(*) FROM split_jobs WHERE user_id = $1) AS splitted,
		(SELECT COUNT(*) FROM remove_pages_jobs WHERE user_id = $1) AS removed_pages,
		(SELECT COUNT(*) FROM compress_jobs WHERE user_id = $1) AS compressed,
		(SELECT COUNT(*) FROM extract_pages_jobs WHERE user_id = $1) AS extracted,
		(SELECT COUNT(*) FROM organize_jobs WHERE user_id = $1) AS organized,
		(SELECT COUNT(*) FROM convert_to_pdf_jobs WHERE user_id = $1 AND conversion_type = 'jpg_to_pdf') AS jpg_to_pdf,
		(SELECT COUNT(*) FROM convert_from_pdf_jobs WHERE user_id = $1 AND conversion_type = 'pdf_to_jpg') AS pdf_to_jpg,
		(SELECT COUNT(*) FROM convert_from_pdf_jobs WHERE user_id = $1 AND conversion_type = 'pdf_to_word') AS pdf_to_word,
		(SELECT COUNT(*) FROM rotate_jobs WHERE user_id = $1) AS rotated,
		(SELECT COUNT(*) FROM add_page_number_jobs WHERE user_id = $1) AS page_numbered,
		(SELECT COUNT(*) FROM crop_pdf_jobs WHERE user_id = $1) AS cropped,
		(SELECT COUNT(*) FROM watermark_jobs WHERE user_id = $1) AS watermarked,
		(SELECT COUNT(*) FROM protect_jobs WHERE user_id = $1) AS protected,
		(SELECT COUNT(*) FROM unlock_jobs WHERE user_id = $1) AS unlocked;
	`

	err := r.db.QueryRow(ctx, query, userID).Scan(
		&stats.Merged,
		&stats.Splitted,
		&stats.RemovedPages,
		&stats.Compressed,
		&stats.Extracted,
		&stats.Organized,
		&stats.JPGToPDF,
		&stats.PDFToJPG,
		&stats.PDFToWord,
		&stats.Rotated,
		&stats.AddedPageNumbers,
		&stats.Cropped,
		&stats.Watermarked,
		&stats.Protected,
		&stats.Unlocked,
	)
	if err != nil {
		r.log.Error("failed to get user stats", logger.Error(err))
		return stats, err
	}

	return stats, nil
}
