package postgres

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"convertpdfgo/config"
	"convertpdfgo/pkg/logger"
	"convertpdfgo/storage"
)

type Store struct {
	pool  *pgxpool.Pool
	log   logger.ILogger
	redis storage.IRedisStorage
}

func New(ctx context.Context, cfg config.Config, log logger.ILogger, redis storage.IRedisStorage) (storage.IStorage, error) {
	url := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		cfg.PostgresUser,
		cfg.PostgresPassword,
		cfg.PostgresHost,
		cfg.PostgresPort,
		cfg.PostgresDB,
	)

	poolConfig, err := pgxpool.ParseConfig(url)
	if err != nil {
		log.Error("error while parsing config", logger.Error(err))
		return nil, err
	}

	// Connection pool tuning. The defaults (4 conns) are too low for an
	// async API + worker pulling from the same DB.
	poolConfig.MaxConns = 25
	poolConfig.MinConns = 5
	poolConfig.MaxConnLifetime = time.Hour
	poolConfig.MaxConnIdleTime = 30 * time.Minute
	poolConfig.HealthCheckPeriod = time.Minute

	// Per-connection safety nets:
	//   * statement_timeout caps any single query so a runaway pdfcpu/Gotenberg
	//     read can't pin a connection.
	//   * idle_in_transaction_session_timeout kicks orphaned txns off the pool
	//     before they exhaust MaxConns.
	poolConfig.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		const stmts = `
			SET statement_timeout = '30s';
			SET idle_in_transaction_session_timeout = '1min';
			SET lock_timeout = '5s';
		`
		if _, err := conn.Exec(ctx, stmts); err != nil {
			return fmt.Errorf("apply session timeouts: %w", err)
		}
		return nil
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		log.Error("error while connecting to database", logger.Error(err))
		return nil, err
	}

	// Migrations should run on the API service only. Worker (and any other
	// process that shares the DB) skips this entirely so two boots can't
	// race on the schema_migrations row and leave it in a "dirty" state —
	// a real bug we hit during the first deploy.
	//
	// Set RUN_MIGRATIONS=true in the API container's env. Default off.
	if os.Getenv("RUN_MIGRATIONS") == "true" {
		absPath, err := filepath.Abs("migrations/postgres")
		if err != nil {
			log.Error("failed to get absolute path for migrations", logger.Error(err))
			return nil, err
		}
		m, err := migrate.New("file://"+absPath, url)
		if err != nil {
			log.Error("migration error", logger.Error(err))
			return nil, err
		}
		if err = m.Up(); err != nil && !strings.Contains(err.Error(), "no change") {
			log.Error("migration up error", logger.Error(err))
			return nil, err
		}
		log.Info("postgres connected and migrated")
	} else {
		log.Info("postgres connected (migrations skipped — set RUN_MIGRATIONS=true on api service)")
	}

	return &Store{
		pool:  pool,
		log:   log,
		redis: redis,
	}, nil
}

func (s *Store) Close() {
	s.pool.Close()
}

func (s *Store) User() storage.IUserStorage {
	return NewUserRepo(s.pool, s.log)
}

func (s *Store) OTP() storage.IOTPStorage {
	return NewOTPRepo(s.pool, s.log)
}

func (s *Store) Role() storage.IRoleStorage {
	return NewRoleRepo(s.pool, s.log)
}

func (s *Store) Sysuser() storage.ISysuserStorage {
	return NewSysuserRepo(s.pool, s.log)
}

func (s *Store) AuthAudit() storage.IAuthAuditStorage {
	return NewAuthAuditRepo(s.pool, s.log)
}

func (s *Store) Permission() storage.IPermissionStorage {
	return NewPermissionRepo(s.pool, s.log)
}

func (s *Store) AdminStats() storage.IAdminStatsStorage {
	return NewAdminStatsRepo(s.pool, s.log)
}

func (s *Store) Redis() storage.IRedisStorage {
	return s.redis
}

func (s *Store) Merge() storage.IMergeStorage {
	return NewMergeRepo(s.pool, s.log)
}

func (s *Store) File() storage.IFileStorage {
	return NewFileRepo(s.pool, s.log)
}

func (s *Store) Split() storage.ISplitStorage {
	return NewSplitRepo(s.pool, s.log)
}

func (s *Store) RemovePage() storage.IRemovePageStorage {
	return NewRemovePageRepo(s.pool, s.log)
}

func (s *Store) ExtractPage() storage.IExtractPageStorage {
	return NewExtractPageRepo(s.pool, s.log)
}

func (s *Store) Compress() storage.ICompressStorage {
	return NewCompressRepo(s.pool, s.log)
}

func (s *Store) PDFToJPG() storage.IPDFToJPGStorage {
	return NewPDFToJPGRepo(s.pool, s.log)
}

func (s *Store) Rotate() storage.IRotateStorage {
	return NewRotateRepo(s.pool, s.log)
}

func (s *Store) AddPageNumber() storage.IAddPageNumberStorage {
	return NewAddPageNumberRepo(s.pool, s.log)
}
func (s *Store) Crop() storage.ICropPDFStorage {
	return NewCropRepo(s.pool, s.log)
}

func (s *Store) Unlock() storage.IUnlockPDFStorage {
	return NewUnlockRepo(s.pool, s.log)
}
func (s *Store) Protect() storage.IProtectStorage {
	return NewProtectRepo(s.pool, s.log)
}

func (s *Store) Stat() storage.IStatStorage {
	return NewStatsRepo(s.pool, s.log)
}
func (s *Store) Log() storage.ILogService {
	return NewLogRepo(s.pool, s.log)
}
func (s *Store) JPGToPDF() storage.IJPGToPDFStorage {
	return NewJPGToPDFRepo(s.pool, s.log)
}
func (s *Store) Inspect() storage.IInspectStorage {
	return NewInspectRepo(s.pool, s.log)
}

func (s *Store) SharedLink() storage.ISharedLinkStorage {
	return NewSharedLinkRepo(s.pool, s.log)
}

func (s *Store) AddHeaderFooter() storage.AddHeaderFooterStorage {
	return NewAddHeaderFooterRepo(s.pool, s.log)
}

func (s *Store) DetectBlankPages() storage.IDetectBlankPagesStorage {
	return NewDetectBlankPagesRepo(s.pool, s.log)
}

func (s *Store) QRCode() storage.IQRCodeStorage {
	return NewQRCodeRepo(s.pool, s.log)
}

func (s *Store) OCR() storage.IOCRStorage           { return NewOCRRepo(s.pool, s.log) }
func (s *Store) ScanToPDF() storage.IScanToPDFStorage { return NewScanToPDFRepo(s.pool, s.log) }
func (s *Store) OCRExtractText() storage.IOCRExtractTextStorage {
	return NewOCRExtractTextRepo(s.pool, s.log)
}
func (s *Store) ImageToText() storage.IImageToTextStorage {
	return NewImageToTextRepo(s.pool, s.log)
}
func (s *Store) OCRDetectLang() storage.IOCRDetectLangStorage {
	return NewOCRDetectLangRepo(s.pool, s.log)
}
func (s *Store) OCRStructured() storage.IOCRStructuredStorage {
	return NewOCRStructuredRepo(s.pool, s.log)
}

func (s *Store) PDFToHTML() storage.IPDFToHTMLStorage {
	return NewPDFToHTMLRepo(s.pool, s.log)
}

func (s *Store) PDFToPPT() storage.IPDFToPPTStorage {
	return NewPDFToPPTRepo(s.pool, s.log)
}

func (s *Store) PDFToExcel() storage.IPDFToExcelStorage {
	return NewPDFToExcelRepo(s.pool, s.log)
}

func (s *Store) PDFToWord() storage.IPDFToWordStorage {
	return NewPDFToWordRepo(s.pool, s.log)
}

func (s *Store) WordToPDF() storage.IWordToPDFStorage {
	return NewWordToPDFRepo(s.pool, s.log)
}

func (s *Store) ExcelToPDF() storage.IExcelToPDFStorage {
	return NewExcelToPDFRepo(s.pool, s.log)
}
func (s *Store) PowerPointToPDF() storage.IPowerPointToPDFStorage {
	return NewPowerPointToPDFRepo(s.pool, s.log)
}

func (s *Store) HTMLToPDF() storage.IHTMLToPDFStorage {
	return NewHTMLToPDFRepo(s.pool, s.log)
}

func (s *Store) Watermark() storage.IWatermarkStorage {
	return NewWatermarkRepo(s.pool, s.log)
}

func (s *Store) ExtractText() storage.IExtractTextStorage {
	return NewExtractTextRepo(s.pool, s.log)
}

func (s *Store) NUp() storage.INUpStorage {
	return NewNUpRepo(s.pool, s.log)
}

func (s *Store) URLToPDF() storage.IURLToPDFStorage {
	return NewURLToPDFRepo(s.pool, s.log)
}

func (s *Store) Metadata() storage.IMetadataStorage {
	return NewMetadataRepo(s.pool, s.log)
}
