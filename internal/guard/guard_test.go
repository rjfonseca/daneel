//nolint:revive // package comment not required
package guard

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGuardValidator(t *testing.T) {
	tempDir := t.TempDir()

	publicContent := filepath.Join(tempDir, "public", "content")
	_ = os.MkdirAll(publicContent, 0750)

	notesDir := filepath.Join(tempDir, "notes")
	_ = os.MkdirAll(notesDir, 0750)

	// Note with violation: leak link outside public, token, and custom keywords
	unsafeNote := filepath.Join(publicContent, "unsafe.md")
	unsafeContent := `---
type: tecnologia
description: Insecure test
---
# Insecure Note

Link outside public: [Nota](../../notes/privada.md).
Token JWT: eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.
Custom Keyword: confidential
Custom Domain: https://internal.acme.corp/api
WikiLink: [[Other Note]]
`
	_ = os.WriteFile(unsafeNote, []byte(unsafeContent), 0600)

	// Safe Note
	safeNote := filepath.Join(publicContent, "safe.md")
	safeContent := `---
type: tecnologia
description: Safe and audited note
---
# Safe Note

Clean technical content.
`
	_ = os.WriteFile(safeNote, []byte(safeContent), 0600)

	customRules := BuildCustomPatterns([]string{"internal.acme.corp"}, []string{"confidential"})
	val := NewValidator(tempDir, customRules...)

	// Test single file
	rep, err := val.CheckFile(unsafeNote)
	if err != nil {
		t.Fatalf("CheckFile failed: %v", err)
	}

	if len(rep.Issues) == 0 {
		t.Errorf("expected issues in unsafe note, got 0")
	}

	hasZerothLaw := false
	hasFirstLaw := false
	hasSecondLaw := false

	for _, iss := range rep.Issues {
		if iss.Law == ZerothLaw {
			hasZerothLaw = true
		}
		if iss.Law == FirstLaw {
			hasFirstLaw = true
		}
		if iss.Law == SecondLaw {
			hasSecondLaw = true
		}
	}

	if !hasZerothLaw {
		t.Errorf("expected at least one ZerothLaw violation (JWT or restricted domain)")
	}
	if !hasFirstLaw {
		t.Errorf("expected at least one FirstLaw violation (confidential keyword)")
	}
	if !hasSecondLaw {
		t.Errorf("expected at least one SecondLaw violation (unconverted wikilink)")
	}

	// Test safe note
	safeRep, err := val.CheckFile(safeNote)
	if err != nil {
		t.Fatalf("CheckFile on safe note failed: %v", err)
	}
	if len(safeRep.Issues) != 0 {
		t.Errorf("expected 0 issues in safe note, got %d", len(safeRep.Issues))
	}

	// Test full audit
	auditRep, err := val.AuditPublicVault()
	if err != nil {
		t.Fatalf("AuditPublicVault failed: %v", err)
	}
	if auditRep.Passed {
		t.Errorf("expected audit to fail due to unsafe note")
	}
	if auditRep.TotalFilesAudited != 2 {
		t.Errorf("expected 2 files audited, got %d", auditRep.TotalFilesAudited)
	}
}
