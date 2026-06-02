package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestWitnessMode_BootsAndExits(t *testing.T) {
	if testing.Short() {
		t.Skip("witness boot smoke test needs the full binary")
	}

	tmp := t.TempDir()

	bin := filepath.Join(t.TempDir(), "agentfm")
	build := exec.Command("go", "build", "-o", bin, "./cmd/agentfm")
	build.Dir = "../.."
	build.Stdout = os.Stdout
	build.Stderr = os.Stderr
	if err := build.Run(); err != nil {
		t.Fatalf("build agentfm: %v", err)
	}

	swarmKeyPath := filepath.Join(tmp, "swarm.key")
	pskBytes := []byte("/key/swarm/psk/1.0.0/\n/base16/\n" +
		"0000000000000000000000000000000000000000000000000000000000000001\n")
	if err := os.WriteFile(swarmKeyPath, pskBytes, 0o600); err != nil {
		t.Fatalf("write swarm key: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, bin, "-mode", "witness", "-port", "0", "-swarmkey", swarmKeyPath)
	cmd.Env = append(os.Environ(), "HOME="+tmp)
	cmd.Dir = tmp
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	if err := cmd.Start(); err != nil {
		t.Fatalf("start witness: %v", err)
	}

	dbPath := filepath.Join(tmp, ".agentfm", "witness_ledger.db")
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(dbPath); err == nil {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	if _, err := os.Stat(dbPath); err != nil {
		_ = cmd.Process.Kill()
		t.Fatalf("witness never created %s within 15s: %v\nstderr:\n%s\nstdout:\n%s", dbPath, err, errBuf.String(), outBuf.String())
	}

	if err := cmd.Process.Signal(os.Interrupt); err != nil {
		_ = cmd.Process.Kill()
		t.Fatalf("signal interrupt: %v\nstderr:\n%s\nstdout:\n%s", err, errBuf.String(), outBuf.String())
	}
	if err := cmd.Wait(); err != nil {
		t.Logf("witness exited: %v (expected on signal)", err)
	}
}
