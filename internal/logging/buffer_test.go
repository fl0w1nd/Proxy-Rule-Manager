package logging

import (
	"io"
	"log/slog"
	"testing"
	"time"
)

func TestBufferAdd(t *testing.T) {
	buf := NewBuffer(10)
	r := slog.NewRecord(time.Now(), slog.LevelInfo, "hello", 0)
	r.AddAttrs(slog.String("key", "value"))
	buf.Add(r)

	if buf.Count() != 1 {
		t.Errorf("Count = %d, want 1", buf.Count())
	}

	entries := buf.Query("", 10)
	if len(entries) != 1 {
		t.Fatalf("len = %d, want 1", len(entries))
	}
	if entries[0].Message != "hello" {
		t.Errorf("Message = %q", entries[0].Message)
	}
	if entries[0].Attrs["key"] != "value" {
		t.Errorf("Attrs[key] = %q", entries[0].Attrs["key"])
	}
}

func TestBufferRingOverflow(t *testing.T) {
	buf := NewBuffer(3)
	for i := 0; i < 5; i++ {
		r := slog.NewRecord(time.Now(), slog.LevelInfo, "", 0)
		r.AddAttrs(slog.Int("i", i))
		buf.Add(r)
	}

	if buf.Count() != 3 {
		t.Errorf("Count = %d, want 3", buf.Count())
	}

	entries := buf.Query("", 10)
	if len(entries) != 3 {
		t.Fatalf("len = %d, want 3", len(entries))
	}
	// Newest first
	if entries[0].Attrs["i"] != "4" {
		t.Errorf("newest entry i = %q, want 4", entries[0].Attrs["i"])
	}
}

func TestBufferQueryLevel(t *testing.T) {
	buf := NewBuffer(10)
	buf.Add(slog.NewRecord(time.Now(), slog.LevelInfo, "info", 0))
	buf.Add(slog.NewRecord(time.Now(), slog.LevelWarn, "warn", 0))
	buf.Add(slog.NewRecord(time.Now(), slog.LevelError, "error", 0))

	entries := buf.Query("WARN", 10)
	if len(entries) != 1 {
		t.Errorf("len = %d, want 1", len(entries))
	}
}

func TestBufferQueryLimit(t *testing.T) {
	buf := NewBuffer(10)
	for i := 0; i < 5; i++ {
		buf.Add(slog.NewRecord(time.Now(), slog.LevelInfo, "", 0))
	}
	entries := buf.Query("", 2)
	if len(entries) != 2 {
		t.Errorf("len = %d, want 2", len(entries))
	}
}

func TestLoggerWithAttrsReachesBuffer(t *testing.T) {
	buf := NewBuffer(10)
	handler := &multiHandler{
		primary: slog.NewTextHandler(io.Discard, nil),
		buffer:  buf,
	}
	logger := slog.New(handler).With("rule", "example").WithGroup("source").With("label", "upstream")
	logger.Info("compiled", "entries", 3)

	entries := buf.Query("", 1)
	if len(entries) != 1 || entries[0].Attrs["rule"] != "example" ||
		entries[0].Attrs["source.label"] != "upstream" || entries[0].Attrs["source.entries"] != "3" {
		t.Fatalf("buffered attrs: %+v", entries)
	}
}
