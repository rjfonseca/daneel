package spooler

import (
	"encoding/json"
	"time"
)

// Event represents an incoming hook event from an agent.
type Event struct {
	SessionID string          `json:"session_id,omitempty"`
	Event     string          `json:"event,omitempty"`
	Agent     string          `json:"agent,omitempty"`
	CWD       string          `json:"cwd,omitempty"`
	GitRoot   string          `json:"git_root,omitempty"`
	Raw       json.RawMessage `json:"raw,omitempty"`
	Timestamp time.Time       `json:"timestamp"`
}

// SpoolEntry is the structured line serialized as JSONL in the spool storage.
type SpoolEntry struct {
	SessionID string          `json:"session_id"`
	Agent     string          `json:"agent"`
	Event     string          `json:"event"`
	CWD       string          `json:"cwd"`
	GitRoot   string          `json:"git_root,omitempty"`
	Timestamp string          `json:"timestamp"`
	Payload   json.RawMessage `json:"payload"`
}

// SessionSummary summarizes an active or uncompiled session in the spool.
type SessionSummary struct {
	SessionID   string    `json:"session_id"`
	FilePath    string    `json:"file_path"`
	SizeBytes   int64     `json:"size_bytes"`
	EventCount  int       `json:"event_count"`
	LastUpdated time.Time `json:"last_updated"`
}
