//nolint:revive // package comment not required
package auditor

import (
	"time"
)

// LearningSignalType classifies the signal discovered from agent sessions.
type LearningSignalType string

const (
	// SignalCorrection indicates the user corrected the agent (conventions, rules, paths).
	SignalCorrection LearningSignalType = "user_correction"
	// SignalRepetition indicates a repetitive multi-step prompt suitable for a new skill.
	SignalRepetition LearningSignalType = "repetitive_workflow"
	// SignalKnowledgeGap indicates a technical question where the agent failed to use the vault.
	SignalKnowledgeGap LearningSignalType = "knowledge_gap"
)

// SessionTurn represents a paired exchange between user input and agent response.
type SessionTurn struct {
	Timestamp      time.Time `json:"timestamp"`
	ConversationID string    `json:"conversation_id"`
	StepIndex      int       `json:"step_index"`
	UserPrompt     string    `json:"user_prompt"`
	AgentReply     string    `json:"agent_reply"`
	ToolNames      []string  `json:"tool_names"`
}

// ExtractedLearning represents a discovered rule, correction, or knowledge gap.
type ExtractedLearning struct {
	SignalType      LearningSignalType `json:"signal_type"`
	Topic           string             `json:"topic"`
	Description     string             `json:"description"`
	SuggestedAction string             `json:"suggested_action"`
	ConversationID  string             `json:"conversation_id,omitempty"`
	Timestamp       time.Time          `json:"timestamp"`
}

// SkillProposal represents a newly mined agent skill candidate based on repetitive prompts.
type SkillProposal struct {
	Name          string `json:"name"`
	Title         string `json:"title"`
	Description   string `json:"description"`
	PromptPattern string `json:"prompt_pattern"`
	Frequency     int    `json:"frequency"`
	DraftContent  string `json:"draft_content"`
}

// AuditOptions controls session scanning and extraction filters.
type AuditOptions struct {
	Since      time.Duration
	MineSkills bool
	Evals      bool
	Apply      bool
	DryRun     bool
	BrainDir   string
	VaultRoot  string
}

// AuditReport summarizes the outcome of auditing agent sessions.
type AuditReport struct {
	Since                string              `json:"since"`
	ScannedConversations int                 `json:"scanned_conversations"`
	ScannedTurns         int                 `json:"scanned_turns"`
	Learnings            []ExtractedLearning `json:"learnings"`
	SkillProposals       []SkillProposal     `json:"skill_proposals"`
	AppliedLearnings     int                 `json:"applied_learnings"`
	AppliedSkills        int                 `json:"applied_skills"`
}

// Interaction represents an agnostic turn of conversation between human and AI agent.
type Interaction struct {
	AgentName      string    `json:"agent_name"` // "antigravity", "claude-code", "cursor"
	ConversationID string    `json:"conversation_id,omitempty"`
	Timestamp      time.Time `json:"timestamp"`
	UserPrompt     string    `json:"user_prompt"`
	AgentReply     string    `json:"agent_reply"`
	ToolsUsed      []string  `json:"tools_used"`
}

// AgentLogScanner defines the contract for reading local logs and transcripts from AI agents.
type AgentLogScanner interface {
	Name() string
	ScanRecent(since time.Duration) ([]Interaction, error)
}

// SuspiciousDelta represents a filtered interaction turn where the agent asked for clarification without using vault tools.
type SuspiciousDelta struct {
	Interaction    Interaction `json:"interaction"`
	ExtractedQuery string      `json:"extracted_query"`
	Reason         string      `json:"reason"`
}

// EvalClassification categorizes agent evaluation discoveries.
type EvalClassification string

const (
	// ClassificationMissedOpportunity indicates information exists in the vault, but the agent interrogated the user.
	ClassificationMissedOpportunity EvalClassification = "missed_opportunity"
	// ClassificationKnowledgeGap indicates information was missing from the vault, signaling a documentation or harvester gap.
	ClassificationKnowledgeGap EvalClassification = "knowledge_gap"
)

// AgentEvalResult represents a classified evaluation finding.
type AgentEvalResult struct {
	Classification  EvalClassification `json:"classification"`
	AgentName       string             `json:"agent_name"`
	ConversationID  string             `json:"conversation_id,omitempty"`
	Timestamp       time.Time          `json:"timestamp"`
	UserPrompt      string             `json:"user_prompt"`
	AgentReply      string             `json:"agent_reply"`
	Doubt           string             `json:"doubt"`
	MatchedNotes    []string           `json:"matched_notes,omitempty"`
	SuggestedAction string             `json:"suggested_action"`
	Details         string             `json:"details"`
}

// AgentEvalSummary aggregates metrics for a specific agent.
type AgentEvalSummary struct {
	ScannedInteractions int `json:"scanned_interactions"`
	SuspiciousDeltas    int `json:"suspicious_deltas"`
	MissedOpportunities int `json:"missed_opportunities"`
	KnowledgeGaps       int `json:"knowledge_gaps"`
}

// AgentEvalReport summarizes an agent evals execution across agents.
type AgentEvalReport struct {
	Since               string                      `json:"since"`
	AgentFilter         string                      `json:"agent_filter"`
	ScannedInteractions int                         `json:"scanned_interactions"`
	SuspiciousDeltas    int                         `json:"suspicious_deltas"`
	MissedOpportunities int                         `json:"missed_opportunities"`
	KnowledgeGaps       int                         `json:"knowledge_gaps"`
	ByAgent             map[string]AgentEvalSummary `json:"by_agent"`
	Results             []AgentEvalResult           `json:"results"`
	AppliedCount        int                         `json:"applied_count"`
}

// AgentEvalOptions specifies options for running agent evals.
type AgentEvalOptions struct {
	Since       time.Duration
	AgentFilter string // "all", "antigravity", "claude-code", "cursor"
	Apply       bool
	DryRun      bool
	BrainDir    string
	ClaudeDir   string
	CursorDir   string
	VaultRoot   string
}
