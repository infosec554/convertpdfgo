package models

// Response is the envelope every endpoint wraps its payload in.
//
// Note: historical clients (including the integration test suite) read
// PascalCase keys (`StatusCode` / `Description` / `Data`) — we keep
// them explicit here rather than migrating to snake_case to avoid a
// breaking change. New endpoints should still nest snake_case fields
// inside Data.
type Response struct {
	StatusCode  int         `json:"StatusCode"`
	Description string      `json:"Description"`
	Data        interface{} `json:"Data"`
}
