package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"convertpdfgo/api/models"
	"convertpdfgo/pkg/logger"
	"convertpdfgo/storage"
)

type contactRepo struct {
	db  *pgxpool.Pool
	log logger.ILogger
}

func NewContactRepo(db *pgxpool.Pool, log logger.ILogger) storage.IContactStorage {
	return &contactRepo{db: db, log: log}
}

func (r *contactRepo) Submit(ctx context.Context, m *models.ContactMessage) error {
	if m.ID == "" {
		m.ID = uuid.NewString()
	}
	const q = `
		INSERT INTO contact_messages
		    (id, user_id, name, email, subject, body, status, created_at)
		VALUES ($1, $2, $3, $4, NULLIF($5,''), $6, 'new', NOW())
	`
	var uid interface{}
	if m.UserID != nil && *m.UserID != "" {
		uid = *m.UserID
	}
	_, err := r.db.Exec(ctx, q, m.ID, uid, m.Name, m.Email, m.Subject, m.Body)
	if err != nil {
		return fmt.Errorf("contact.Submit: %w", err)
	}
	return nil
}

// List runs the dashboard query — filterable by status + ILIKE on
// name / email / subject. Returns (rows, total_matching) so the UI
// can render a page count.
func (r *contactRepo) List(ctx context.Context, f models.ContactListFilter) ([]models.ContactMessage, int, error) {
	if f.Limit <= 0 || f.Limit > 200 {
		f.Limit = 50
	}

	// Build the WHERE dynamically. Always carries `1=1` so appending
	// AND clauses below stays trivial.
	args := []interface{}{}
	where := "1=1"
	if f.Status != "" {
		args = append(args, f.Status)
		where += fmt.Sprintf(" AND status = $%d", len(args))
	}
	if q := strings.TrimSpace(f.Q); q != "" {
		args = append(args, "%"+q+"%")
		i := len(args)
		where += fmt.Sprintf(" AND (email ILIKE $%d OR name ILIKE $%d OR COALESCE(subject,'') ILIKE $%d)", i, i, i)
	}

	var total int
	if err := r.db.QueryRow(ctx,
		"SELECT COUNT(*) FROM contact_messages WHERE "+where, args...,
	).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("contact.List count: %w", err)
	}

	args = append(args, f.Limit, f.Offset)
	rows, err := r.db.Query(ctx, fmt.Sprintf(`
		SELECT id, user_id, name, email, COALESCE(subject,''), body, status,
		       replied_at, replied_by, created_at
		FROM contact_messages
		WHERE %s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d
	`, where, len(args)-1, len(args)), args...)
	if err != nil {
		return nil, 0, fmt.Errorf("contact.List query: %w", err)
	}
	defer rows.Close()

	out := make([]models.ContactMessage, 0, f.Limit)
	for rows.Next() {
		var m models.ContactMessage
		var uid, repliedBy sql.NullString
		var repliedAt sql.NullTime
		if err := rows.Scan(
			&m.ID, &uid, &m.Name, &m.Email, &m.Subject, &m.Body, &m.Status,
			&repliedAt, &repliedBy, &m.CreatedAt,
		); err != nil {
			r.log.Error("contact.List scan", logger.Error(err))
			continue
		}
		if uid.Valid {
			m.UserID = &uid.String
		}
		if repliedBy.Valid {
			m.RepliedBy = &repliedBy.String
		}
		if repliedAt.Valid {
			t := repliedAt.Time
			m.RepliedAt = &t
		}
		out = append(out, m)
	}
	return out, total, nil
}

// GetByID returns one message with its full reply thread. Used by
// the admin "view + reply" screen.
func (r *contactRepo) GetByID(ctx context.Context, id string) (*models.ContactMessage, error) {
	const q = `
		SELECT id, user_id, name, email, COALESCE(subject,''), body, status,
		       replied_at, replied_by, created_at
		FROM contact_messages
		WHERE id = $1
	`
	var m models.ContactMessage
	var uid, repliedBy sql.NullString
	var repliedAt sql.NullTime
	err := r.db.QueryRow(ctx, q, id).Scan(
		&m.ID, &uid, &m.Name, &m.Email, &m.Subject, &m.Body, &m.Status,
		&repliedAt, &repliedBy, &m.CreatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("contact.GetByID: not found")
		}
		return nil, fmt.Errorf("contact.GetByID: %w", err)
	}
	if uid.Valid {
		m.UserID = &uid.String
	}
	if repliedBy.Valid {
		m.RepliedBy = &repliedBy.String
	}
	if repliedAt.Valid {
		t := repliedAt.Time
		m.RepliedAt = &t
	}

	// Pull the replies in chronological order.
	rrows, err := r.db.Query(ctx, `
		SELECT id, contact_message_id, admin_user_id, body, sent_at
		FROM contact_replies WHERE contact_message_id = $1
		ORDER BY sent_at ASC
	`, id)
	if err != nil {
		return nil, fmt.Errorf("contact.GetByID replies: %w", err)
	}
	defer rrows.Close()
	for rrows.Next() {
		var rep models.ContactReply
		if err := rrows.Scan(&rep.ID, &rep.ContactMessageID, &rep.AdminUserID, &rep.Body, &rep.SentAt); err != nil {
			r.log.Error("contact.GetByID reply scan", logger.Error(err))
			continue
		}
		m.Replies = append(m.Replies, rep)
	}
	return &m, nil
}

// SaveReply persists a new reply row AND flips the parent message
// to status='replied'. The two writes share a transaction so the
// list view never shows a "replied" message with zero reply rows.
func (r *contactRepo) SaveReply(ctx context.Context, messageID, adminUserID, body string) (string, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return "", fmt.Errorf("contact.SaveReply begin: %w", err)
	}
	defer tx.Rollback(ctx)

	replyID := uuid.NewString()
	if _, err := tx.Exec(ctx, `
		INSERT INTO contact_replies (id, contact_message_id, admin_user_id, body)
		VALUES ($1, $2, $3, $4)
	`, replyID, messageID, adminUserID, body); err != nil {
		return "", fmt.Errorf("contact.SaveReply insert: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE contact_messages
		SET status='replied', replied_at=NOW(), replied_by=$2
		WHERE id=$1
	`, messageID, adminUserID); err != nil {
		return "", fmt.Errorf("contact.SaveReply mark replied: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return "", fmt.Errorf("contact.SaveReply commit: %w", err)
	}
	return replyID, nil
}

// UpdateStatus is the manual override (admin marks something as
// 'closed' or back to 'new' without writing a reply).
func (r *contactRepo) UpdateStatus(ctx context.Context, id, status string) error {
	const q = `UPDATE contact_messages SET status = $2 WHERE id = $1`
	_, err := r.db.Exec(ctx, q, id, status)
	if err != nil {
		return fmt.Errorf("contact.UpdateStatus: %w", err)
	}
	return nil
}

func (r *contactRepo) Delete(ctx context.Context, id string) error {
	_, err := r.db.Exec(ctx, `DELETE FROM contact_messages WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("contact.Delete: %w", err)
	}
	return nil
}
