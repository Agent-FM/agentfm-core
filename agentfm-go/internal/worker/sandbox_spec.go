package worker

import (
	"errors"
	"fmt"
	"strconv"
)

// This file isolates the container execution *policy* — the exact argv handed
// to `podman run` — from the side effects around it (session IDs, directory
// creation, GPU probing, process spawning). That split exists for one reason:
// BuildRunArgs is pure, so the policy can be asserted in a unit test.
//
// Before this split the argv was assembled inline inside executePodman, which
// spawns a process and therefore cannot be exercised without a container
// runtime. The practical consequence was that removing a security flag broke
// no test. Every hardening item that follows (S2/S3/S4, R1) lands here and
// ships with a test that fails when the flag is removed.
//
// The golden tests in sandbox_spec_test.go pin the CURRENT argv byte for byte.
// They are expected — and intended — to fail the moment hardening is added.
// That failure is the review signal: an argv change to the sandbox must never
// pass unnoticed.

// NetworkMode selects the network namespace the task container runs in.
//
// It is a closed enum validated by BuildRunArgs: an unrecognised value is an
// error, never a silent fallback. NetworkHost mirrors the long-standing
// behaviour; NetworkNone is the first half of roadmap item S4. The egress
// allowlist (reach the model endpoint and nothing else) is the second half and
// is not implemented yet.
type NetworkMode string

const (
	// NetworkHost shares the worker host's network namespace with the task
	// container.
	//
	// It is deliberately permissive: the container reaches loopback services
	// (Ollama on 127.0.0.1:11434, admin endpoints) and cloud instance
	// metadata on 169.254.169.254. On the public mesh the prompt driving the
	// agent comes from an arbitrary peer, so a prompt-injected agent can be
	// steered into any of those. printNetworkPosture (worker.go) surfaces
	// this to the operator at startup. See docs/security.md.
	NetworkHost NetworkMode = "host"

	// NetworkNone gives the task container a network namespace with only its
	// own loopback interface: no route to the worker host, its loopback
	// services, cloud metadata or the internet. Agents that call a model over
	// HTTP (Ollama, vLLM) cannot run in this mode.
	NetworkNone NetworkMode = "none"
)

// ErrInvalidNetworkFlag is returned by ResolveNetworkMode for a
// -sandbox-network value that names no mode.
var ErrInvalidNetworkFlag = errors.New("sandbox network: must be \"host\" or \"none\"")

// ResolveNetworkMode turns the operator's -sandbox-network value into a mode.
//
// An explicit value always wins. An empty value means "pick the default for
// this mesh": a worker on a private swarm only receives tasks from holders of
// the swarm key, so it keeps the historical host networking; a worker on the
// public mesh receives prompts from anyone, so it defaults to NetworkNone and
// the operator must opt back in to host networking knowingly.
func ResolveNetworkMode(flagValue string, privateSwarm bool) (NetworkMode, error) {
	switch flagValue {
	case "":
		if privateSwarm {
			return NetworkHost, nil
		}
		return NetworkNone, nil
	case string(NetworkHost):
		return NetworkHost, nil
	case string(NetworkNone):
		return NetworkNone, nil
	default:
		return "", fmt.Errorf("%w, got %q", ErrInvalidNetworkFlag, flagValue)
	}
}

// defaultGPUDevice is the CDI device string used when the host exposes an
// NVIDIA GPU. It grants the container every GPU on the machine with no VRAM
// ceiling — roadmap item R1 replaces this with a per-task quota.
const defaultGPUDevice = "nvidia.com/gpu=all"

// containerOutputPath is the in-container mount point for the task's output
// directory. The agent contract is that anything written here is collected,
// zipped and returned to the Boss.
const containerOutputPath = "/tmp/output"

// Sentinel errors so callers can branch with errors.Is rather than string
// matching. All are programmer/config errors: a well-formed worker never
// produces them at runtime.
var (
	ErrNoImage            = errors.New("sandbox spec: image is required")
	ErrNoContainerName    = errors.New("sandbox spec: container name is required")
	ErrNoOutputDir        = errors.New("sandbox spec: output dir is required")
	ErrUnsupportedNetwork = errors.New("sandbox spec: unsupported network mode")
)

// SandboxSpec is the complete, self-contained description of one task
// container. It holds no file handles, no context and no host state: every
// value is resolved by the caller before construction, which is what keeps
// BuildRunArgs pure.
//
// Optional fields are expressed as zero values rather than pointers or flags:
// an empty EnvFilePath means "no env file", an empty GPUDevice means "no GPU".
type SandboxSpec struct {
	// ContainerName is the --name passed to podman. The caller derives it
	// from a random session ID so concurrent tasks cannot collide, and the
	// cleanup path can force-remove an orphan by name.
	ContainerName string

	// Image is the OCI image reference to run. Required.
	Image string

	// Prompt is the task text from the Boss.
	//
	// It is currently passed as the container's final argv element, which is
	// how the reference agents read it (agent-example/*/run.py reads
	// sys.argv[1]). This leaks task content to any local process able to read
	// /proc/<pid>/cmdline or run `ps`. Moving it to stdin is roadmap lot 3 and
	// is a breaking change for existing agent images, so it is deliberately
	// NOT part of this refactor.
	Prompt string

	// OutputDir is the host directory bind-mounted at containerOutputPath.
	// Required. The caller must have created it.
	OutputDir string

	// ModelName is exported to the container as AGENTFM_MODEL so the agent
	// knows which local LLM to target. May be empty.
	ModelName string

	// EnvFilePath, when non-empty, is passed as --env-file: every variable in
	// that file enters the container's environment.
	//
	// This is a blanket pass-through with no key filtering, so operator
	// secrets in the file reach the task. Replacing it with an explicit
	// allowlist is roadmap lot 3; it is preserved verbatim here so this
	// refactor stays behaviour-preserving.
	EnvFilePath string

	// GPUDevice, when non-empty, is passed as --device. Empty means the
	// container gets no GPU access at all.
	GPUDevice string

	// Network selects the container's network namespace. Required — the zero
	// value is rejected so a caller cannot fall into a mode by omission.
	Network NetworkMode

	// Limits are the cgroup ceilings for this container (roadmap R1). Zero
	// fields emit no flag, reproducing the historical unbounded behaviour.
	Limits ResourceLimits
}

// BuildRunArgs renders spec into the argument vector for `podman run`.
//
// It is pure: no I/O, no clock, no environment reads, no globals. Given the
// same spec it returns the same slice, which is what lets the golden tests
// assert the exact security posture of the sandbox.
//
// Argument order is part of the contract the golden tests pin. Podman does not
// care about the order of most flags, but a stable order makes diffs of the
// expected argv readable in review — which is the point.
func BuildRunArgs(spec SandboxSpec) ([]string, error) {
	if spec.ContainerName == "" {
		return nil, ErrNoContainerName
	}
	if spec.Image == "" {
		return nil, ErrNoImage
	}
	if spec.OutputDir == "" {
		return nil, ErrNoOutputDir
	}

	args := []string{"run", "--rm", "--name", spec.ContainerName}

	// Closed switch, no default fallthrough to a permissive mode: an
	// unrecognised NetworkMode is an error. The golden tests pin each mode's
	// argv so a change here is reviewed rather than absorbed silently.
	switch spec.Network {
	case NetworkHost:
		args = append(args, "--network", "host")
	case NetworkNone:
		args = append(args, "--network", "none")
	default:
		return nil, fmt.Errorf("network mode %q: %w", spec.Network, ErrUnsupportedNetwork)
	}

	// Resource ceilings (R1). A zero field emits nothing, so an operator who
	// configures no limit gets exactly the pre-R1 argv.
	if err := spec.Limits.Validate(); err != nil {
		return nil, err
	}
	if spec.Limits.CPUs > 0 {
		args = append(args, "--cpus", strconv.FormatFloat(spec.Limits.CPUs, 'f', -1, 64))
	}
	if spec.Limits.MemoryBytes > 0 {
		// --memory-swap is set to the SAME value as --memory, which is how
		// podman expresses "no swap on top of the memory ceiling". Omitting it
		// grants the container an equal amount of swap by default: the
		// container then thrashes the host's disk instead of being killed,
		// which is a worse failure than the one the limit exists to prevent.
		mem := strconv.FormatInt(spec.Limits.MemoryBytes, 10)
		args = append(args, "--memory", mem, "--memory-swap", mem)
	}
	if spec.Limits.PidsLimit > 0 {
		args = append(args, "--pids-limit", strconv.FormatInt(spec.Limits.PidsLimit, 10))
	}

	if spec.GPUDevice != "" {
		args = append(args, "--device", spec.GPUDevice)
	}

	// :z relabels the volume for SELinux shared access. It is a labelling
	// directive, not a confinement one — it does not restrict what the
	// container may do with the mount.
	args = append(args, "-v", spec.OutputDir+":"+containerOutputPath+":z")

	if spec.EnvFilePath != "" {
		args = append(args, "--env-file", spec.EnvFilePath)
	}

	args = append(args, "-e", "AGENTFM_MODEL="+spec.ModelName)

	// Image then prompt, both last: podman treats everything after the image
	// reference as the container's own argv.
	args = append(args, spec.Image, spec.Prompt)

	return args, nil
}
