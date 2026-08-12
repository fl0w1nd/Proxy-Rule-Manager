package logging

import (
	"log/slog"
	"sync"
	"time"
)

// LogEntry is one captured log record for API consumption.
type LogEntry struct {
	Time    time.Time         `json:"time"`
	Level   string            `json:"level"`
	Message string            `json:"message"`
	Attrs   map[string]string `json:"attrs,omitempty"`
}

// Buffer is a thread-safe ring buffer that stores the last N log entries.
type Buffer struct {
	mu      sync.RWMutex
	entries []LogEntry
	cap     int
	head    int
	count   int
}

// NewBuffer creates a ring buffer with the given capacity.
func NewBuffer(capacity int) *Buffer {
	if capacity <= 0 {
		capacity = 1000
	}
	return &Buffer{
		entries: make([]LogEntry, capacity),
		cap:     capacity,
	}
}

// Add captures a slog.Record into the ring buffer.
func (b *Buffer) Add(r slog.Record) {
	entry := LogEntry{
		Time:    r.Time,
		Level:   r.Level.String(),
		Message: r.Message,
		Attrs:   make(map[string]string),
	}
	r.Attrs(func(a slog.Attr) bool {
		entry.Attrs[a.Key] = a.Value.String()
		return true
	})

	b.mu.Lock()
	b.entries[b.head] = entry
	b.head = (b.head + 1) % b.cap
	if b.count < b.cap {
		b.count++
	}
	b.mu.Unlock()
}

// Query returns recent log entries filtered by level and limited by count,
// newest first. If level is empty, all levels are returned.
func (b *Buffer) Query(level string, limit int) []LogEntry {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if limit <= 0 || limit > b.count {
		limit = b.count
	}

	result := make([]LogEntry, 0, limit)
	// Iterate from newest (head-1) backwards, stopping at limit.
	for i := 0; i < b.count && len(result) < limit; i++ {
		idx := (b.head - 1 - i + b.cap) % b.cap
		entry := b.entries[idx]
		if level != "" && entry.Level != level {
			continue
		}
		result = append(result, entry)
	}
	return result
}

// Count returns the number of entries currently in the buffer.
func (b *Buffer) Count() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.count
}
