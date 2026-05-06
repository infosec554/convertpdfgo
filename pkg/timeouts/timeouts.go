package timeouts

import "time"

const (
	DB      = 5 * time.Second
	Process = 5 * time.Minute
)
