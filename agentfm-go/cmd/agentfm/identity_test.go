package main

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestSlugifyAgent(t *testing.T) {
	cases := map[string]string{
		"HR Agent (Public)": "hr-agent-public",
		"Finance/Bot":       "finance-bot",
		"  spaced  name  ":  "spaced-name",
		"":                  "",
	}
	for in, want := range cases {
		if got := slugifyAgent(in); got != want {
			t.Errorf("slugifyAgent(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestResolveWorkerIdentityPath(t *testing.T) {
	override, err := resolveWorkerIdentityPath("HR Agent", "/tmp/custom.key")
	if err != nil {
		t.Fatalf("override: %v", err)
	}
	if override != "/tmp/custom.key" {
		t.Errorf("override path = %q, want /tmp/custom.key", override)
	}

	def, err := resolveWorkerIdentityPath("HR Agent (Public)", "")
	if err != nil {
		t.Fatalf("default: %v", err)
	}
	if !strings.HasSuffix(def, filepath.Join(".agentfm", "worker_identity_hr-agent-public.key")) {
		t.Errorf("default path = %q, want suffix worker_identity_hr-agent-public.key", def)
	}

	empty, err := resolveWorkerIdentityPath("", "")
	if err != nil {
		t.Fatalf("empty agent: %v", err)
	}
	if !strings.HasSuffix(empty, filepath.Join(".agentfm", "worker_identity.key")) {
		t.Errorf("empty-agent path = %q, want suffix worker_identity.key", empty)
	}
}
