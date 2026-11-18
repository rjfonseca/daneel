//nolint:revive // package comment not required
package search

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"unicode"
)

const (
	// DefaultChunkMaxWords is the default maximum word count per chunk.
	DefaultChunkMaxWords = 350
	// DefaultChunkOverlapWords is the default word overlap between split paragraph chunks.
	DefaultChunkOverlapWords = 40
)

// Chunk represents a structurally split section or sub-section of a Markdown note.
type Chunk struct {
	RelPath     string `json:"rel_path"`
	ChunkIdx    int    `json:"chunk_idx"`
	Heading     string `json:"heading"`
	Content     string `json:"content"`
	ContentHash string `json:"content_hash"`
}

// ChunkMarkdown splits Markdown content into structurally coherent chunks.
// It splits on headings (# through ####) while respecting fenced code blocks (``` / ~~~),
// and further splits sections exceeding maxWords on paragraph boundaries with overlapWords overlap.
// Each chunk's Content is prefixed with hierarchical context ("title > heading_breadcrumb")
// and assigned a deterministic SHA-256 ContentHash.
func ChunkMarkdown(relPath, title, content string, maxWords, overlapWords int) []Chunk {
	if maxWords <= 0 {
		maxWords = DefaultChunkMaxWords
	}
	if overlapWords < 0 {
		overlapWords = DefaultChunkOverlapWords
	}
	if overlapWords >= maxWords {
		overlapWords = maxWords / 2
	}

	body := stripLeadingFrontmatter(content)
	if strings.TrimSpace(body) == "" {
		return nil
	}

	type rawSection struct {
		heading string
		body    string
	}

	lines := strings.Split(body, "\n")
	var sections []rawSection
	var currentLines []string
	var headings [5]string // 1-indexed for #..####

	inFence := false
	var fenceChar byte
	fenceLen := 0

	flushSection := func() {
		text := strings.TrimSpace(strings.Join(currentLines, "\n"))
		if text != "" {
			sections = append(sections, rawSection{
				heading: buildBreadcrumb(headings),
				body:    text,
			})
		}
		currentLines = nil
	}

	for _, line := range lines {
		trimmedLeft := strings.TrimLeft(line, " ")
		leadingSpaces := len(line) - len(trimmedLeft)

		if leadingSpaces <= 3 {
			if fc, fl, rest, ok := parseFenceLine(trimmedLeft); ok {
				if !inFence {
					inFence = true
					fenceChar = fc
					fenceLen = fl
				} else if fc == fenceChar && fl >= fenceLen && strings.TrimSpace(rest) == "" {
					inFence = false
					fenceChar = 0
					fenceLen = 0
				}
				currentLines = append(currentLines, line)
				continue
			}
		}

		if !inFence && leadingSpaces <= 3 {
			if level, headingText, ok := parseHeadingLine(trimmedLeft); ok && level >= 1 && level <= 4 {
				flushSection()
				headings[level] = headingText
				for i := level + 1; i <= 4; i++ {
					headings[i] = ""
				}
				continue
			}
		}

		currentLines = append(currentLines, line)
	}
	flushSection()

	if len(sections) == 0 {
		return nil
	}

	var chunks []Chunk
	for _, sec := range sections {
		parts := splitSectionByWords(sec.body, maxWords, overlapWords)
		for _, part := range parts {
			indexedContent := formatChunkContext(title, sec.heading, part)
			sum := sha256.Sum256([]byte(indexedContent))
			chunks = append(chunks, Chunk{
				RelPath:     relPath,
				ChunkIdx:    len(chunks),
				Heading:     sec.heading,
				Content:     indexedContent,
				ContentHash: hex.EncodeToString(sum[:]),
			})
		}
	}

	return chunks
}

func stripLeadingFrontmatter(content string) string {
	normalized := strings.ReplaceAll(content, "\r\n", "\n")
	if !strings.HasPrefix(normalized, "---\n") {
		return normalized
	}
	rest := normalized[4:]
	idx := strings.Index(rest, "\n---\n")
	if idx >= 0 {
		return rest[idx+5:]
	}
	if strings.HasSuffix(rest, "\n---") {
		return ""
	}
	return normalized
}

func parseFenceLine(trimmed string) (char byte, length int, rest string, ok bool) {
	if len(trimmed) < 3 {
		return 0, 0, "", false
	}
	ch := trimmed[0]
	if ch != '`' && ch != '~' {
		return 0, 0, "", false
	}
	count := 0
	for count < len(trimmed) && trimmed[count] == ch {
		count++
	}
	if count < 3 {
		return 0, 0, "", false
	}
	rest = trimmed[count:]
	if ch == '`' && strings.ContainsRune(rest, '`') {
		return 0, 0, "", false
	}
	return ch, count, rest, true
}

func parseHeadingLine(trimmed string) (level int, text string, ok bool) {
	if !strings.HasPrefix(trimmed, "#") {
		return 0, "", false
	}
	i := 0
	for i < len(trimmed) && trimmed[i] == '#' {
		i++
	}
	if i == 0 || i > 6 || i >= len(trimmed) {
		return 0, "", false
	}
	if trimmed[i] != ' ' && trimmed[i] != '\t' {
		return 0, "", false
	}
	heading := strings.TrimSpace(trimmed[i:])
	heading = strings.TrimRight(heading, "#")
	heading = strings.TrimSpace(heading)
	if heading == "" {
		return 0, "", false
	}
	return i, heading, true
}

func buildBreadcrumb(headings [5]string) string {
	var parts []string
	for i := 1; i <= 4; i++ {
		if headings[i] != "" {
			parts = append(parts, headings[i])
		}
	}
	return strings.Join(parts, " > ")
}

func formatChunkContext(title, headingBreadcrumb, body string) string {
	cleanTitle := strings.TrimSpace(title)
	cleanHeading := strings.TrimSpace(headingBreadcrumb)

	var prefix string
	switch {
	case cleanTitle != "" && cleanHeading != "":
		if strings.EqualFold(cleanTitle, cleanHeading) {
			prefix = cleanTitle
		} else {
			prefix = cleanTitle + " > " + cleanHeading
		}
	case cleanTitle != "":
		prefix = cleanTitle
	case cleanHeading != "":
		prefix = cleanHeading
	}

	if prefix == "" {
		return body
	}
	return prefix + "\n\n" + body
}

func splitSectionByWords(body string, maxWords, overlapWords int) []string {
	if countWords(body) <= maxWords {
		return []string{body}
	}

	paragraphs := splitParagraphs(body)
	var rawChunks []string
	var current []string
	currentWords := 0

	flush := func() {
		if len(current) == 0 {
			return
		}
		rawChunks = append(rawChunks, strings.Join(current, "\n\n"))
		current = nil
		currentWords = 0
	}

	for _, para := range paragraphs {
		pWords := countWords(para)
		if pWords == 0 {
			continue
		}

		// If a single paragraph itself exceeds maxWords, flush current and split the paragraph by words.
		if pWords > maxWords {
			flush()
			wordParts := splitLongParagraph(para, maxWords, overlapWords)
			rawChunks = append(rawChunks, wordParts...)
			continue
		}

		if currentWords > 0 && currentWords+pWords > maxWords {
			prevText := strings.Join(current, "\n\n")
			flush()
			if overlapWords > 0 {
				overlap := trailingWords(prevText, overlapWords)
				if overlap != "" && countWords(overlap)+pWords <= maxWords {
					current = append(current, overlap)
					currentWords += countWords(overlap)
				}
			}
		}

		current = append(current, para)
		currentWords += pWords
	}
	flush()

	return rawChunks
}

func splitParagraphs(body string) []string {
	lines := strings.Split(body, "\n")
	var paragraphs []string
	var current []string
	inFence := false
	var fenceChar byte
	fenceLen := 0

	flush := func() {
		p := strings.TrimSpace(strings.Join(current, "\n"))
		if p != "" {
			paragraphs = append(paragraphs, p)
		}
		current = nil
	}

	for _, line := range lines {
		trimmedLeft := strings.TrimLeft(line, " ")
		if len(line)-len(trimmedLeft) <= 3 {
			if fc, fl, rest, ok := parseFenceLine(trimmedLeft); ok {
				if !inFence {
					inFence = true
					fenceChar = fc
					fenceLen = fl
				} else if fc == fenceChar && fl >= fenceLen && strings.TrimSpace(rest) == "" {
					inFence = false
					fenceChar = 0
					fenceLen = 0
				}
			}
		}

		if !inFence && strings.TrimSpace(line) == "" {
			flush()
			continue
		}
		current = append(current, line)
	}
	flush()
	return paragraphs
}

func splitLongParagraph(para string, maxWords, overlapWords int) []string {
	words := strings.Fields(para)
	if len(words) <= maxWords {
		return []string{para}
	}

	step := maxWords - overlapWords
	if step < 1 {
		step = 1
	}

	var parts []string
	for start := 0; start < len(words); start += step {
		end := start + maxWords
		if end > len(words) {
			end = len(words)
		}
		parts = append(parts, strings.Join(words[start:end], " "))
		if end == len(words) {
			break
		}
	}
	return parts
}

func countWords(s string) int {
	inWord := false
	count := 0
	for _, r := range s {
		if unicode.IsSpace(r) {
			inWord = false
		} else if !inWord {
			inWord = true
			count++
		}
	}
	return count
}

func trailingWords(s string, n int) string {
	if n <= 0 {
		return ""
	}
	words := strings.Fields(s)
	if len(words) == 0 {
		return ""
	}
	if len(words) <= n {
		return strings.Join(words, " ")
	}
	return strings.Join(words[len(words)-n:], " ")
}
