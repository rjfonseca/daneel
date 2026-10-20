//nolint:revive // package comment not required
package meetings

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

// MeetingNote represents a structured meeting artifact.
type MeetingNote struct {
	SourceFile   string   `json:"source_file"`
	Title        string   `json:"title"`
	Date         string   `json:"date"`
	Slug         string   `json:"slug"`
	Participants []string `json:"participants"`
	Summary      string   `json:"summary"`
	ActionItems  []string `json:"action_items"`
	Decisions    []string `json:"decisions"`
	Category     string   `json:"category"`
	SavedPath    string   `json:"saved_path"`
}

// IngestReport holds summary of batch processed meetings.
type IngestReport struct {
	TotalProcessed int           `json:"total_processed"`
	Meetings       []MeetingNote `json:"meetings"`
	UpdatedPeople  []string      `json:"updated_people"`
	CleanedInbox   int           `json:"cleaned_inbox"`
}

// Processor coordinates parsing, people indexing, and OKF archiving of meeting notes.
type Processor struct {
	vaultRoot    string
	inboxDir     string
	sourcesDir   string
	peopleDir    string
	dailyDir     string
	peopleLookup map[string]string // normalized variant -> slug
	peopleNames  map[string]string // slug -> full name
}

// NewProcessor creates a new meeting processor instance.
func NewProcessor(vaultRoot string) *Processor {
	peopleDir := filepath.Join(vaultRoot, "notes", "pessoas")
	if _, err := os.Stat(peopleDir); os.IsNotExist(err) {
		peopleDir = filepath.Join(vaultRoot, "meta", "pessoas")
	}

	p := &Processor{
		vaultRoot:    vaultRoot,
		inboxDir:     filepath.Join(vaultRoot, "inbox"),
		sourcesDir:   filepath.Join(vaultRoot, "sources", "reunioes"),
		peopleDir:    peopleDir,
		dailyDir:     filepath.Join(vaultRoot, "daily"),
		peopleLookup: make(map[string]string),
		peopleNames:  make(map[string]string),
	}
	p.loadPeople()
	return p
}

func (p *Processor) loadPeople() {
	if _, err := os.Stat(p.peopleDir); os.IsNotExist(err) {
		return
	}

	entries, err := os.ReadDir(p.peopleDir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") || entry.Name() == "moc-pessoas.md" {
			continue
		}

		slug := strings.TrimSuffix(entry.Name(), ".md")
		data, err := os.ReadFile(filepath.Join(p.peopleDir, entry.Name()))
		if err != nil {
			continue
		}
		content := string(data)

		fullName := strings.Title(strings.ReplaceAll(slug, "-", " ")) //nolint:staticcheck
		if m := regexp.MustCompile(`(?m)^#\s*👤?\s*(.*?)$`).FindStringSubmatch(content); len(m) > 1 {
			fullName = strings.TrimSpace(m[1])
		}

		p.peopleNames[slug] = fullName
		p.addLookup(fullName, slug)
		p.addLookup(slug, slug)
		p.addLookup(strings.ReplaceAll(slug, "-", " "), slug)

		// Parse aliases from frontmatter
		if m := regexp.MustCompile(`aliases:\s*\n((?:\s*-\s*.*?\n)+)`).FindStringSubmatch(content); len(m) > 1 {
			lines := strings.Split(m[1], "\n")
			for _, l := range lines {
				al := strings.Trim(l, "- \t\"'")
				if al != "" {
					p.addLookup(al, slug)
				}
			}
		}
	}
}

func (p *Processor) addLookup(variant string, slug string) {
	norm := normalizeString(variant)
	if norm != "" && p.peopleLookup[norm] == "" {
		p.peopleLookup[norm] = slug
	}
}

func (p *Processor) findPerson(rawName string) (string, string) {
	norm := normalizeString(rawName)
	if slug, ok := p.peopleLookup[norm]; ok {
		return slug, p.peopleNames[slug]
	}

	// Try first and last name match
	parts := strings.Fields(norm)
	if len(parts) >= 2 {
		short := parts[0] + " " + parts[len(parts)-1]
		if slug, ok := p.peopleLookup[short]; ok {
			return slug, p.peopleNames[slug]
		}
	}

	return "", ""
}

// ProcessInbox processes all meeting note candidates in inbox/.
func (p *Processor) ProcessInbox(deleteOriginal bool) (*IngestReport, error) {
	report := &IngestReport{
		Meetings:      []MeetingNote{},
		UpdatedPeople: []string{},
	}

	if _, err := os.Stat(p.inboxDir); os.IsNotExist(err) {
		return report, nil
	}

	entries, err := os.ReadDir(p.inboxDir)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		nameLower := strings.ToLower(entry.Name())
		if strings.Contains(nameLower, "gemini") || strings.Contains(nameLower, "reunião") ||
			strings.Contains(nameLower, "reuniao") || strings.Contains(nameLower, "transcrição") ||
			strings.Contains(nameLower, "1:1") || strings.Contains(nameLower, "1-1") ||
			strings.Contains(nameLower, "alinhamento") || strings.Contains(nameLower, "anotações") {

			filePath := filepath.Join(p.inboxDir, entry.Name())
			note, err := p.ProcessFile(filePath)
			if err == nil && note != nil {
				report.Meetings = append(report.Meetings, *note)
				report.TotalProcessed++

				if deleteOriginal {
					_ = os.Remove(filePath)
					report.CleanedInbox++
				}
			}
		}
	}

	return report, nil
}

// ProcessZip processes meeting notes archived in a .zip file.
func (p *Processor) ProcessZip(zipPath string) (*IngestReport, error) {
	report := &IngestReport{
		Meetings:      []MeetingNote{},
		UpdatedPeople: []string{},
	}

	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return nil, err
	}
	defer r.Close() //nolint:errcheck,gosec

	for _, f := range r.File {
		if f.FileInfo().IsDir() || !strings.HasSuffix(f.Name, ".md") {
			continue
		}

		rc, err := f.Open()
		if err != nil {
			continue
		}
		data, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			continue
		}

		note, err := p.parseAndArchive(filepath.Base(f.Name), string(data))
		if err == nil && note != nil {
			report.Meetings = append(report.Meetings, *note)
			report.TotalProcessed++
		}
	}

	return report, nil
}

// ProcessFile processes a single meeting file.
func (p *Processor) ProcessFile(filePath string) (*MeetingNote, error) {
	data, err := os.ReadFile(filePath) //nolint:gosec // intentional
	if err != nil {
		return nil, err
	}
	return p.parseAndArchive(filepath.Base(filePath), string(data))
}

func (p *Processor) parseAndArchive(filename string, rawContent string) (*MeetingNote, error) {
	// Extract Date
	dateStr := time.Now().Format("2006-01-02")
	dateRegex := regexp.MustCompile(`(\d{4})[-_](\d{2})[-_](\d{2})`)
	if m := dateRegex.FindStringSubmatch(filename); len(m) > 3 {
		dateStr = fmt.Sprintf("%s-%s-%s", m[1], m[2], m[3])
	} else if m := dateRegex.FindStringSubmatch(rawContent); len(m) > 3 {
		dateStr = fmt.Sprintf("%s-%s-%s", m[1], m[2], m[3])
	}

	// Clean Title
	title := strings.TrimSuffix(filename, ".md")
	title = regexp.MustCompile(`\s*-\s*\d{4}[-_]\d{2}[-_]\d{2}.*$`).ReplaceAllString(title, "")
	title = regexp.MustCompile(`\s*-\s*Anotações do Gemini.*$`).ReplaceAllString(title, "")
	title = strings.TrimSpace(title)
	if title == "" {
		title = "Reunião de Alinhamento"
	}

	slug := slugify(title)

	// Parse Attendees, Summary, and Action Items line by line
	var participants []string
	var participantLinks []string
	var summaryLines []string
	var actionItems []string

	lines := strings.Split(rawContent, "\n")
	currentSection := ""

	nameRegex := regexp.MustCompile(`\[([^\]]+)\]\(mailto:[^\)]+\)|([A-ZÁÉÍÓÚÂÊÎÔÛÃÕÇ][a-záéíóúâêîôûãõç]+(?:\s+[A-ZÁÉÍÓÚÂÊÎÔÛÃÕÇ][a-záéíóúâêîôûãõç]+)+)`)

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		lower := strings.ToLower(trimmed)

		if strings.HasPrefix(lower, "convidado") || strings.HasPrefix(lower, "participate") || strings.HasPrefix(lower, "attendee") {
			currentSection = "attendees"
			// Process line after prefix
			rawNames := strings.TrimSpace(regexp.MustCompile(`(?i)^(?:convidados?|participants?|attendees?)\s*`).ReplaceAllString(trimmed, ""))
			if rawNames != "" {
				found := nameRegex.FindAllStringSubmatch(rawNames, -1)
				for _, match := range found {
					rawName := match[1]
					if rawName == "" {
						rawName = match[2]
					}
					rawName = strings.TrimSpace(rawName)
					if rawName != "" && !strings.Contains(rawName, "~~") {
						participants = append(participants, rawName)
						pSlug, pName := p.findPerson(rawName)
						if pSlug != "" {
							participantLinks = append(participantLinks, fmt.Sprintf("[%s](../../notes/pessoas/%s.md)", pName, pSlug))
							p.syncPersonInteraction(pSlug, dateStr, title)
						} else {
							participantLinks = append(participantLinks, rawName)
						}
					}
				}
			}
			continue
		}

		if strings.Contains(lower, "resumo") && (strings.HasPrefix(trimmed, "###") || strings.HasPrefix(trimmed, "##") || strings.HasPrefix(trimmed, "**resumo")) {
			currentSection = "summary"
			continue
		}

		if (strings.Contains(lower, "próximas etapas") || strings.Contains(lower, "proximas etapas") || strings.Contains(lower, "encaminhamentos") || strings.Contains(lower, "ações")) && (strings.HasPrefix(trimmed, "###") || strings.HasPrefix(trimmed, "##")) {
			currentSection = "actions"
			continue
		}

		if strings.HasPrefix(trimmed, "###") || strings.HasPrefix(trimmed, "## ") || strings.HasPrefix(trimmed, "Anexos") || strings.HasPrefix(trimmed, "Registros") {
			if !strings.Contains(lower, "resumo") && !strings.Contains(lower, "etapas") {
				currentSection = ""
			}
		}

		switch currentSection {
		case "attendees":
			if trimmed != "" {
				found := nameRegex.FindAllStringSubmatch(trimmed, -1)
				for _, match := range found {
					rawName := match[1]
					if rawName == "" {
						rawName = match[2]
					}
					rawName = strings.TrimSpace(rawName)
					if rawName != "" && !strings.Contains(rawName, "~~") {
						participants = append(participants, rawName)
						pSlug, pName := p.findPerson(rawName)
						if pSlug != "" {
							participantLinks = append(participantLinks, fmt.Sprintf("[%s](../../notes/pessoas/%s.md)", pName, pSlug))
							p.syncPersonInteraction(pSlug, dateStr, title)
						} else {
							participantLinks = append(participantLinks, rawName)
						}
					}
				}
			}
		case "summary":
			if trimmed != "" && !strings.HasPrefix(trimmed, "###") && !strings.HasPrefix(trimmed, "##") {
				summaryLines = append(summaryLines, trimmed)
			}
		case "actions":
			if (strings.HasPrefix(trimmed, "-") || strings.HasPrefix(trimmed, "*")) && len(trimmed) > 2 {
				actionItems = append(actionItems, strings.Trim(trimmed, "- *\t"))
			}
		}
	}

	summary := strings.Join(summaryLines, "\n\n")

	// Format OKF Archived Document
	var sb strings.Builder
	sb.WriteString("---\n")
	sb.WriteString("type: source\n")
	sb.WriteString("subtype: meeting\n")
	fmt.Fprintf(&sb, "description: Notes and decisions from meeting %s on %s.\n", title, dateStr)
	sb.WriteString("tags:\n")
	sb.WriteString("  - sources/reunioes\n")
	if len(participantLinks) > 0 {
		sb.WriteString("participants:\n")
		for _, pl := range participantLinks {
			fmt.Fprintf(&sb, "  - \"%s\"\n", pl)
		}
	}
	sb.WriteString("---\n\n")

	fmt.Fprintf(&sb, "# 🎙️ %s\n\n", title)
	fmt.Fprintf(&sb, "**Date:** %s\n", dateStr)
	if len(participantLinks) > 0 {
		fmt.Fprintf(&sb, "**Participants:** %s\n\n", strings.Join(participantLinks, ", "))
	} else {
		sb.WriteString("\n")
	}

	sb.WriteString("---\n\n")

	if summary != "" {
		sb.WriteString("## 📌 Executive Summary\n\n")
		sb.WriteString(summary + "\n\n")
	}

	if len(actionItems) > 0 {
		sb.WriteString("## 🎯 Action Items & Next Steps\n\n")
		for _, a := range actionItems {
			fmt.Fprintf(&sb, "- [ ] %s\n", a)
		}
		sb.WriteString("\n")
	}

	destFilename := fmt.Sprintf("%s-%s.md", dateStr, slug)
	destPath := filepath.Join(p.sourcesDir, destFilename)
	_ = os.MkdirAll(p.sourcesDir, 0750)

	if err := os.WriteFile(destPath, []byte(sb.String()), 0600); err != nil { //nolint:gosec // intentional
		return nil, err
	}

	return &MeetingNote{
		SourceFile:   filename,
		Title:        title,
		Date:         dateStr,
		Slug:         slug,
		Participants: participants,
		Summary:      summary,
		ActionItems:  actionItems,
		SavedPath:    filepath.ToSlash(filepath.Join("sources", "reunioes", destFilename)),
	}, nil
}

func (p *Processor) syncPersonInteraction(slug string, dateStr string, title string) {
	personFile := filepath.Join(p.peopleDir, slug+".md")
	data, err := os.ReadFile(personFile) //nolint:gosec // intentional
	if err != nil {
		return
	}

	content := string(data)
	if strings.Contains(content, fmt.Sprintf("- **%s**", dateStr)) || strings.Contains(content, fmt.Sprintf("**%s**", dateStr)) {
		return
	}

	entry := fmt.Sprintf("- **%s** - Meeting: %s.\n", dateStr, title)
	if strings.Contains(content, "## 🤝 Histórico de Interações\n") {
		content = strings.Replace(content, "## 🤝 Histórico de Interações\n", "## 🤝 Histórico de Interações\n"+entry, 1)
		_ = os.WriteFile(personFile, []byte(content), 0600) //nolint:gosec // intentional
	} else if strings.Contains(content, "## 🤝 Recent Interaction History\n") {
		content = strings.Replace(content, "## 🤝 Recent Interaction History\n", "## 🤝 Recent Interaction History\n"+entry, 1)
		_ = os.WriteFile(personFile, []byte(content), 0600) //nolint:gosec // intentional
	} else if strings.Contains(content, "## 🤝 Interactions\n") {
		content = strings.Replace(content, "## 🤝 Interactions\n", "## 🤝 Interactions\n"+entry, 1)
		_ = os.WriteFile(personFile, []byte(content), 0600) //nolint:gosec // intentional
	}
}

func normalizeString(s string) string {
	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	res, _, _ := transform.String(t, strings.ToLower(s))
	reg := regexp.MustCompile(`[^a-z0-9\s]`)
	clean := reg.ReplaceAllString(res, "")
	return strings.TrimSpace(clean)
}

func slugify(s string) string {
	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	res, _, _ := transform.String(t, strings.ToLower(s))
	reg := regexp.MustCompile(`[^a-z0-9\s-]`)
	clean := reg.ReplaceAllString(res, "")
	clean = regexp.MustCompile(`[-\s]+`).ReplaceAllString(clean, "-")
	return strings.Trim(clean, "-")
}
