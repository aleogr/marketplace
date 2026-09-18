// Package logging builds the process logger.
//
// The output is JSON using the field names Cloud Logging reads, so that a
// deployment on Cloud Run gets severity, message and timestamp without any
// parser configured on the ingestion side (see docs/design.md, section 3).
package logging

import (
	"io"
	"log/slog"
)

// New returns a logger writing JSON to w, discarding entries below level.
func New(w io.Writer, level slog.Level) *slog.Logger {
	handler := slog.NewJSONHandler(w, &slog.HandlerOptions{
		Level:       level,
		ReplaceAttr: cloudLoggingFields,
	})
	return slog.New(handler)
}

// cloudLoggingFields renames slog's top-level keys to the ones Cloud Logging
// recognises and maps slog's levels onto the Cloud Logging severity scale,
// which has no WARN.
func cloudLoggingFields(groups []string, attr slog.Attr) slog.Attr {
	if len(groups) > 0 {
		return attr
	}

	switch attr.Key {
	case slog.MessageKey:
		attr.Key = "message"
	case slog.LevelKey:
		attr.Key = "severity"
		if level, ok := attr.Value.Any().(slog.Level); ok {
			attr.Value = slog.StringValue(severity(level))
		}
	}
	return attr
}

// severity maps a slog level onto the closest Cloud Logging severity.
func severity(level slog.Level) string {
	switch {
	case level < slog.LevelInfo:
		return "DEBUG"
	case level < slog.LevelWarn:
		return "INFO"
	case level < slog.LevelError:
		return "WARNING"
	default:
		return "ERROR"
	}
}
