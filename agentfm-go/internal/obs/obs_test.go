package obs

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
)

func TestJSONHandlerEmitsComponent(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	logger := buildLogger("worker", FormatJSON, "info", &buf)
	logger.Info("hello", slog.String(FieldTaskID, "abc-123"))

	var entry map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &entry); err != nil {
		t.Fatalf("unmarshal: %v\nbody=%q", err, buf.String())
	}
	if entry[FieldComponent] != "worker" {
		t.Errorf("component=%v, want %q", entry[FieldComponent], "worker")
	}
	if entry["msg"] != "hello" {
		t.Errorf("msg=%v, want hello", entry["msg"])
	}
	if entry[FieldTaskID] != "abc-123" {
		t.Errorf("task_id=%v, want abc-123", entry[FieldTaskID])
	}
}

func TestConsoleHandlerEmitsKeyValue(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	logger := buildLogger("boss", FormatConsole, "info", &buf)
	logger.Info("startup", slog.String(FieldPeerID, "12D3K"))

	body := buf.String()
	if !strings.Contains(body, `component=boss`) {
		t.Errorf("expected component=boss in body, got %q", body)
	}
	if !strings.Contains(body, `peer_id=12D3K`) {
		t.Errorf("expected peer_id=12D3K in body, got %q", body)
	}
	if !strings.Contains(body, `msg=startup`) {
		t.Errorf("expected msg=startup in body, got %q", body)
	}
}

func TestLevelFilterDropsBelow(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	logger := buildLogger("relay", FormatJSON, "warn", &buf)
	logger.Info("invisible")
	logger.Debug("also invisible")
	logger.Warn("visible")

	body := buf.String()
	if strings.Contains(body, "invisible") {
		t.Errorf("expected info/debug to be dropped at level=warn, got %q", body)
	}
	if !strings.Contains(body, "visible") {
		t.Errorf("expected warn to be emitted, got %q", body)
	}
}

func TestUnknownFormatFallsBackWithoutPanic(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	// Buffer is not a *os.File so isTerminal returns false → JSON.
	logger := buildLogger("relay", "yaml-please", "info", &buf)
	logger.Info("ok")
	if !strings.Contains(buf.String(), `"msg":"ok"`) {
		t.Errorf("expected fallback to JSON for unknown format, got %q", buf.String())
	}
}

func TestParseLevelAcceptsCommonAliases(t *testing.T) {
	t.Parallel()
	cases := map[string]slog.Level{
		"debug":   slog.LevelDebug,
		"DEBUG":   slog.LevelDebug,
		"info":    slog.LevelInfo,
		"":        slog.LevelInfo,
		"warn":    slog.LevelWarn,
		"warning": slog.LevelWarn,
		"WARN":    slog.LevelWarn,
		"error":   slog.LevelError,
		"garbage": slog.LevelInfo,
	}
	for in, want := range cases {
		if got := parseLevel(in); got != want {
			t.Errorf("parseLevel(%q)=%v, want %v", in, got, want)
		}
	}
}
