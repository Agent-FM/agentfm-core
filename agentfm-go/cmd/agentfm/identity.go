package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var slugNonAlnum = regexp.MustCompile(`[^a-z0-9]+`)

func slugifyAgent(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	s = slugNonAlnum.ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}

func resolveWorkerIdentityPath(agentName, override string) (string, error) {
	if override != "" {
		return override, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home dir for worker identity: %w", err)
	}
	dir := filepath.Join(home, ".agentfm")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("create identity dir %s: %w", dir, err)
	}
	slug := slugifyAgent(agentName)
	name := "worker_identity.key"
	if slug != "" {
		name = fmt.Sprintf("worker_identity_%s.key", slug)
	}
	return filepath.Join(dir, name), nil
}
