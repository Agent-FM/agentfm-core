package version

// AppVersion is the human-readable version reported by `agentfm --help`,
// the API gateway banner, telemetry, and Prometheus metrics. Overridden
// at build time via:
//
//	-ldflags "-X agentfm/internal/version.AppVersion=<v>"
//
// (see Makefile's build-all target). The const literal below is the
// fallback for `go build` / `go run` invocations that skip ldflags.
const AppVersion = "1.3.0"
