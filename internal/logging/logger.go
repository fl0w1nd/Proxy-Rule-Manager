// Package logging provides structured logging via log/slog with a ring buffer
// for API-queryable recent log entries.
package logging

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strings"
)

// Setup configures a slog logger. mode can be "json" (production) or "text"
// (development). The buffer captures all log entries for API queries.
func Setup(mode string, buf *Buffer) *slog.Logger {
	var baseHandler slog.Handler
	var w io.Writer = os.Stderr

	opts := &slog.HandlerOptions{Level: slog.LevelDebug}

	switch mode {
	case "json":
		baseHandler = slog.NewJSONHandler(w, opts)
	default:
		baseHandler = slog.NewTextHandler(w, opts)
	}

	handler := &multiHandler{
		primary: baseHandler,
		buffer:  buf,
	}

	logger := slog.New(handler)
	slog.SetDefault(logger)
	return logger
}

// multiHandler fans out log records to both a primary handler and the buffer.
type multiHandler struct {
	primary     slog.Handler
	buffer      *Buffer
	bufferAttrs []slog.Attr
	groups      []string
}

func (h *multiHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.primary.Enabled(ctx, level)
}

func (h *multiHandler) Handle(ctx context.Context, r slog.Record) error {
	if h.buffer != nil {
		bufferRecord := slog.NewRecord(r.Time, r.Level, r.Message, r.PC)
		bufferRecord.AddAttrs(h.bufferAttrs...)
		r.Attrs(func(attr slog.Attr) bool {
			bufferRecord.AddAttrs(prefixAttr(attr, h.groups))
			return true
		})
		h.buffer.Add(bufferRecord)
	}
	return h.primary.Handle(ctx, r)
}

func (h *multiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	bufferAttrs := append([]slog.Attr(nil), h.bufferAttrs...)
	for _, attr := range attrs {
		bufferAttrs = append(bufferAttrs, prefixAttr(attr, h.groups))
	}
	return &multiHandler{
		primary:     h.primary.WithAttrs(attrs),
		buffer:      h.buffer,
		bufferAttrs: bufferAttrs,
		groups:      append([]string(nil), h.groups...),
	}
}

func (h *multiHandler) WithGroup(name string) slog.Handler {
	groups := append([]string(nil), h.groups...)
	if name != "" {
		groups = append(groups, name)
	}
	return &multiHandler{
		primary:     h.primary.WithGroup(name),
		buffer:      h.buffer,
		bufferAttrs: append([]slog.Attr(nil), h.bufferAttrs...),
		groups:      groups,
	}
}

func prefixAttr(attr slog.Attr, groups []string) slog.Attr {
	if len(groups) == 0 || attr.Key == "" {
		return attr
	}
	attr.Key = strings.Join(groups, ".") + "." + attr.Key
	return attr
}

var _ slog.Handler = (*multiHandler)(nil)
