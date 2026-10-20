package daemon

import (
	"sync"
	"time"

	graphv1 "github.com/rjfonseca/daneel/api/v1"
)

var (
	defaultHub     *Hub
	defaultHubOnce sync.Once
)

// Hub maintains active context and routes UI events.
type Hub struct {
	mu            sync.RWMutex
	activeContext ActiveContext
	subscribers   map[chan *graphv1.UIEvent]struct{}
}

// NewHub creates a new Hub.
func NewHub() *Hub {
	return &Hub{
		subscribers: make(map[chan *graphv1.UIEvent]struct{}),
	}
}

// GetDefaultHub returns the singleton Hub.
func GetDefaultHub() *Hub {
	defaultHubOnce.Do(func() {
		defaultHub = NewHub()
	})
	return defaultHub
}

// GetActiveContext returns a copy of the current active context.
func (h *Hub) GetActiveContext() ActiveContext {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.activeContext
}

// SetActiveContext updates the active context.
func (h *Hub) SetActiveContext(ctx ActiveContext) {
	h.mu.Lock()
	defer h.mu.Unlock()
	ctx.UpdatedAt = time.Now()
	h.activeContext = ctx
}

// PublishUIEvent broadcasts a UI event to all active subscribers.
func (h *Hub) PublishUIEvent(evt *graphv1.UIEvent) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for ch := range h.subscribers {
		select {
		case ch <- evt:
		default:
			// Non-blocking send; skip if subscriber channel is full
		}
	}
}

// SubscribeUIEvents registers a new subscriber and returns a channel and an unsubscribe function.
func (h *Hub) SubscribeUIEvents() (<-chan *graphv1.UIEvent, func()) {
	h.mu.Lock()
	defer h.mu.Unlock()

	ch := make(chan *graphv1.UIEvent, 100)
	h.subscribers[ch] = struct{}{}

	unsub := func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		delete(h.subscribers, ch)
		close(ch)
	}

	return ch, unsub
}

// BroadcastGalaxyReload creates and broadcasts a GALAXY_RELOAD event.
func (h *Hub) BroadcastGalaxyReload() {
	h.PublishUIEvent(&graphv1.UIEvent{
		Action:    graphv1.UIActionType_ACTION_GALAXY_RELOAD,
		Timestamp: time.Now().UnixMilli(),
	})
}
