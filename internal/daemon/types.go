package daemon

import "time"

// Surface represents the active UI surface.
type Surface string

const (
	SurfaceWeb  Surface = "web"
	SurfaceTUI  Surface = "tui"
	SurfaceNone Surface = "none"
)

// ActiveContext holds the state of the active context.
type ActiveContext struct {
	Surface        Surface   `json:"surface"`
	ActiveNotePath string    `json:"active_note_path"`
	FocusedDomain  string    `json:"focused_domain"`
	FocusedSector  string    `json:"focused_sector,omitempty"`
	SelectedText   string    `json:"selected_text"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// Request is an IPC request payload.
type Request struct {
	Method string      `json:"method"`
	Params interface{} `json:"params,omitempty"`
}

// Response is an IPC response payload.
type Response struct {
	Result interface{} `json:"result,omitempty"`
	Error  string      `json:"error,omitempty"`
}

// UIEventPayload represents the payload to publish a UI event via IPC.
type UIEventPayload struct {
	Action      int32  `json:"action"`
	TargetPath  string `json:"target_path"`
	PayloadJSON string `json:"payload_json"`
}

// HookEventPayload represents an agent hook event received via IPC.
type HookEventPayload struct {
	SessionID string `json:"session_id,omitempty"`
	Event     string `json:"event,omitempty"`
	Agent     string `json:"agent,omitempty"`
	CWD       string `json:"cwd,omitempty"`
	GitRoot   string `json:"git_root,omitempty"`
	Payload   string `json:"payload,omitempty"`
}
