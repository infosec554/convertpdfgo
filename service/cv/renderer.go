package cv

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"strings"
	"sync"

	"convertpdfgo/api/models"
	"convertpdfgo/pkg/logger"
)

//go:embed templates/*.html.tmpl
var templateFS embed.FS

// renderer turns a CV (session.locale + ordered sections) into HTML.
// The exporter then hands that HTML to Gotenberg for PDF output, or
// the API serves it directly for the live preview iframe.
type renderer struct {
	log   logger.ILogger
	cache sync.Map // template_id → *template.Template
}

func newRenderer(log logger.ILogger) *renderer {
	return &renderer{log: log}
}

// Render finds the template, normalises the section list into a flat
// view-model the template can read with simple dot-notation, and
// executes it.
func (r *renderer) Render(templateID, locale string, sections []models.CVSection) (string, error) {
	if templateID == "" {
		templateID = "modern"
	}
	t, err := r.loadTemplate(templateID)
	if err != nil {
		return "", err
	}
	view := buildViewModel(locale, sections)
	var buf bytes.Buffer
	if err := t.Execute(&buf, view); err != nil {
		return "", fmt.Errorf("render: execute: %w", err)
	}
	return buf.String(), nil
}

func (r *renderer) loadTemplate(id string) (*template.Template, error) {
	if cached, ok := r.cache.Load(id); ok {
		return cached.(*template.Template), nil
	}
	path := "templates/" + id + ".html.tmpl"
	raw, err := templateFS.ReadFile(path)
	if err != nil {
		// Fall back to modern if the requested template is missing —
		// users shouldn't lose their CV because the database has a
		// stale template_id we removed in a migration.
		if id != "modern" {
			r.log.Warning("cv renderer: template not found; falling back to modern",
				logger.String("template_id", id))
			return r.loadTemplate("modern")
		}
		return nil, fmt.Errorf("template %s: %w", id, err)
	}
	t, err := template.New(id).Funcs(templateFuncs).Parse(string(raw))
	if err != nil {
		return nil, fmt.Errorf("template %s parse: %w", id, err)
	}
	r.cache.Store(id, t)
	return t, nil
}

// templateFuncs are tiny helpers each .html.tmpl can call. Keep this
// list small — anything more complex belongs in buildViewModel.
var templateFuncs = template.FuncMap{
	"join":    func(s []string, sep string) string { return strings.Join(s, sep) },
	"nonzero": func(s string) bool { return strings.TrimSpace(s) != "" },
}

// CVViewModel is the shape every template renders against. We pre-
// compute it instead of pushing the raw JSONB map straight in so the
// .html.tmpl files stay readable.
type CVViewModel struct {
	Locale     string
	Personal   PersonalView
	Summary    string
	Experience []ExperienceItem
	Education  []EducationItem
	Skills     SkillsView
	Languages  []LanguageItem
	Projects   []ProjectItem
	Awards     []AwardItem
	Certs      []CertItem
	HasContent map[string]bool // for {{if .HasContent.experience}} guards
}

type PersonalView struct {
	Name, Title, Email, Phone, Location, LinkedIn, GitHub, Website string
}

type ExperienceItem struct {
	Title, Company, Period, Location string
	Bullets                          []string
}

type EducationItem struct {
	Degree, School, Period, GPA, Honors string
}

type SkillsView struct {
	Technical []string
	Soft      []string
}

type LanguageItem struct {
	Name, Level string
}

type ProjectItem struct {
	Name, Description, Link string
	Tech                    []string
}

type AwardItem struct {
	Title, Issuer, Year string
}

type CertItem struct {
	Name, Issuer, Year string
}

// buildViewModel walks the section rows and pulls out the strongly-
// typed fields each template uses. Anything missing falls back to
// zero-values so {{nonzero}} guards in the templates can hide empty
// blocks cleanly.
func buildViewModel(locale string, sections []models.CVSection) CVViewModel {
	v := CVViewModel{Locale: locale, HasContent: map[string]bool{}}
	if locale == "" {
		v.Locale = "en"
	}
	for _, s := range sections {
		switch s.SectionType {
		case models.SectionPersonal:
			v.Personal = personalFromMap(s.Content)
			v.HasContent["personal"] = true
		case models.SectionSummary:
			if txt, _ := s.Content["text"].(string); strings.TrimSpace(txt) != "" {
				v.Summary = txt
				v.HasContent["summary"] = true
			}
		case models.SectionExperience:
			v.Experience = experienceFromMap(s.Content)
			v.HasContent["experience"] = len(v.Experience) > 0
		case models.SectionEducation:
			v.Education = educationFromMap(s.Content)
			v.HasContent["education"] = len(v.Education) > 0
		case models.SectionSkills:
			v.Skills = skillsFromMap(s.Content)
			v.HasContent["skills"] = len(v.Skills.Technical)+len(v.Skills.Soft) > 0
		case models.SectionLanguages:
			v.Languages = languagesFromMap(s.Content)
			v.HasContent["languages"] = len(v.Languages) > 0
		case models.SectionProjects:
			v.Projects = projectsFromMap(s.Content)
			v.HasContent["projects"] = len(v.Projects) > 0
		case models.SectionAwards:
			v.Awards = awardsFromMap(s.Content)
			v.HasContent["awards"] = len(v.Awards) > 0
		case models.SectionCertifications:
			v.Certs = certsFromMap(s.Content)
			v.HasContent["certifications"] = len(v.Certs) > 0
		}
	}
	return v
}

func personalFromMap(m map[string]any) PersonalView {
	return PersonalView{
		Name:     asString(m, "name"),
		Title:    asString(m, "title"),
		Email:    asString(m, "email"),
		Phone:    asString(m, "phone"),
		Location: asString(m, "location"),
		LinkedIn: asString(m, "linkedin"),
		GitHub:   asString(m, "github"),
		Website:  asString(m, "website"),
	}
}

func experienceFromMap(m map[string]any) []ExperienceItem {
	items, _ := m["items"].([]any)
	out := make([]ExperienceItem, 0, len(items))
	for _, it := range items {
		im, ok := it.(map[string]any)
		if !ok {
			continue
		}
		out = append(out, ExperienceItem{
			Title:    asString(im, "title"),
			Company:  asString(im, "company"),
			Period:   asString(im, "period"),
			Location: asString(im, "location"),
			Bullets:  asStringSlice(im, "bullets"),
		})
	}
	return out
}

func educationFromMap(m map[string]any) []EducationItem {
	items, _ := m["items"].([]any)
	out := make([]EducationItem, 0, len(items))
	for _, it := range items {
		im, ok := it.(map[string]any)
		if !ok {
			continue
		}
		out = append(out, EducationItem{
			Degree: asString(im, "degree"),
			School: asString(im, "school"),
			Period: asString(im, "period"),
			GPA:    asString(im, "gpa"),
			Honors: asString(im, "honors"),
		})
	}
	return out
}

func skillsFromMap(m map[string]any) SkillsView {
	return SkillsView{
		Technical: asStringSlice(m, "technical"),
		Soft:      asStringSlice(m, "soft"),
	}
}

func languagesFromMap(m map[string]any) []LanguageItem {
	items, _ := m["items"].([]any)
	out := make([]LanguageItem, 0, len(items))
	for _, it := range items {
		im, ok := it.(map[string]any)
		if !ok {
			continue
		}
		out = append(out, LanguageItem{
			Name:  asString(im, "name"),
			Level: asString(im, "level"),
		})
	}
	return out
}

func projectsFromMap(m map[string]any) []ProjectItem {
	items, _ := m["items"].([]any)
	out := make([]ProjectItem, 0, len(items))
	for _, it := range items {
		im, ok := it.(map[string]any)
		if !ok {
			continue
		}
		out = append(out, ProjectItem{
			Name:        asString(im, "name"),
			Description: asString(im, "description"),
			Link:        asString(im, "link"),
			Tech:        asStringSlice(im, "tech"),
		})
	}
	return out
}

func awardsFromMap(m map[string]any) []AwardItem {
	items, _ := m["items"].([]any)
	out := make([]AwardItem, 0, len(items))
	for _, it := range items {
		im, ok := it.(map[string]any)
		if !ok {
			continue
		}
		out = append(out, AwardItem{
			Title:  asString(im, "title"),
			Issuer: asString(im, "issuer"),
			Year:   asString(im, "year"),
		})
	}
	return out
}

func certsFromMap(m map[string]any) []CertItem {
	items, _ := m["items"].([]any)
	out := make([]CertItem, 0, len(items))
	for _, it := range items {
		im, ok := it.(map[string]any)
		if !ok {
			continue
		}
		out = append(out, CertItem{
			Name:   asString(im, "name"),
			Issuer: asString(im, "issuer"),
			Year:   asString(im, "year"),
		})
	}
	return out
}

func asString(m map[string]any, key string) string {
	s, _ := m[key].(string)
	return strings.TrimSpace(s)
}

func asStringSlice(m map[string]any, key string) []string {
	raw, _ := m[key].([]any)
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		if s, ok := item.(string); ok {
			if s = strings.TrimSpace(s); s != "" {
				out = append(out, s)
			}
		}
	}
	return out
}
