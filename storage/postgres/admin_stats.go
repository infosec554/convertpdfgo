package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"convertpdfgo/api/models"
	"convertpdfgo/pkg/logger"
	"convertpdfgo/storage"
)

type adminStatsRepo struct {
	db  *pgxpool.Pool
	log logger.ILogger
}

func NewAdminStatsRepo(db *pgxpool.Pool, log logger.ILogger) storage.IAdminStatsStorage {
	return &adminStatsRepo{db: db, log: log}
}

// UsersByDay — generate_series guarantees a row for empty days too,
// so the resulting chart has no gaps.
func (r *adminStatsRepo) UsersByDay(ctx context.Context, days int) ([]models.DailyCount, error) {
	if days <= 0 || days > 365 {
		days = 30
	}
	const q = `
		WITH days AS (
			SELECT generate_series(
				current_date - ($1::int - 1),
				current_date,
				'1 day'::interval
			)::date AS day
		)
		SELECT to_char(d.day, 'YYYY-MM-DD'),
		       COALESCE(c.cnt, 0) AS cnt
		FROM days d
		LEFT JOIN (
			SELECT created_at::date AS day, COUNT(*) AS cnt
			FROM users
			WHERE created_at >= current_date - ($1::int - 1)
			GROUP BY 1
		) c ON c.day = d.day
		ORDER BY d.day
	`
	rows, err := r.db.Query(ctx, q, days)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]models.DailyCount, 0, days)
	for rows.Next() {
		var dc models.DailyCount
		if err := rows.Scan(&dc.Day, &dc.Count); err != nil {
			r.log.Error("admin_stats: scan day", logger.Error(err))
			continue
		}
		out = append(out, dc)
	}
	return out, nil
}

// UserBuckets aggregates the five summary counters in a single round-trip.
// Replaces the previous handler that issued FIVE User.List() calls (10 SQL
// queries — each List does a SELECT + COUNT(*)). One scan, FILTER does the
// rest.
func (r *adminStatsRepo) UserBuckets(ctx context.Context) (map[string]int, error) {
	const q = `
		SELECT
			COUNT(*)                                                AS total,
			COUNT(*) FILTER (WHERE status = 'active')               AS active,
			COUNT(*) FILTER (WHERE status = 'blocked')              AS blocked,
			COUNT(*) FILTER (WHERE role   = 'admin')                AS admins,
			COUNT(*) FILTER (WHERE role   = 'moderator')            AS moderators
		FROM users
	`
	var total, active, blocked, admins, moderators int
	if err := r.db.QueryRow(ctx, q).Scan(&total, &active, &blocked, &admins, &moderators); err != nil {
		return nil, err
	}
	return map[string]int{
		"total":      total,
		"active":     active,
		"blocked":    blocked,
		"admins":     admins,
		"moderators": moderators,
	}, nil
}

// TopUsersByDisk — left join so anonymous (guest) uploads still show up
// with email = ”. Limit hard-capped at 200 to keep the response small.
func (r *adminStatsRepo) TopUsersByDisk(ctx context.Context, limit int) ([]models.TopUserDisk, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	const q = `
		SELECT
			COALESCE(f.user_id::text, '')   AS user_id,
			COALESCE(u.email, '')           AS email,
			COUNT(*)                        AS file_count,
			COALESCE(SUM(f.file_size), 0)   AS total_size
		FROM files f
		LEFT JOIN users u ON u.id = f.user_id
		GROUP BY f.user_id, u.email
		ORDER BY total_size DESC
		LIMIT $1
	`
	rows, err := r.db.Query(ctx, q, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]models.TopUserDisk, 0, limit)
	for rows.Next() {
		var t models.TopUserDisk
		if err := rows.Scan(&t.UserID, &t.Email, &t.FileCount, &t.TotalSize); err != nil {
			r.log.Error("admin_stats: scan top disk", logger.Error(err))
			continue
		}
		out = append(out, t)
	}
	return out, nil
}

// PublicCounts runs three lightweight COUNTs against indexed tables.
// The big one is the SUM over every *_jobs table — listed by hand
// rather than read from information_schema so adding a new job
// shows up as a real diff here (good — keeps the metric honest).
// Status filter is 'done' so failed / pending jobs don't inflate the
// "files processed" headline on the landing page.
func (r *adminStatsRepo) PublicCounts(ctx context.Context) (models.PublicCounts, error) {
	const q = `
		WITH done_counts AS (
			SELECT COUNT(*) AS n FROM merge_jobs            WHERE status='done'
			UNION ALL SELECT COUNT(*) FROM split_jobs        WHERE status='done'
			UNION ALL SELECT COUNT(*) FROM remove_pages_jobs WHERE status='done'
			UNION ALL SELECT COUNT(*) FROM extract_pages_jobs WHERE status='done'
			UNION ALL SELECT COUNT(*) FROM compress_jobs     WHERE status='done'
			UNION ALL SELECT COUNT(*) FROM rotate_jobs       WHERE status='done'
			UNION ALL SELECT COUNT(*) FROM crop_pdf_jobs     WHERE status='done'
			UNION ALL SELECT COUNT(*) FROM add_page_number_jobs WHERE status='done'
			UNION ALL SELECT COUNT(*) FROM add_header_footer_jobs WHERE status='done'
			UNION ALL SELECT COUNT(*) FROM qr_code_jobs      WHERE status='done'
			UNION ALL SELECT COUNT(*) FROM protect_jobs      WHERE status='done'
			UNION ALL SELECT COUNT(*) FROM unlock_jobs       WHERE status='done'
			UNION ALL SELECT COUNT(*) FROM pdf_inspect_jobs  WHERE status='done'
			UNION ALL SELECT COUNT(*) FROM detect_blank_pages_jobs WHERE status='done'
			UNION ALL SELECT COUNT(*) FROM jpg_to_pdf_jobs   WHERE status='done'
			UNION ALL SELECT COUNT(*) FROM pdf_to_jpg_jobs   WHERE status='done'
			UNION ALL SELECT COUNT(*) FROM word_to_pdf_jobs  WHERE status='done'
			UNION ALL SELECT COUNT(*) FROM pdf_to_word_jobs  WHERE status='done'
			UNION ALL SELECT COUNT(*) FROM excel_to_pdf_jobs WHERE status='done'
			UNION ALL SELECT COUNT(*) FROM powerpoint_to_pdf_jobs WHERE status='done'
			UNION ALL SELECT COUNT(*) FROM html_to_pdf_jobs  WHERE status='done'
			UNION ALL SELECT COUNT(*) FROM watermark_jobs    WHERE status='done'
			UNION ALL SELECT COUNT(*) FROM extract_text_jobs WHERE status='done'
			UNION ALL SELECT COUNT(*) FROM nup_jobs          WHERE status='done'
			UNION ALL SELECT COUNT(*) FROM url_to_pdf_jobs   WHERE status='done'
			UNION ALL SELECT COUNT(*) FROM metadata_jobs     WHERE status='done'
			UNION ALL SELECT COUNT(*) FROM pdf_to_html_jobs  WHERE status='done'
			UNION ALL SELECT COUNT(*) FROM pdf_to_ppt_jobs   WHERE status='done'
			UNION ALL SELECT COUNT(*) FROM pdf_to_excel_jobs WHERE status='done'
			UNION ALL SELECT COUNT(*) FROM ocr_jobs          WHERE status='done'
			UNION ALL SELECT COUNT(*) FROM scan_to_pdf_jobs  WHERE status='done'
			UNION ALL SELECT COUNT(*) FROM ocr_extract_text_jobs WHERE status='done'
			UNION ALL SELECT COUNT(*) FROM image_to_text_jobs WHERE status='done'
			UNION ALL SELECT COUNT(*) FROM ocr_detect_lang_jobs WHERE status='done'
			UNION ALL SELECT COUNT(*) FROM ocr_structured_jobs WHERE status='done'
		)
		SELECT
		    (SELECT COUNT(*) FROM users)::int                  AS total_users,
		    (SELECT COUNT(*) FROM files)::int                  AS total_files_uploaded,
		    (SELECT COALESCE(SUM(n),0) FROM done_counts)::int  AS total_files_processed
	`
	var out models.PublicCounts
	if err := r.db.QueryRow(ctx, q).Scan(
		&out.TotalUsers, &out.TotalFilesUploaded, &out.TotalFilesProcessed,
	); err != nil {
		return models.PublicCounts{}, fmt.Errorf("admin_stats.PublicCounts: %w", err)
	}
	return out, nil
}
