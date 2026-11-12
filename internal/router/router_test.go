//nolint:revive // package comment not required
package router

import (
	"testing"
)

func TestRouter_Classify(t *testing.T) {
	r := New()

	tests := []struct {
		input          string
		expectedIntent IntentType
		minConfidence  float64
		expectedParam  string
	}{
		{
			input:          "briefing matinal",
			expectedIntent: IntentRoutineMorning,
			minConfidence:  0.85,
		},
		{
			input:          "morning routine",
			expectedIntent: IntentRoutineMorning,
			minConfidence:  0.85,
		},
		{
			input:          "rotina noturna",
			expectedIntent: IntentRoutineEvening,
			minConfidence:  0.85,
		},
		{
			input:          "status do cofre",
			expectedIntent: IntentVaultHealth,
			minConfidence:  0.85,
		},
		{
			input:          "doctor",
			expectedIntent: IntentVaultHealth,
			minConfidence:  0.85,
		},
		{
			input:          "dirty repos",
			expectedIntent: IntentWorkspace,
			minConfidence:  0.85,
		},
		{
			input:          "workspace status",
			expectedIntent: IntentWorkspace,
			minConfidence:  0.85,
		},
		{
			input:          "briefing Ada Lovelace",
			expectedIntent: IntentPeopleBriefing,
			minConfidence:  0.85,
			expectedParam:  "Ada Lovelace",
		},
		{
			input:          "1:1 with Alan Turing",
			expectedIntent: IntentPeopleBriefing,
			minConfidence:  0.85,
			expectedParam:  "Alan Turing",
		},
		{
			input:          "how do I configure event-driven architecture with Kafka?",
			expectedIntent: IntentNone,
			minConfidence:  0.0,
		},
	}

	for _, tt := range tests {
		match := r.Evaluate(tt.input)
		if tt.expectedIntent == IntentNone {
			if match.Confidence >= 0.85 {
				t.Errorf("input '%s' should not match high confidence, got %s (%.2f)", tt.input, match.Intent, match.Confidence)
			}
			continue
		}

		if match.Intent != tt.expectedIntent {
			t.Errorf("input '%s': expected intent %s, got %s (confidence: %.2f)", tt.input, tt.expectedIntent, match.Intent, match.Confidence)
		}
		if match.Confidence < tt.minConfidence {
			t.Errorf("input '%s': expected confidence >= %.2f, got %.2f", tt.input, tt.minConfidence, match.Confidence)
		}
		if tt.expectedParam != "" && match.Params["name"] != tt.expectedParam {
			t.Errorf("input '%s': expected param name '%s', got '%s'", tt.input, tt.expectedParam, match.Params["name"])
		}
	}
}

func BenchmarkRouter(b *testing.B) {
	r := New()
	queries := []string{
		"briefing matinal",
		"doctor",
		"1:1 with Ada Lovelace",
		"workspace status",
		"how do I configure event-driven architecture with Kafka?",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		q := queries[i%len(queries)]
		_ = r.Evaluate(q)
	}
}
