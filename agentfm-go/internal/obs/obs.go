// Package obs configures the AgentFM structured-logging spine.
//
// Each binary calls Init(component, format, level) once at startup. After
// that, slog.Default() is the configured logger, and standard fields
// (component, peer_id, task_id, protocol, err) ride on every entry.
//
// Format selection rule:
//   - "json"    — slog.JSONHandler, intended for log shippers / ELK / Loki
//   - "console" — slog.TextHandler, single-line key=value, grep-friendly
//   - "auto"    — TTY detection: console if stdout is a terminal, json otherwise
//
// pterm-styled output (TUI banners, interactive boxes, the boss radar)
// is intentionally NOT routed through slog. Those are user-facing UI and
// stay decorative.
package obs

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	"github.com/mattn/go-isatty"
)

// Format selectors accepted by Init.
const (
	FormatJSON    = "json"
	FormatConsole = "console"
	FormatAuto    = "auto"
)

// Standard log field names. Use these in slog.String / slog.Any calls so
// every component agrees on the schema.
const (
	FieldComponent   = "component"
	FieldPeerID      = "peer_id"
	FieldTaskID      = "task_id"
	FieldProtocol    = "protocol"
	FieldErr         = "err"
	FieldRemoteAddr  = "remote_addr"
	FieldRoute       = "route"
	FieldAuthOutcome = "auth_outcome"
)

// Init configures slog.Default() for the calling binary.
//
// component is baked into every log entry as "component" so log aggregators
// can filter without parsing call-site info.
//
// format must be FormatJSON, FormatConsole, or FormatAuto (case-insensitive);
// any other value falls back to FormatAuto with a one-time warning on stderr.
//
// level must be one of debug, info, warn, error (case-insensitive).
func Init(component, format, level string) {
	slog.SetDefault(buildLogger(component, format, level, os.Stdout))
}

// buildLogger is the testable seam — pure construction, no global mutation.
func buildLogger(component, format, level string, w io.Writer) *slog.Logger {
	lvl := parseLevel(level)
	handler := buildHandler(format, w, lvl)
	return slog.New(handler).With(slog.String(FieldComponent, component))
}

func parseLevel(level string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func buildHandler(format string, w io.Writer, lvl slog.Level) slog.Handler {
	opts := &slog.HandlerOptions{Level: lvl}
	switch strings.ToLower(strings.TrimSpace(format)) {
	case FormatJSON:
		return slog.NewJSONHandler(w, opts)
	case FormatConsole:
		return slog.NewTextHandler(w, opts)
	case FormatAuto, "":
		if isTerminal(w) {
			return slog.NewTextHandler(w, opts)
		}
		return slog.NewJSONHandler(w, opts)
	default:
		fmt.Fprintf(os.Stderr, "obs: unknown log format %q, falling back to auto\n", format)
		if isTerminal(w) {
			return slog.NewTextHandler(w, opts)
		}
		return slog.NewJSONHandler(w, opts)
	}
}

// isTerminal returns true iff w is an *os.File pointing at a TTY.
func isTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	return isatty.IsTerminal(f.Fd())
}
