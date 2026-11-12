package toon

import (
	"strings"
	"testing"
)

type sampleUser struct {
	ID   int    `toon:"id"`
	Name string `toon:"name"`
	Role string `toon:"role"`
}

type samplePayload struct {
	Users []sampleUser `toon:"users"`
}

func TestEncodeTabular_Simple(t *testing.T) {
	headers := []string{"title", "path", "score"}
	rows := [][]string{
		{"Architecture Decision", "notes/arch.md", "9.50"},
		{"API Security", "notes/sec.md", "8.20"},
	}

	result := EncodeTabular("results", headers, rows)

	expectedHeader := "results[2]{title,path,score}:"
	if !strings.Contains(result, expectedHeader) {
		t.Errorf("expected header %q, got: %s", expectedHeader, result)
	}

	expectedRow1 := "  Architecture Decision,notes/arch.md,9.50"
	if !strings.Contains(result, expectedRow1) {
		t.Errorf("expected row %q, got: %s", expectedRow1, result)
	}
}

func TestEncodeTabular_AutoDelimiterPipeWhenCommaPresent(t *testing.T) {
	headers := []string{"title", "snippet"}
	rows := [][]string{
		{"Clean Code", "Write tests, keep functions short, and refactor."},
		{"TDD", "Red, green, refactor cycle."},
	}

	result := EncodeTabular("results", headers, rows)

	// Since rows have commas, it should automatically choose pipe '|'
	expectedHeader := "results[2|]{title|snippet}:"
	if !strings.Contains(result, expectedHeader) {
		t.Errorf("expected header %q, got: %s", expectedHeader, result)
	}

	expectedRow1 := "  Clean Code|Write tests, keep functions short, and refactor."
	if !strings.Contains(result, expectedRow1) {
		t.Errorf("expected row %q, got: %s", expectedRow1, result)
	}
}

func TestEncodeTabular_Empty(t *testing.T) {
	headers := []string{"title", "path"}
	var rows [][]string

	result := EncodeTabular("results", headers, rows)
	expectedHeader := "results[0]{title,path}:"
	if !strings.Contains(result, expectedHeader) {
		t.Errorf("expected header %q, got: %s", expectedHeader, result)
	}
}

func TestEncodeTabular_NewlineSanitization(t *testing.T) {
	headers := []string{"id", "description"}
	rows := [][]string{
		{"task-1", "First line\nSecond line\r\nThird line"},
	}

	result := EncodeTabular("tasks", headers, rows)
	if strings.Contains(result, "\nSecond") {
		t.Errorf("newlines inside cell should be escaped or flattened, got: %s", result)
	}
	if !strings.Contains(result, "First line Second line Third line") && !strings.Contains(result, `First line\nSecond line\nThird line`) {
		t.Errorf("expected sanitized content, got: %s", result)
	}
}

func TestMarshalStruct(t *testing.T) {
	payload := samplePayload{
		Users: []sampleUser{
			{ID: 1, Name: "Ada Lovelace", Role: "Staff Engineer"},
			{ID: 2, Name: "Alan Turing", Role: "Principal Engineer"},
		},
	}

	out, err := MarshalString(payload)
	if err != nil {
		t.Fatalf("MarshalString failed: %v", err)
	}

	if !strings.Contains(out, "users[2]{id,name,role}:") {
		t.Errorf("expected tabular users[2], got: %s", out)
	}
	if !strings.Contains(out, "Ada Lovelace") {
		t.Errorf("expected Ada Lovelace, got: %s", out)
	}
}

func TestCalculateSavings(t *testing.T) {
	rawJSON := `{"results": [{"title": "Test Title", "path": "notes/test.md", "score": 9.5}, {"title": "Test Title 2", "path": "notes/test2.md", "score": 8.0}]}`
	rawTOON := "results[2]{title,path,score}:\n  Test Title,notes/test.md,9.5\n  Test Title 2,notes/test2.md,8.0\n"

	jsonTok, toonTok, savings := CalculateSavings(rawJSON, rawTOON)
	if jsonTok <= toonTok {
		t.Errorf("expected jsonTok (%d) > toonTok (%d)", jsonTok, toonTok)
	}
	if savings <= 0 {
		t.Errorf("expected positive savings percentage, got %.2f%%", savings)
	}
}
