package request

import (
	"sync"
	"time"
)

// HistoryItem is a locally stored, non-secret record of a request.
type HistoryItem struct {
	ID          int               `json:"id"`
	Timestamp   time.Time         `json:"timestamp"`
	Environment string            `json:"environment"`
	Method      string            `json:"method"`
	Path        string            `json:"path"`
	PathParams  map[string]string `json:"path_params,omitempty"`
	Query       map[string]string `json:"query,omitempty"`
	Headers     map[string]string `json:"headers,omitempty"`
	Body        string            `json:"body,omitempty"`
	AuthMode    AuthMode          `json:"auth_mode"`
	Status      int               `json:"status"`
	DurationMS  int64             `json:"duration_ms"`
	OK          bool              `json:"ok"`
	Error       string            `json:"error,omitempty"`
}

// History is an in-memory ring of recent requests. JWTs are never stored.
type History struct {
	mu      sync.RWMutex
	items   []HistoryItem
	nextID  int
	maxSize int
}

// NewHistory keeps the last maxSize requests.
func NewHistory(maxSize int) *History {
	if maxSize <= 0 {
		maxSize = 50
	}
	return &History{maxSize: maxSize, nextID: 1}
}

// Add appends an item and evicts the oldest if needed.
func (h *History) Add(item HistoryItem) HistoryItem {
	h.mu.Lock()
	defer h.mu.Unlock()
	item.ID = h.nextID
	h.nextID++
	if item.Timestamp.IsZero() {
		item.Timestamp = time.Now()
	}
	h.items = append(h.items, item)
	if len(h.items) > h.maxSize {
		h.items = h.items[len(h.items)-h.maxSize:]
	}
	return item
}

// List returns newest-first copies.
func (h *History) List() []HistoryItem {
	h.mu.RLock()
	defer h.mu.RUnlock()
	out := make([]HistoryItem, len(h.items))
	for i := range h.items {
		out[len(h.items)-1-i] = h.items[i]
	}
	return out
}

// Get returns a history item by id.
func (h *History) Get(id int) (HistoryItem, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, it := range h.items {
		if it.ID == id {
			return it, true
		}
	}
	return HistoryItem{}, false
}

// Clear drops all history.
func (h *History) Clear() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.items = nil
}
