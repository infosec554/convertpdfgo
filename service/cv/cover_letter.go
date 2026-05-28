package cv

import (
	"context"
	"fmt"
	"strings"

	"convertpdfgo/api/models"
	"convertpdfgo/pkg/logger"
	"convertpdfgo/service/llm"
	"convertpdfgo/storage"
)

// coverLetter generates a tailored cover letter for a target job using
// the structured sections of a CV as the source of truth. We never
// invent achievements; the prompt explicitly tells the model to pull
// only from the candidate's real experience so the output is honest
// and ATS-aligned.
type coverLetter struct {
	stg storage.IStorage
	llm llm.Provider
	log logger.ILogger
}

func newCoverLetter(stg storage.IStorage, p llm.Provider, log logger.ILogger) *coverLetter {
	return &coverLetter{stg: stg, llm: p, log: log}
}

// Generate returns ~250 words of cover-letter text addressed to the
// posted role, drawing on the flattened section content. Tone is one
// of "enthusiastic" | "formal" | "conversational"; anything else
// defaults to formal.
func (c *coverLetter) Generate(ctx context.Context, cvID string, req models.CVCoverLetterRequest) (*models.CVCoverLetterResponse, error) {
	sess, err := c.stg.CV().GetSession(ctx, cvID)
	if err != nil {
		return nil, err
	}
	sections, err := c.stg.CV().ListSections(ctx, cvID)
	if err != nil {
		return nil, err
	}

	var resumeText strings.Builder
	for _, s := range sections {
		resumeText.WriteString("# ")
		resumeText.WriteString(strings.ToUpper(s.SectionType))
		resumeText.WriteByte('\n')
		resumeText.WriteString(flattenSectionText(s.Content))
		resumeText.WriteString("\n\n")
	}

	tone := strings.ToLower(strings.TrimSpace(req.Tone))
	switch tone {
	case "enthusiastic", "formal", "conversational":
		// ok
	default:
		tone = "formal"
	}

	title := strings.TrimSpace(req.JobTitle)
	if title == "" {
		title = "the role described below"
	}

	prompt := fmt.Sprintf(`You are a professional career coach writing a cover letter on behalf of the candidate.

CONSTRAINTS:
- 220-300 words. Three to four short paragraphs.
- Only use facts present in the candidate's resume below. Do NOT invent achievements, employers, dates, or skills.
- Match the job description's keywords where the resume genuinely supports them.
- Tone: %s.
- Output in %s.
- Output the letter body only — no salutation greeting line, no signature, no markdown headers, no bullet points. Just plain paragraphs separated by a blank line.

TARGET ROLE: %s

JOB DESCRIPTION:
%s

CANDIDATE'S RESUME:
%s`,
		tone, sess.Locale, title, req.JobDescription, resumeText.String())

	ch, err := c.llm.Chat(ctx, []llm.Message{{Role: llm.RoleUser, Content: prompt}}, llm.ChatOpts{
		Temperature: 0.4,
		MaxTokens:   1200,
	})
	if err != nil {
		return nil, fmt.Errorf("cover-letter: llm: %w", err)
	}
	var sb strings.Builder
	for c := range ch {
		if c.Err != nil {
			return nil, c.Err
		}
		sb.WriteString(c.Delta)
	}

	body := strings.TrimSpace(sb.String())
	words := len(strings.Fields(body))
	return &models.CVCoverLetterResponse{
		Text:      body,
		WordCount: words,
		Tone:      tone,
		Locale:    sess.Locale,
	}, nil
}
