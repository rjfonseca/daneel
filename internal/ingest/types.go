package ingest

import (
	graphv1 "github.com/rjfonseca/daneel/api/v1"
)

// IngestOptions configures file or content ingestion behavior.
type IngestOptions struct {
	Destination string   `json:"destination,omitempty"` // Target relative vault directory (default: "inbox")
	Recursive   bool     `json:"recursive,omitempty"`   // Recursively traverse directories (default: true)
	Move        bool     `json:"move,omitempty"`        // Remove source file after successful ingestion
	SourceURL   string   `json:"source_url,omitempty"`  // Optional origin URL or reference
	Tags        []string `json:"tags,omitempty"`        // Optional tags to merge into note frontmatter
}

// IngestedNote represents a single successfully processed note.
type IngestedNote struct {
	SourcePath string `json:"source_path,omitempty"`
	RelPath    string `json:"rel_path"`
	Title      string `json:"title"`
	SizeBytes  int64  `json:"size_bytes"`
	Warning    string `json:"warning,omitempty"`
}

// IngestReport summarizes batch or single file ingestion results.
type IngestReport struct {
	Items          []IngestedNote `json:"items"`
	TotalProcessed int            `json:"total_processed"`
	Errors         []string       `json:"errors,omitempty"`
}

// EventHub broadcasts UI events across active subscribers (e.g. *daemon.Hub).
type EventHub interface {
	PublishUIEvent(evt *graphv1.UIEvent)
}
