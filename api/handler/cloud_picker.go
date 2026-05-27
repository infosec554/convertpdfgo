package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// CloudPickerConfig is the public configuration the frontend needs to
// boot the Dropbox Chooser and Google Picker SDKs. None of these values
// are secret — Dropbox app key, Google API key + OAuth client ID all
// ship inside the JavaScript SDKs that run in the user's browser. The
// matching secrets stay server-side and only get used for cloud-export
// (token exchange happens via the existing /oauth/:provider/callback).
//
// We hand them out via a single endpoint instead of baking them into
// the Next.js build so that rotating a key only needs an api restart,
// not a frontend rebuild.
type cloudPickerConfig struct {
	DropboxAppKey  string `json:"dropbox_app_key,omitempty"`
	GoogleAPIKey   string `json:"google_api_key,omitempty"`
	GoogleClientID string `json:"google_client_id,omitempty"`
}

// GetCloudPickerConfig godoc
// @Router  /api/cloud-picker/config [get]
// @Tags    file
// @Success 200 {object} cloudPickerConfig
func (h Handler) GetCloudPickerConfig(c *gin.Context) {
	// We reuse the Google OAuth client for Drive Picker too — same
	// project, just a wider scope on the OAuth grant.
	cfg := cloudPickerConfig{
		DropboxAppKey:  h.cfg.DropboxAppKey,
		GoogleClientID: h.cfg.GDriveClientID,
		// Google API key for the Drive Picker comes later — wire it in
		// when we ship the Google Picker MVP (separate env var so
		// rotation doesn't disturb the Drive-export flow).
		GoogleAPIKey: "",
	}
	c.JSON(http.StatusOK, cfg)
}
