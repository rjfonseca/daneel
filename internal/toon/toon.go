//nolint:revive // package comment not required
package toon

import (
	"fmt"
	"strings"

	toonlib "github.com/toon-format/toon-go"
)

// Marshal serializes a Go data structure into TOON bytes.
func Marshal(v any) ([]byte, error) {
	return toonlib.Marshal(v)
}

// MarshalString serializes a Go data structure into a TOON string.
func MarshalString(v any) (string, error) {
	return toonlib.MarshalString(v)
}

// EncodeTabular renders a uniform tabular array in TOON notation with an explicit [N] size guardrail.
// If any cell contains a comma and no explicit delimiter is given, it automatically upgrades to pipe '|'.
func EncodeTabular(name string, headers []string, rows [][]string, explicitDelimiter ...string) string {
	delim := ","
	delimMarker := ""

	if len(explicitDelimiter) > 0 && explicitDelimiter[0] != "" {
		delim = explicitDelimiter[0]
		if delim != "," {
			delimMarker = delim
		}
	} else {
		// Auto-detect if delimiter needs to be upgraded
		hasComma := false
		hasPipe := false
		for _, row := range rows {
			for _, cell := range row {
				if strings.Contains(cell, ",") {
					hasComma = true
				}
				if strings.Contains(cell, "|") {
					hasPipe = true
				}
			}
		}

		if hasComma && !hasPipe {
			delim = "|"
			delimMarker = "|"
		} else if hasComma && hasPipe {
			delim = "\t"
			delimMarker = "\t"
		}
	}

	var sb strings.Builder
	count := len(rows)
	headerJoined := strings.Join(headers, delim)

	// Format: name[N]{h1,h2}: or name[N|]{h1|h2}:
	fmt.Fprintf(&sb, "%s[%d%s]{%s}:\n", name, count, delimMarker, headerJoined)

	for _, row := range rows {
		sb.WriteString("  ")
		sanitizedRow := make([]string, len(headers))
		for i := 0; i < len(headers); i++ {
			var cell string
			if i < len(row) {
				cell = row[i]
			}
			// Sanitize internal newlines to preserve tabular row integrity
			cell = strings.ReplaceAll(cell, "\r\n", " ")
			cell = strings.ReplaceAll(cell, "\n", " ")
			cell = strings.TrimSpace(cell)

			// If cell contains active delimiter, quote it
			if strings.Contains(cell, delim) {
				cell = `"` + strings.ReplaceAll(cell, `"`, `\"`) + `"`
			}
			sanitizedRow[i] = cell
		}
		sb.WriteString(strings.Join(sanitizedRow, delim))
		sb.WriteString("\n")
	}

	return sb.String()
}

// EstimateTokens calculates an approximate token count for LLM prompts.
// Takes into account punctuation density which inflates subword tokenizers.
func EstimateTokens(s string) int {
	if len(s) == 0 {
		return 0
	}
	// Measure punctuation overhead (JSON and Markdown have high punctuation ratios)
	punctCount := 0
	for _, r := range s {
		if r == '{' || r == '}' || r == '[' || r == ']' || r == '"' || r == ':' || r == '#' || r == '>' || r == '*' {
			punctCount++
		}
	}
	// Base token estimate: 1 token ~ 4 characters, plus 0.5 additional tokens per structural delimiter
	baseTokens := len(s) / 4
	total := baseTokens + (punctCount / 2)
	if total < 1 {
		return 1
	}
	return total
}

// CalculateSavings returns estimated tokens for JSON vs TOON and the percentage saved.
func CalculateSavings(rawJSON, rawTOON string) (jsonTokens, toonTokens int, savingsPct float64) {
	jsonTokens = EstimateTokens(rawJSON)
	toonTokens = EstimateTokens(rawTOON)
	if jsonTokens <= 0 {
		return 0, toonTokens, 0.0
	}
	savings := float64(jsonTokens-toonTokens) / float64(jsonTokens) * 100.0
	return jsonTokens, toonTokens, savings
}
