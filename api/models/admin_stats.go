package models

import "time"

// DailyCount — bitta sana uchun nechta yozuv yaratilgani.
type DailyCount struct {
	Day   string `json:"day"` // YYYY-MM-DD
	Count int    `json:"count"`
}

// TopUserDisk — kim qancha disk band qilgan, list endpointi uchun.
type TopUserDisk struct {
	UserID    string `json:"user_id"`
	Email     string `json:"email,omitempty"`
	FileCount int    `json:"file_count"`
	TotalSize int64  `json:"total_size_bytes"`
}

// SuspiciousLogin — oxirgi N daqiqada `login_fail` necha marta sodir bo'lgan
// va qaysi IP'lar/email'lar shubhali deb belgilangan.
type SuspiciousLogin struct {
	Email     string    `json:"email"`
	UserID    *string   `json:"user_id,omitempty"`
	FailCount int       `json:"fail_count"`
	IPs       []string  `json:"ips"`
	LastSeen  time.Time `json:"last_seen"`
}

// AdminFileFilter — admin file listing parametrlari. Bo'sh maydonlar
// "har qanday" deb hisoblanadi. UserID="guest" — anonymous uploads.
// Bound via ShouldBindQuery; never serialized — json:"-" makes that
// intent explicit and keeps the jsontags audit honest.
type AdminFileFilter struct {
	UserID   string `form:"user_id"   json:"-"`
	FileType string `form:"file_type" json:"-"` // .pdf, .docx, ...
	Q        string `form:"q"         json:"-"` // file_name LIKE %q%
	Limit    int    `form:"limit"     json:"-"`
	Offset   int    `form:"offset"    json:"-"`
}

// LockedAccount — login_limiter qulflagan email/key.
type LockedAccount struct {
	Email     string `json:"email"`
	FailCount int    `json:"fail_count"`
}

// PublicCounts is the un-authenticated landing-page payload. Each
// number is a global aggregate — nothing here is tied to a specific
// user, so it's safe to expose without auth. The service layer
// caches this for ~30 seconds; pollers shouldn't amplify load on
// the DB.
type PublicCounts struct {
	TotalUsers          int `json:"total_users"`
	TotalFilesUploaded  int `json:"total_files_uploaded"`
	TotalFilesProcessed int `json:"total_files_processed"` // sum of every *_jobs.status='done'
}
