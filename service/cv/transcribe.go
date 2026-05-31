package cv

import (
	"context"
	"fmt"
	"strings"

	"convertpdfgo/pkg/logger"
	"convertpdfgo/service/llm"
)

// transcriber turns a chunk of recorded audio into text via Gemini's
// multimodal API. We use this as the universal fallback for browsers
// that don't reliably implement the Web Speech API (Yandex, Firefox,
// some Chromium forks). The audio bytes ride through the existing
// Attachment plumbing — no new wire protocol needed.
type transcriber struct {
	llm llm.Provider
	log logger.ILogger
}

func newTranscriber(p llm.Provider, log logger.ILogger) *transcriber {
	return &transcriber{llm: p, log: log}
}

// Transcribe runs one audio→text pass. `mime` should be the recorder's
// own MIME, e.g. "audio/webm;codecs=opus". `localeHint` (optional) lets
// the model know which language to expect — improves accuracy on
// accented English or mixed-language speech.
func (t *transcriber) Transcribe(ctx context.Context, audio []byte, mime, localeHint string) (string, error) {
	if len(audio) == 0 {
		return "", fmt.Errorf("transcribe: empty audio")
	}
	if mime == "" {
		mime = "audio/webm"
	}
	// Gemini wants the bare MIME with no codec parameter — strip the
	// ";codecs=…" suffix MediaRecorder typically appends.
	if i := strings.IndexByte(mime, ';'); i >= 0 {
		mime = strings.TrimSpace(mime[:i])
	}

	hint := ""
	if l := strings.TrimSpace(localeHint); l != "" {
		hint = fmt.Sprintf(" The speaker is using %s — transcribe in that language.", l)
	}

	prompt := "Transcribe the spoken words in this audio clip. " +
		"Output only the transcript — no commentary, no formatting, no labels." + hint

	ch, err := t.llm.Chat(ctx, []llm.Message{
		{
			Role:    llm.RoleUser,
			Content: prompt,
			Attachments: []llm.Attachment{{
				MIMEType: mime,
				Data:     audio,
			}},
		},
	}, llm.ChatOpts{
		Temperature: 0,
		MaxTokens:   2000,
	})
	if err != nil {
		return "", fmt.Errorf("transcribe: llm: %w", err)
	}

	var sb strings.Builder
	for c := range ch {
		if c.Err != nil {
			return "", c.Err
		}
		sb.WriteString(c.Delta)
	}
	return strings.TrimSpace(sb.String()), nil
}
