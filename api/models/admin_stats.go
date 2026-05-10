package models

import "time"

// DailyCount — bitta sana uchun nechta yozuv yaratilgani.
type DailyCount struct {
	Day   string `json:"day"`   // YYYY-MM-DD
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
