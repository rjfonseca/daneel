package routine

import (
	"context"
	"time"
)

// Routine represents the domain model of a scheduled background routine.
type Routine struct {
	Name        string   `json:"name" yaml:"name"`               // e.g. "tidy", "harvest", "doctor"
	Command     string   `json:"command" yaml:"command"`         // daneel subcommand to execute
	Args        []string `json:"args,omitempty" yaml:"args"`     // Additional CLI arguments
	Schedule    string   `json:"schedule" yaml:"schedule"`       // "hourly", "daily", "0 */6 * * *", etc.
	Description string   `json:"description" yaml:"description"` // Human-readable description
	Enabled     bool     `json:"enabled" yaml:"enabled"`         // Whether the routine is enabled
}

// RoutineStatus represents the operational runtime state of a routine.
type RoutineStatus struct {
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Schedule    string    `json:"schedule"`
	Enabled     bool      `json:"enabled"`
	Active      bool      `json:"active"`
	LastTrigger time.Time `json:"last_trigger,omitempty"`
	NextTrigger time.Time `json:"next_trigger,omitempty"`
	LastResult  string    `json:"last_result,omitempty"` // "success", "failed", "running", "unknown"
}

// SchedulerPort defines the Ports & Adapters abstraction for background routine scheduling.
type SchedulerPort interface {
	Name() string
	Install(ctx context.Context, r Routine) error
	Uninstall(ctx context.Context, name string) error
	Enable(ctx context.Context, name string) error
	Disable(ctx context.Context, name string) error
	List(ctx context.Context) ([]RoutineStatus, error)
	Status(ctx context.Context, name string) (*RoutineStatus, error)
	RunNow(ctx context.Context, name string) error
	Logs(ctx context.Context, name string, lines int, follow bool) error
}
