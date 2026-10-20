//nolint:revive // package comment not required
package lore

import (
	"strings"
	"testing"
)

func TestGetRandomQuote(t *testing.T) {
	q := GetRandomQuote()
	if len(q.Quote) == 0 {
		t.Errorf("expected non-empty quote text")
	}
	if len(q.Speaker) == 0 {
		t.Errorf("expected non-empty speaker")
	}
	if len(q.Source) == 0 {
		t.Errorf("expected non-empty source")
	}

	formatted := FormatQuote(q)
	if !strings.Contains(formatted, q.Speaker) {
		t.Errorf("expected formatted quote to contain speaker '%s', got '%s'", q.Speaker, formatted)
	}
}
