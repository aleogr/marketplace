package logging_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"testing"
	"time"

	"github.com/aleogr/marketplace/internal/platform/logging"
)

func logOne(t *testing.T, level slog.Level, emit func(*slog.Logger)) map[string]any {
	t.Helper()

	var buf bytes.Buffer
	emit(logging.New(&buf, level))

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("log line is not JSON: %v\nline: %s", err, buf.String())
	}
	return entry
}

// Cloud Logging parses severity, message and time by those names. slog writes
// level, msg and time, so two of the three would arrive as opaque payload and
// every entry would show up with no severity at all.
func TestNewWritesTheFieldNamesCloudLoggingReads(t *testing.T) {
	entry := logOne(t, slog.LevelInfo, func(log *slog.Logger) {
		log.Info("server started", "port", 8080)
	})

	if got := entry["severity"]; got != "INFO" {
		t.Errorf(`entry["severity"] = %v, want "INFO"`, got)
	}
	if got := entry["message"]; got != "server started" {
		t.Errorf(`entry["message"] = %v, want "server started"`, got)
	}
	if got := entry["port"]; got != float64(8080) {
		t.Errorf(`entry["port"] = %v, want 8080`, got)
	}
	for _, absent := range []string{"level", "msg"} {
		if _, ok := entry[absent]; ok {
			t.Errorf("entry still carries the slog field %q", absent)
		}
	}
}

func TestNewWritesAnRFC3339Timestamp(t *testing.T) {
	entry := logOne(t, slog.LevelInfo, func(log *slog.Logger) {
		log.Info("server started")
	})

	value, ok := entry["time"].(string)
	if !ok {
		t.Fatalf(`entry["time"] = %v, want a string`, entry["time"])
	}
	if _, err := time.Parse(time.RFC3339Nano, value); err != nil {
		t.Errorf("time %q is not RFC 3339: %v", value, err)
	}
}

// Cloud Logging's severity scale has no WARN: an unmapped slog warning is
// filed as DEFAULT and disappears from a severity>=WARNING alert filter.
func TestNewMapsWarnToTheCloudLoggingSeverity(t *testing.T) {
	entry := logOne(t, slog.LevelDebug, func(log *slog.Logger) {
		log.Warn("label expired")
	})

	if got := entry["severity"]; got != "WARNING" {
		t.Errorf(`entry["severity"] = %v, want "WARNING"`, got)
	}
}

func TestNewHonoursTheMinimumLevel(t *testing.T) {
	var buf bytes.Buffer
	logging.New(&buf, slog.LevelWarn).Info("this is below the threshold")

	if buf.Len() != 0 {
		t.Errorf("wrote %q, want nothing below the minimum level", buf.String())
	}
}
