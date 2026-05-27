package storage

import (
	"context"
	"time"

	"convertpdfgo/api/models"
)

type IStorage interface {
	User() IUserStorage
	OTP() IOTPStorage
	Role() IRoleStorage
	Redis() IRedisStorage
	AuthAudit() IAuthAuditStorage
	Permission() IPermissionStorage
	AdminStats() IAdminStatsStorage
	Close()
	Merge() IMergeStorage
	File() IFileStorage
	Split() ISplitStorage
	RemovePage() IRemovePageStorage
	ExtractPage() IExtractPageStorage

	Compress() ICompressStorage
	PDFToJPG() IPDFToJPGStorage
	Rotate() IRotateStorage
	AddPageNumber() IAddPageNumberStorage
	Crop() ICropPDFStorage
	Unlock() IUnlockPDFStorage
	Protect() IProtectStorage
	Stat() IStatStorage
	Log() ILogService
	JPGToPDF() IJPGToPDFStorage
	Inspect() IInspectStorage

	SharedLink() ISharedLinkStorage
	AddHeaderFooter() AddHeaderFooterStorage
	DetectBlankPages() IDetectBlankPagesStorage
	QRCode() IQRCodeStorage

	PDFToWord() IPDFToWordStorage
	WordToPDF() IWordToPDFStorage
	ExcelToPDF() IExcelToPDFStorage
	PowerPointToPDF() IPowerPointToPDFStorage
	HTMLToPDF() IHTMLToPDFStorage

	// Phase 1 features (PDF_FEATURES.md): watermark, extract-text, n-up,
	// url-to-pdf, metadata.
	Watermark() IWatermarkStorage
	ExtractText() IExtractTextStorage
	NUp() INUpStorage
	URLToPDF() IURLToPDFStorage
	Metadata() IMetadataStorage

	// PDF → office formats. word path lives above; html/ppt/excel were
	// added once LibreOffice landed in the runtime image.
	PDFToHTML() IPDFToHTMLStorage
	PDFToPPT() IPDFToPPTStorage
	PDFToExcel() IPDFToExcelStorage

	// OCR family (ocrmypdf + tesseract).
	OCR() IOCRStorage
	ScanToPDF() IScanToPDFStorage
	OCRExtractText() IOCRExtractTextStorage
	ImageToText() IImageToTextStorage
	OCRDetectLang() IOCRDetectLangStorage
	OCRStructured() IOCRStructuredStorage

	// Cloud-export OAuth tokens (Dropbox / GDrive / OneDrive).
	StorageToken() IStorageTokenStorage

	// User → admin contact inbox + admin reply thread.
	Contact() IContactStorage

	// PDF Chat AI — RAG-based chat with uploaded PDFs (migration 0017).
	Chat() IChatStorage
}

// IChatStorage backs the chat_sessions / chat_messages / pdf_chunks /
// chat_suggestions tables created by migration 0017. All vector ops go
// through pgvector — postgres must run an image that ships the extension.
type IChatStorage interface {
	// Sessions
	CreateSession(ctx context.Context, s *models.ChatSession) error
	GetSession(ctx context.Context, id string) (*models.ChatSession, error)
	UpdateSessionIndexing(ctx context.Context, id, status, errMsg, title, summary string, pageCount, chunkCount int) error
	ListSessions(ctx context.Context, userID *string, limit int) ([]models.ChatSession, error)
	DeleteSession(ctx context.Context, id string) error

	// Messages
	AppendMessage(ctx context.Context, m *models.ChatMessage) error
	ListMessages(ctx context.Context, sessionID string, limit int) ([]models.ChatMessage, error)

	// Suggested questions (rendered in the sidebar prompts)
	SaveSuggestions(ctx context.Context, sessionID string, qs []string) error
	GetSuggestions(ctx context.Context, sessionID string) ([]string, error)

	// PDF chunks + vector search
	SaveChunks(ctx context.Context, sessionID string, chunks []models.PDFChunk, embeddings [][]float32) error
	SearchChunks(ctx context.Context, sessionID string, queryEmbedding []float32, limit int) ([]models.PDFChunkWithScore, error)

	// SetSessionFileID swaps the session's underlying file_id — used
	// when the indexer converts a non-PDF upload (Word/Excel/PPT) to a
	// PDF and wants future PDF-viewer requests to fetch the converted
	// version, not the original Office doc.
	SetSessionFileID(ctx context.Context, sessionID, fileID string) error
}

// IContactStorage holds the contact_messages + contact_replies
// tables (migration 0015). Submit is open to guests; the rest is
// admin-only and gated by the contacts:* permissions.
type IContactStorage interface {
	Submit(ctx context.Context, m *models.ContactMessage) error
	List(ctx context.Context, f models.ContactListFilter) ([]models.ContactMessage, int, error)
	GetByID(ctx context.Context, id string) (*models.ContactMessage, error)
	SaveReply(ctx context.Context, messageID, adminUserID, body string) (string, error)
	UpdateStatus(ctx context.Context, id, status string) error
	Delete(ctx context.Context, id string) error
}

// IStorageTokenStorage backs the "Save to ..." export targets — one
// row per (user_id, provider). Reauthenticating overwrites in place
// via the unique index on those two columns.
type IStorageTokenStorage interface {
	Upsert(ctx context.Context, t *models.UserStorageToken) error
	Get(ctx context.Context, userID, provider string) (*models.UserStorageToken, error)
	ListByUser(ctx context.Context, userID string) ([]models.UserStorageToken, error)
	Delete(ctx context.Context, userID, provider string) error
	UpdateAccessOnly(ctx context.Context, userID, provider, accessToken string, expiresAt *time.Time) error
}

type IOCRStorage interface {
	Create(ctx context.Context, job *models.OCRJob) error
	GetByID(ctx context.Context, id string) (*models.OCRJob, error)
	Update(ctx context.Context, job *models.OCRJob) error
}

type IScanToPDFStorage interface {
	Create(ctx context.Context, job *models.ScanToPDFJob) error
	GetByID(ctx context.Context, id string) (*models.ScanToPDFJob, error)
	Update(ctx context.Context, job *models.ScanToPDFJob) error
}

type IOCRExtractTextStorage interface {
	Create(ctx context.Context, job *models.OCRExtractTextJob) error
	GetByID(ctx context.Context, id string) (*models.OCRExtractTextJob, error)
	Update(ctx context.Context, job *models.OCRExtractTextJob) error
}

type IImageToTextStorage interface {
	Create(ctx context.Context, job *models.ImageToTextJob) error
	GetByID(ctx context.Context, id string) (*models.ImageToTextJob, error)
	Update(ctx context.Context, job *models.ImageToTextJob) error
}

type IOCRDetectLangStorage interface {
	Create(ctx context.Context, job *models.OCRDetectLangJob) error
	GetByID(ctx context.Context, id string) (*models.OCRDetectLangJob, error)
	Update(ctx context.Context, job *models.OCRDetectLangJob) error
}

type IOCRStructuredStorage interface {
	Create(ctx context.Context, job *models.OCRStructuredJob) error
	GetByID(ctx context.Context, id string) (*models.OCRStructuredJob, error)
	Update(ctx context.Context, job *models.OCRStructuredJob) error
}

type IPDFToHTMLStorage interface {
	Create(ctx context.Context, job *models.PDFToHTMLJob) error
	GetByID(ctx context.Context, id string) (*models.PDFToHTMLJob, error)
	Update(ctx context.Context, job *models.PDFToHTMLJob) error
}

type IPDFToPPTStorage interface {
	Create(ctx context.Context, job *models.PDFToPPTJob) error
	GetByID(ctx context.Context, id string) (*models.PDFToPPTJob, error)
	Update(ctx context.Context, job *models.PDFToPPTJob) error
}

type IPDFToExcelStorage interface {
	Create(ctx context.Context, job *models.PDFToExcelJob) error
	GetByID(ctx context.Context, id string) (*models.PDFToExcelJob, error)
	Update(ctx context.Context, job *models.PDFToExcelJob) error
}

type IUserStorage interface {
	Create(ctx context.Context, req models.CreateUser) (string, error)
	GetForLoginByEmail(ctx context.Context, email string) (models.LoginUser, error)
	GetByID(ctx context.Context, id string) (*models.User, error)

	// Account-management methods used by /auth/* endpoints.
	ExistsByEmail(ctx context.Context, email string) (bool, error)
	UpdatePassword(ctx context.Context, id, hashedPassword string) error
	UpdateProfile(ctx context.Context, id, name string) error

	// Admin-only mutators.
	UpdateRole(ctx context.Context, id, role string) error
	UpdateStatus(ctx context.Context, id, status string) error
	List(ctx context.Context, filter models.UserListFilter) ([]models.User, int, error)

	// Invite flow.
	//   CreatePending writes a status='pending' row (no password) — call
	//   from POST /admin/users/invite. Idempotent for repeat invites
	//   to the same email; errors when the email already maps to an
	//   active or blocked account.
	//   ActivatePending flips pending → active and sets the password
	//   hash. Called from POST /auth/accept-invite.
	CreatePending(ctx context.Context, email, role string) (string, error)
	ActivatePending(ctx context.Context, id, passwordHash string) error
}

// IPermissionStorage maps role names to wire-format permission strings
// (e.g. "users:read"). Used by RequirePermission middleware.
type IPermissionStorage interface {
	ListPermissionsForRole(ctx context.Context, role string) ([]string, error)
	ListAllPermissions(ctx context.Context) ([]models.Permission, error)
}

// IAuthAuditStorage records auth events (login, logout, password change, ...)
// for incident response and "why am I locked out" debugging.
type IAuthAuditStorage interface {
	Record(ctx context.Context, event models.AuthAudit) error
	ListByUser(ctx context.Context, userID string, limit int) ([]models.AuthAudit, error)

	// ListSuspicious returns emails that have at least minFails login_fail
	// events in the last `withinMinutes`. Used by /admin/audit/suspicious.
	ListSuspicious(ctx context.Context, withinMinutes, minFails int) ([]models.SuspiciousLogin, error)

	// ListRecent returns the latest events across all users, newest first.
	// Powers the admin dashboard's "recent activity" feed.
	ListRecent(ctx context.Context, limit int) ([]models.AuthAudit, error)
}

// IAdminStatsStorage hosts dashboard aggregates that don't belong to any
// single domain repo (cross-cutting counts).
type IAdminStatsStorage interface {
	// UsersByDay returns one row per day for the last `days` days, with
	// the count of users registered on that day.
	UsersByDay(ctx context.Context, days int) ([]models.DailyCount, error)

	// TopUsersByDisk lists the heaviest disk consumers — joined with users
	// to surface the email when available.
	TopUsersByDisk(ctx context.Context, limit int) ([]models.TopUserDisk, error)

	// UserBuckets returns the dashboard's user-count summary in a single
	// query (FILTER aggregates), replacing several separate User().List()
	// calls. Keys: total, active, blocked, admins, users.
	UserBuckets(ctx context.Context) (map[string]int, error)

	// PublicCounts aggregates the small set of numbers we're willing
	// to expose without auth — total registered users, total jobs
	// processed across the *_jobs tables, total files uploaded ever.
	// One query that UNIONs SELECT COUNT(*)s; the result is cached
	// at the service layer so polling the landing page doesn't
	// hammer the DB.
	PublicCounts(ctx context.Context) (models.PublicCounts, error)
}

type IOTPStorage interface {
	Create(ctx context.Context, email string, code string, expiresAt time.Time) (string, error)
	GetUnconfirmedByID(ctx context.Context, id string) (email string, code string, expiresAt time.Time, err error)
	UpdateStatusToConfirmed(ctx context.Context, id string) error
	GetByIDAndEmail(ctx context.Context, id string, email string) (bool, error)
}

type IRoleStorage interface {
	Create(ctx context.Context, name string, createdBy string) (string, error)
	Update(ctx context.Context, id, name string) error
	GetAll(ctx context.Context) ([]models.Role, error)
	Exists(ctx context.Context, id string) (bool, error)
}

type IRedisStorage interface {
	SetX(ctx context.Context, key string, value interface{}, duration time.Duration) error
	Get(ctx context.Context, key string) (string, error)

	// Keys runs a SCAN-based key listing for the given pattern (e.g.
	// "login_fail:*"). Use sparingly — it walks the keyspace; not for
	// hot paths. Returned at most maxKeys (hard-capped at 10000).
	Keys(ctx context.Context, pattern string, maxKeys int) ([]string, error)

	// Del removes a key. Returns nil if the key didn't exist.
	Del(ctx context.Context, key string) error
}

type IFileStorage interface {
	Save(ctx context.Context, file models.File) (string, error)
	GetByID(ctx context.Context, id string) (models.File, error)
	Delete(ctx context.Context, id string) error
	ListByUser(ctx context.Context, userID string) ([]models.File, error)

	GetOldFiles(ctx context.Context, olderThanDays int) ([]models.OldFile, error)
	DeleteByID(ctx context.Context, id string) error

	// FindBySHA256 returns the most recent file with the given hash that
	// belongs to the same owner as the incoming upload, or (zero,
	// pgx.ErrNoRows) if there's no match. Scoping by user prevents the
	// dedup short-circuit from cross-pollinating ownership — an
	// authenticated user uploading the same bytes a guest happened to
	// share earlier must NOT inherit the guest row, otherwise their
	// "private" file is silently world-readable. Pass nil for guest
	// uploads (matches the IS NULL slice); pass &userID for owned
	// uploads (matches that user's slice only).
	FindBySHA256(ctx context.Context, sha string, userID *string) (models.File, error)

	// GetGuestFilesOlderThanMinutes returns guest (NULL user_id) files
	// uploaded more than N minutes ago. Used by the fast cleaner so the DB
	// doesn't accumulate one-shot anonymous uploads.
	GetGuestFilesOlderThanMinutes(ctx context.Context, minutes int) ([]models.OldFile, error)

	// GetFilesOlderThanMinutes is the *unfiltered* sibling — covers both
	// guest and registered uploads. Powers the universal 5-minute privacy
	// cleaner: every file (input + output) disappears once the user has had
	// time to download it. Migration 0008 made the *_jobs FKs CASCADE so
	// deleting from `files` automatically purges referencing job rows.
	GetFilesOlderThanMinutes(ctx context.Context, minutes int) ([]models.OldFile, error)

	// ListAll is the admin-side paginated listing. Filter values are
	// optional; empty = "any". Returns the page slice and the total
	// matching rows so the UI can render a page count.
	ListAll(ctx context.Context, filter models.AdminFileFilter) ([]models.File, int, error)
}
type IMergeStorage interface {
	Create(ctx context.Context, job *models.MergeJob) error
	GetByID(ctx context.Context, id string) (*models.MergeJob, error)
	AddInputFiles(ctx context.Context, jobID string, fileIDs []string) error
	GetInputFiles(ctx context.Context, jobID string) ([]string, error)
	Update(ctx context.Context, job *models.MergeJob) error // 👉 YANGI QATOR

}

type ISplitStorage interface {
	Create(ctx context.Context, job *models.SplitJob) error
	GetByID(ctx context.Context, id string) (*models.SplitJob, error)
	UpdateOutputFiles(ctx context.Context, jobID string, outputIDs []string) error
}

type IRemovePageStorage interface {
	Create(ctx context.Context, job *models.RemoveJob) error
	Update(ctx context.Context, job *models.RemoveJob) error
	GetByID(ctx context.Context, id string) (*models.RemoveJob, error)
}

type IExtractPageStorage interface {
	Create(ctx context.Context, job *models.ExtractJob) error
	Update(ctx context.Context, job *models.ExtractJob) error
	GetByID(ctx context.Context, id string) (*models.ExtractJob, error)
}

type IOrganizeStorage interface {
	Create(ctx context.Context, job *models.OrganizeJob) error
	Update(ctx context.Context, job *models.OrganizeJob) error
	GetByID(ctx context.Context, id string) (*models.OrganizeJob, error)
}

type ICompressStorage interface {
	Create(ctx context.Context, job *models.CompressJob) error
	Update(ctx context.Context, job *models.CompressJob) error
	GetByID(ctx context.Context, id string) (*models.CompressJob, error)
}

type IPDFToJPGStorage interface {
	Create(ctx context.Context, job *models.PDFToJPGJob) error
	Update(ctx context.Context, job *models.PDFToJPGJob) error
	GetByID(ctx context.Context, id string) (*models.PDFToJPGJob, error)
}

type IRotateStorage interface {
	Create(ctx context.Context, job *models.RotateJob) error
	GetByID(ctx context.Context, id string) (*models.RotateJob, error)
	Update(ctx context.Context, job *models.RotateJob) error
}

type IAddPageNumberStorage interface {
	Create(ctx context.Context, job *models.AddPageNumberJob) error
	GetByID(ctx context.Context, id string) (*models.AddPageNumberJob, error)
	Update(ctx context.Context, job *models.AddPageNumberJob) error
}

type ICropPDFStorage interface {
	Create(ctx context.Context, job *models.CropPDFJob) error
	GetByID(ctx context.Context, id string) (*models.CropPDFJob, error)
	Update(ctx context.Context, job *models.CropPDFJob) error
}

type IUnlockPDFStorage interface {
	Create(ctx context.Context, job *models.UnlockPDFJob) error
	GetByID(ctx context.Context, id string) (*models.UnlockPDFJob, error)
	Update(ctx context.Context, job *models.UnlockPDFJob) error
}
type IProtectStorage interface {
	Create(ctx context.Context, job *models.ProtectPDFJob) error
	GetByID(ctx context.Context, id string) (*models.ProtectPDFJob, error)
	Update(ctx context.Context, job *models.ProtectPDFJob) error
}

type IStatStorage interface {
	GetUserStats(ctx context.Context, userID string) (models.UserStats, error)
}

type ILogService interface {
	GetLogsByJobID(ctx context.Context, jobID string) ([]models.Log, error)
}
type IJPGToPDFStorage interface {
	Create(ctx context.Context, job *models.JPGToPDFJob) error
	GetByID(ctx context.Context, id string) (*models.JPGToPDFJob, error)
	UpdateStatusAndOutput(ctx context.Context, id, status, outputFileID string) error
}

type IInspectStorage interface {
	Create(ctx context.Context, job *models.InspectJob) error
	GetByID(ctx context.Context, id string) (*models.InspectJob, error)
}

type ISharedLinkStorage interface {
	Create(ctx context.Context, req *models.SharedLink) error
	GetByToken(ctx context.Context, token string) (*models.SharedLink, error)
}

type AddHeaderFooterStorage interface {
	Create(ctx context.Context, job *models.AddHeaderFooterJob) error
	Update(ctx context.Context, job *models.AddHeaderFooterJob) error
	GetByID(ctx context.Context, id string) (*models.AddHeaderFooterJob, error)
}

type IDetectBlankPagesStorage interface {
	Create(ctx context.Context, job *models.DetectBlankPagesJob) error
	Update(ctx context.Context, job *models.DetectBlankPagesJob) error
	GetByID(ctx context.Context, id string) (*models.DetectBlankPagesJob, error)
}

type IQRCodeStorage interface {
	Create(ctx context.Context, job *models.QRCodeJob) error
	Update(ctx context.Context, job *models.QRCodeJob) error
	GetByID(ctx context.Context, id string) (*models.QRCodeJob, error)
}

type IPDFToWordStorage interface {
	Create(ctx context.Context, job *models.PDFToWordJob) error
	GetByID(ctx context.Context, id string) (*models.PDFToWordJob, error)
	Update(ctx context.Context, job *models.PDFToWordJob) error
}
type IWordToPDFStorage interface {
	Create(ctx context.Context, job *models.WordToPDFJob) error
	GetByID(ctx context.Context, id string) (*models.WordToPDFJob, error)
	Update(ctx context.Context, job *models.WordToPDFJob) error
}

type IExcelToPDFStorage interface {
	Create(ctx context.Context, job *models.ExcelToPDFJob) error
	GetByID(ctx context.Context, id string) (*models.ExcelToPDFJob, error)
	Update(ctx context.Context, job *models.ExcelToPDFJob) error
}

type IPowerPointToPDFStorage interface {
	Create(ctx context.Context, job *models.PowerPointToPDFJob) error
	GetByID(ctx context.Context, id string) (*models.PowerPointToPDFJob, error)
	Update(ctx context.Context, job *models.PowerPointToPDFJob) error
}

type IHTMLToPDFStorage interface {
	Create(ctx context.Context, job *models.HTMLToPDFJob) error
	GetByID(ctx context.Context, id string) (*models.HTMLToPDFJob, error)
	Update(ctx context.Context, job *models.HTMLToPDFJob) error
}

type IWatermarkStorage interface {
	Create(ctx context.Context, job *models.WatermarkJob) error
	GetByID(ctx context.Context, id string) (*models.WatermarkJob, error)
	Update(ctx context.Context, job *models.WatermarkJob) error
}

type IExtractTextStorage interface {
	Create(ctx context.Context, job *models.ExtractTextJob) error
	GetByID(ctx context.Context, id string) (*models.ExtractTextJob, error)
	Update(ctx context.Context, job *models.ExtractTextJob) error
}

type INUpStorage interface {
	Create(ctx context.Context, job *models.NUpJob) error
	GetByID(ctx context.Context, id string) (*models.NUpJob, error)
	Update(ctx context.Context, job *models.NUpJob) error
}

type IURLToPDFStorage interface {
	Create(ctx context.Context, job *models.URLToPDFJob) error
	GetByID(ctx context.Context, id string) (*models.URLToPDFJob, error)
	Update(ctx context.Context, job *models.URLToPDFJob) error
}

type IMetadataStorage interface {
	Create(ctx context.Context, job *models.MetadataJob) error
	GetByID(ctx context.Context, id string) (*models.MetadataJob, error)
	Update(ctx context.Context, job *models.MetadataJob) error
}
