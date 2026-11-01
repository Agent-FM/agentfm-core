package worker

import (
	"errors"
	"slices"
	"testing"
)

// The two Golden tests below pin the EXACT argv handed to `podman run`.
//
// If you are reading this because one of them just failed: that is the point.
// Any change to the sandbox's argv changes the isolation posture of every task
// this worker executes, so it must be an explicit, reviewed edit of the
// expected slice — never an incidental diff absorbed by a loose assertion.
//
// They currently encode an UNHARDENED sandbox: host networking, no dropped
// capabilities, a writable root filesystem, no cgroup limits, a blanket
// --env-file pass-through, and the task prompt in argv. That is the state this
// refactor preserves on purpose. Roadmap lots 2 and 3 change it, and these
// tests are the tripwire that forces each change to be seen.

func assertArgs(t *testing.T, got, want []string) {
	t.Helper()
	if slices.Equal(got, want) {
		return
	}
	t.Errorf("podman argv mismatch\n got: %q\nwant: %q", got, want)
	// Point at the first divergence — with ~12 arguments a bare dump of both
	// slices is hard to read in a failure report.
	for i := 0; i < max(len(got), len(want)); i++ {
		var g, w string
		if i < len(got) {
			g = got[i]
		}
		if i < len(want) {
			w = want[i]
		}
		if g != w {
			t.Errorf("first difference at index %d: got %q, want %q", i, g, w)
			return
		}
	}
}

// All four combinations of the two optional inputs are pinned. The two crossed
// cases matter as much as the extremes: --device and --env-file are emitted at
// different points in the vector, so a future edit that reorders them would
// slip past a matrix that only covered "neither" and "both".
func TestBuildRunArgs_Golden(t *testing.T) {
	base := SandboxSpec{
		ContainerName: "agentfm-sandbox-deadbeef",
		Image:         "agentfm-test:v1",
		Prompt:        "draft a sick-leave email",
		OutputDir:     "/tmp/agentfm/run_deadbeef",
		ModelName:     "llama3.2",
		Network:       NetworkHost,
	}

	tests := []struct {
		name   string
		mutate func(*SandboxSpec)
		want   []string
	}{
		{
			name:   "no gpu, no env file",
			mutate: func(*SandboxSpec) {},
			want: []string{
				"run", "--rm", "--name", "agentfm-sandbox-deadbeef",
				"--network", "host",
				"-v", "/tmp/agentfm/run_deadbeef:/tmp/output:z",
				"-e", "AGENTFM_MODEL=llama3.2",
				"agentfm-test:v1", "draft a sick-leave email",
			},
		},
		{
			name:   "gpu, no env file",
			mutate: func(s *SandboxSpec) { s.GPUDevice = defaultGPUDevice },
			want: []string{
				"run", "--rm", "--name", "agentfm-sandbox-deadbeef",
				"--network", "host",
				"--device", "nvidia.com/gpu=all",
				"-v", "/tmp/agentfm/run_deadbeef:/tmp/output:z",
				"-e", "AGENTFM_MODEL=llama3.2",
				"agentfm-test:v1", "draft a sick-leave email",
			},
		},
		{
			name:   "env file, no gpu",
			mutate: func(s *SandboxSpec) { s.EnvFilePath = "/agents/demo/.env" },
			want: []string{
				"run", "--rm", "--name", "agentfm-sandbox-deadbeef",
				"--network", "host",
				"-v", "/tmp/agentfm/run_deadbeef:/tmp/output:z",
				"--env-file", "/agents/demo/.env",
				"-e", "AGENTFM_MODEL=llama3.2",
				"agentfm-test:v1", "draft a sick-leave email",
			},
		},
		{
			name: "gpu and env file",
			mutate: func(s *SandboxSpec) {
				s.GPUDevice = defaultGPUDevice
				s.EnvFilePath = "/agents/demo/.env"
			},
			want: []string{
				"run", "--rm", "--name", "agentfm-sandbox-deadbeef",
				"--network", "host",
				"--device", "nvidia.com/gpu=all",
				"-v", "/tmp/agentfm/run_deadbeef:/tmp/output:z",
				"--env-file", "/agents/demo/.env",
				"-e", "AGENTFM_MODEL=llama3.2",
				"agentfm-test:v1", "draft a sick-leave email",
			},
		},
		{
			name:   "network none (S4)",
			mutate: func(s *SandboxSpec) { s.Network = NetworkNone },
			want: []string{
				"run", "--rm", "--name", "agentfm-sandbox-deadbeef",
				"--network", "none",
				"-v", "/tmp/agentfm/run_deadbeef:/tmp/output:z",
				"-e", "AGENTFM_MODEL=llama3.2",
				"agentfm-test:v1", "draft a sick-leave email",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			spec := base
			tc.mutate(&spec)

			got, err := BuildRunArgs(spec)
			if err != nil {
				t.Fatalf("BuildRunArgs returned an unexpected error: %v", err)
			}
			assertArgs(t, got, tc.want)
		})
	}
}

// Resource ceilings (R1). Pinned separately from the base golden so the two
// concerns fail independently: a broken limit must not read as a broken argv.
func TestBuildRunArgs_GoldenWithResourceLimits(t *testing.T) {
	got, err := BuildRunArgs(SandboxSpec{
		ContainerName: "c",
		Image:         "img",
		Prompt:        "p",
		OutputDir:     "/out",
		ModelName:     "m",
		Network:       NetworkHost,
		Limits: ResourceLimits{
			CPUs:        1.5,
			MemoryBytes: 512 << 20,
			PidsLimit:   1024,
		},
	})
	if err != nil {
		t.Fatalf("BuildRunArgs returned an unexpected error: %v", err)
	}

	assertArgs(t, got, []string{
		"run", "--rm", "--name", "c",
		"--network", "host",
		"--cpus", "1.5",
		"--memory", "536870912", "--memory-swap", "536870912",
		"--pids-limit", "1024",
		"-v", "/out:/tmp/output:z",
		"-e", "AGENTFM_MODEL=m",
		"img", "p",
	})
}

// --memory without --memory-swap at the same value grants the container an
// equal amount of swap, so it thrashes the host's disk instead of dying. The
// pair must never drift apart.
func TestBuildRunArgs_MemoryLimitAlsoDisablesSwap(t *testing.T) {
	got, err := BuildRunArgs(SandboxSpec{
		ContainerName: "c", Image: "img", OutputDir: "/out", Network: NetworkHost,
		Limits: ResourceLimits{MemoryBytes: 256 << 20},
	})
	if err != nil {
		t.Fatalf("BuildRunArgs returned an unexpected error: %v", err)
	}

	memAt := slices.Index(got, "--memory")
	swapAt := slices.Index(got, "--memory-swap")
	if memAt < 0 || swapAt < 0 {
		t.Fatalf("--memory / --memory-swap missing from %q", got)
	}
	if got[memAt+1] != got[swapAt+1] {
		t.Errorf("swap is not disabled: --memory=%s but --memory-swap=%s", got[memAt+1], got[swapAt+1])
	}
}

// Zero limits must emit nothing at all, so an operator who configures no
// ceiling gets byte-for-byte the pre-R1 argv.
func TestBuildRunArgs_ZeroLimitsEmitNoFlags(t *testing.T) {
	got, err := BuildRunArgs(SandboxSpec{
		ContainerName: "c", Image: "img", OutputDir: "/out", Network: NetworkHost,
	})
	if err != nil {
		t.Fatalf("BuildRunArgs returned an unexpected error: %v", err)
	}
	for _, flag := range []string{"--cpus", "--memory", "--memory-swap", "--pids-limit"} {
		if slices.Contains(got, flag) {
			t.Errorf("%s emitted although no limit is configured: %q", flag, got)
		}
	}
}

// An unusable limit must stop the argv from being built at all rather than
// producing a container podman will reject or, worse, accept with a ceiling
// that means something else.
func TestBuildRunArgs_RejectsInvalidLimits(t *testing.T) {
	for _, tc := range []struct {
		name   string
		limits ResourceLimits
	}{
		{"negative cpus", ResourceLimits{CPUs: -1}},
		{"negative memory", ResourceLimits{MemoryBytes: -1}},
		{"negative pids", ResourceLimits{PidsLimit: -1}},
		{"unbootably small memory", ResourceLimits{MemoryBytes: 4096}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			args, err := BuildRunArgs(SandboxSpec{
				ContainerName: "c", Image: "img", OutputDir: "/out",
				Network: NetworkHost, Limits: tc.limits,
			})
			if !errors.Is(err, ErrInvalidLimit) {
				t.Fatalf("got %v, want ErrInvalidLimit", err)
			}
			if args != nil {
				t.Errorf("args must be nil on error, got %q", args)
			}
		})
	}
}

// Optional inputs must be omitted entirely rather than emitted empty: a bare
// `--device` or `--env-file` with no value would shift every following
// argument and make podman consume the next flag as its operand.
func TestBuildRunArgs_OmitsOptionalFlagsWhenUnset(t *testing.T) {
	got, err := BuildRunArgs(SandboxSpec{
		ContainerName: "c",
		Image:         "img",
		OutputDir:     "/out",
		Network:       NetworkHost,
	})
	if err != nil {
		t.Fatalf("BuildRunArgs returned an unexpected error: %v", err)
	}
	for _, flag := range []string{"--device", "--env-file"} {
		if slices.Contains(got, flag) {
			t.Errorf("%s emitted although the corresponding field is unset: %q", flag, got)
		}
	}
}

// Deny-by-default on the network mode. An unrecognised value — including the
// zero value, which is what a caller that forgets the field produces — must be
// an error and never fall through to a permissive namespace.
func TestBuildRunArgs_RejectsUnknownNetworkMode(t *testing.T) {
	for _, mode := range []NetworkMode{"", "bridge", "NONE", "HOST"} {
		t.Run(string(mode), func(t *testing.T) {
			_, err := BuildRunArgs(SandboxSpec{
				ContainerName: "c",
				Image:         "img",
				OutputDir:     "/out",
				Network:       mode,
			})
			if !errors.Is(err, ErrUnsupportedNetwork) {
				t.Fatalf("network mode %q: got error %v, want ErrUnsupportedNetwork", mode, err)
			}
		})
	}
}

// The default must follow who can send this worker a prompt: anyone on the
// public mesh (none), only swarm-key holders on a private swarm (host). An
// explicit flag wins either way, and an unknown value is an error rather than
// a fallback.
func TestResolveNetworkMode(t *testing.T) {
	tests := []struct {
		flag    string
		private bool
		want    NetworkMode
	}{
		{"", false, NetworkNone},
		{"", true, NetworkHost},
		{"host", false, NetworkHost},
		{"none", true, NetworkNone},
	}
	for _, tc := range tests {
		got, err := ResolveNetworkMode(tc.flag, tc.private)
		if err != nil {
			t.Fatalf("ResolveNetworkMode(%q, private=%v): unexpected error %v", tc.flag, tc.private, err)
		}
		if got != tc.want {
			t.Errorf("ResolveNetworkMode(%q, private=%v) = %q, want %q", tc.flag, tc.private, got, tc.want)
		}
	}

	for _, bad := range []string{"bridge", "HOST", "slirp4netns", " none"} {
		if _, err := ResolveNetworkMode(bad, false); !errors.Is(err, ErrInvalidNetworkFlag) {
			t.Errorf("ResolveNetworkMode(%q): got error %v, want ErrInvalidNetworkFlag", bad, err)
		}
	}
}

func TestBuildRunArgs_RejectsIncompleteSpec(t *testing.T) {
	base := SandboxSpec{
		ContainerName: "c",
		Image:         "img",
		OutputDir:     "/out",
		Network:       NetworkHost,
	}

	tests := []struct {
		name    string
		mutate  func(*SandboxSpec)
		wantErr error
	}{
		{"no container name", func(s *SandboxSpec) { s.ContainerName = "" }, ErrNoContainerName},
		{"no image", func(s *SandboxSpec) { s.Image = "" }, ErrNoImage},
		{"no output dir", func(s *SandboxSpec) { s.OutputDir = "" }, ErrNoOutputDir},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			spec := base
			tc.mutate(&spec)

			args, err := BuildRunArgs(spec)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("got error %v, want %v", err, tc.wantErr)
			}
			if args != nil {
				t.Errorf("args must be nil on error, got %q", args)
			}
		})
	}
}

// The reference agents read their task from argv[1]
// (agent-example/*/run.py: sys.argv[1]), so the prompt must stay the final
// element. Lot 3 moves it to stdin — a breaking change for existing agent
// images, which is precisely why it needs its own deliberate commit.
func TestBuildRunArgs_PromptIsFinalArgument(t *testing.T) {
	const prompt = "summarise this document"

	got, err := BuildRunArgs(SandboxSpec{
		ContainerName: "c",
		Image:         "img",
		Prompt:        prompt,
		OutputDir:     "/out",
		Network:       NetworkHost,
	})
	if err != nil {
		t.Fatalf("BuildRunArgs returned an unexpected error: %v", err)
	}
	if len(got) < 2 {
		t.Fatalf("argv too short: %q", got)
	}
	if got[len(got)-1] != prompt {
		t.Errorf("last argument = %q, want the prompt %q", got[len(got)-1], prompt)
	}
	if got[len(got)-2] != "img" {
		t.Errorf("second-to-last argument = %q, want the image reference", got[len(got)-2])
	}
}

// A prompt is attacker-controlled input: it arrives from the Boss. It must be
// carried as a single argv element regardless of the shell metacharacters,
// spaces or newlines it contains. exec.Command passes argv directly without a
// shell, so this holds — the test guards against anyone later introducing
// `sh -c` or string concatenation.
func TestBuildRunArgs_HostilePromptStaysOneArgument(t *testing.T) {
	const hostile = "; rm -rf / # $(id) `whoami`\nsecond line --network host"

	got, err := BuildRunArgs(SandboxSpec{
		ContainerName: "c",
		Image:         "img",
		Prompt:        hostile,
		OutputDir:     "/out",
		Network:       NetworkHost,
	})
	if err != nil {
		t.Fatalf("BuildRunArgs returned an unexpected error: %v", err)
	}

	var seen int
	for _, a := range got {
		if a == hostile {
			seen++
		}
	}
	if seen != 1 {
		t.Fatalf("prompt should appear exactly once as a whole argument, found %d times in %q", seen, got)
	}

	// Detect splitting or interpolation differentially rather than by
	// substring search: build the same spec with a harmless prompt and require
	// every argument except the last to be identical. A substring heuristic
	// cannot work here — the hostile prompt deliberately quotes "--network
	// host", so any argv element it happens to contain would trip a false
	// positive on correct code.
	benign, err := BuildRunArgs(SandboxSpec{
		ContainerName: "c",
		Image:         "img",
		Prompt:        "BENIGN",
		OutputDir:     "/out",
		Network:       NetworkHost,
	})
	if err != nil {
		t.Fatalf("BuildRunArgs (benign) returned an unexpected error: %v", err)
	}
	if len(got) != len(benign) {
		t.Fatalf("prompt changed the argv length: %d vs %d (%q)", len(got), len(benign), got)
	}
	for i := range got[:len(got)-1] {
		if got[i] != benign[i] {
			t.Errorf("argv[%d] differs with a hostile prompt: %q vs %q", i, got[i], benign[i])
		}
	}
}

// BuildRunArgs is pure: identical input yields identical output, and the input
// struct is left untouched. Later lots derive specs from a shared policy
// object, so aliasing bugs here would silently leak one task's settings into
// another's.
func TestBuildRunArgs_IsPure(t *testing.T) {
	spec := SandboxSpec{
		ContainerName: "c",
		Image:         "img",
		Prompt:        "p",
		OutputDir:     "/out",
		ModelName:     "m",
		EnvFilePath:   "/e",
		GPUDevice:     defaultGPUDevice,
		Network:       NetworkHost,
	}
	before := spec

	first, err := BuildRunArgs(spec)
	if err != nil {
		t.Fatalf("first call: %v", err)
	}
	second, err := BuildRunArgs(spec)
	if err != nil {
		t.Fatalf("second call: %v", err)
	}

	if !slices.Equal(first, second) {
		t.Errorf("not deterministic:\nfirst:  %q\nsecond: %q", first, second)
	}
	if spec != before {
		t.Errorf("spec was mutated: got %+v, want %+v", spec, before)
	}

	// Mutating the returned slice must not affect a later call.
	first[0] = "MUTATED"
	third, err := BuildRunArgs(spec)
	if err != nil {
		t.Fatalf("third call: %v", err)
	}
	if third[0] != "run" {
		t.Errorf("returned slice aliases shared state: third call starts with %q", third[0])
	}
}
