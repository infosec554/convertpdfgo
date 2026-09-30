// Package aicmd turns a sentence a visitor typed — in any language —
// into a tool this site actually has.
//
// The model never decides what exists. It is handed the catalogue and
// asked to pick from it, and its answer is checked against that same
// catalogue before anything is returned. A hallucinated slug becomes
// "unclear" rather than a link to a 404.
package aicmd

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"convertpdfgo/service/llm"
)

// Tool is one entry in the catalogue the model chooses from.
type Tool struct {
	Slug        string `json:"slug"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// Plan is what the router decided. Kind is "tool", "academic" or
// "unclear"; the other fields are populated per kind.
type Plan struct {
	Kind    string `json:"kind"`
	Slug    string `json:"slug,omitempty"`
	Service string `json:"service,omitempty"`
	Summary string `json:"summary,omitempty"`
	Message string `json:"message,omitempty"`
}

// Request is one routing question: what the user typed, and the names
// of any files they attached. Only the names and types travel — the
// bytes stay where they are, since a filename is enough to tell a PDF
// from a spreadsheet and uploading more would cost tokens and privacy
// for nothing.
type Request struct {
	Text      string
	FileNames []string
	Tools     []Tool
}

const systemPrompt = `You route a user's request to exactly one tool from a catalogue.

Rules:
- Reply with JSON only, no prose, matching: {"kind":"tool","slug":"...","summary":"..."} or {"kind":"unclear","message":"..."}
- "slug" MUST be one of the slugs in the catalogue. Never invent one.
- "summary" is one short sentence, in the same language the user wrote in, describing what will happen.
- If the request is about writing a thesis, course paper, report, presentation or a plagiarism check, reply {"kind":"academic","service":"...","summary":"..."}.
- If you cannot tell what is wanted, or no tool fits, reply with kind "unclear" and say why in the user's language.
- The user may write in Uzbek, Russian, English or any other language. Understand it; do not translate your summary to English.`

// Route asks the model to pick a tool, then verifies the answer against
// the catalogue it was given.
func Route(ctx context.Context, p llm.Provider, req Request) (Plan, error) {
	if strings.TrimSpace(req.Text) == "" && len(req.FileNames) == 0 {
		return Plan{Kind: "unclear", Message: "Nothing to do — type what you want, or attach a file."}, nil
	}

	catalogue, err := json.Marshal(req.Tools)
	if err != nil {
		return Plan{}, fmt.Errorf("aicmd: encode catalogue: %w", err)
	}

	var b strings.Builder
	b.WriteString("Catalogue:\n")
	b.Write(catalogue)
	b.WriteString("\n\nUser request: ")
	b.WriteString(strings.TrimSpace(req.Text))
	if len(req.FileNames) > 0 {
		b.WriteString("\nAttached files: ")
		b.WriteString(strings.Join(req.FileNames, ", "))
	}

	stream, err := p.Chat(ctx, []llm.Message{{Role: llm.RoleUser, Content: b.String()}}, llm.ChatOpts{
		System:      systemPrompt,
		Temperature: 0,
		MaxTokens:   300,
		JSONMode:    true,
	})
	if err != nil {
		return Plan{}, err
	}

	var raw strings.Builder
	for chunk := range stream {
		if chunk.Err != nil {
			return Plan{}, chunk.Err
		}
		raw.WriteString(chunk.Delta)
	}

	var plan Plan
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw.String())), &plan); err != nil {
		// A reply that is not JSON is a model failure, not a user one.
		return Plan{Kind: "unclear", Message: "Could not work out what you meant. Try naming the tool."}, nil
	}

	switch plan.Kind {
	case "tool":
		for _, t := range req.Tools {
			if t.Slug == plan.Slug {
				return plan, nil
			}
		}
		// Picked something that does not exist.
		return Plan{Kind: "unclear", Message: "No tool here does that yet."}, nil
	case "academic", "unclear":
		return plan, nil
	default:
		return Plan{Kind: "unclear", Message: "Could not work out what you meant."}, nil
	}
}
