# 🤝 Contributing to AgentFM

First off, thank you for considering contributing to AgentFM! We are building the decentralized AI mesh, and we need your help to make it unstoppable. Whether you are writing core Go network features, building new Python agent templates, or fixing documentation, all contributions are highly appreciated and critical to the mission.

This document outlines the process, technical requirements, and best practices to ensure your code gets merged cleanly and quickly into the grid.

---

## 📑 Table of Contents

1. [Fork and Branch](#1-fork-and-branch)
2. [Local Development & Testing](#2-local-development--testing)
   - [Setting up the Cloud Relay (VPC)](#step-1-setting-up-the-cloud-relay-vpc)
   - [Setting up your Local Environment](#step-2-setting-up-your-local-environment)
3. [Commit Message Standards](#3-write-clean-commit-messages)
4. [Pull Request Process](#4-submit-a-pull-request)
5. [Building Agent Templates (Non-Go)](#5-build-agent-templates)
6. [Code of Conduct](#6-code-of-conduct)

---

### 1. Fork and Branch

Always work on a dedicated branch on your own fork of the repository. **Do not push directly to `main`.**

Please follow these branch naming conventions:
* **`feature/your-feature-name`** — for new additions, capabilities, or major updates.
* **`bugfix/issue-description`** — for squashing bugs and patching vulnerabilities.
* **`docs/what-you-fixed`** — for README, markdown, or documentation updates.

---

### 2. Local Development & Testing

AgentFM is built in Go. Make sure your local environment is up to standard before committing.

**Prerequisites:**
* You must be running **Go 1.25+**.
* You must have access to a small cloud VM (VPC) to act as a relay server for the hole-punching system (required if testing behind a strict NAT environment).

#### Step 1: Setting up the Cloud Relay (VPC)

To properly test P2P networking, set up a permanent lighthouse node on your cloud VM:

1. Clone your fork onto the VM and build the relay server by running:

```sh
make build-relay
```

2. Start the relay server on port 4001:

```sh
./relay -mode relay -port 4001
```

3. You should see an output similar to this:

```sh
🚀 Starting AgentFM Permanent Lighthouse...
🔑 Loaded existing permanent identity from relay_identity.key!

✅ Permanent Relay Node is Online!
--------------------------------------------------
THIS IS YOUR FOREVER ADDRESS (Port 4001):
/ip4/<ip>/tcp/4001/p2p/<peer>
/ip4/127.0.0.1/tcp/4001/p2p/<peer>
/ip6/::1/tcp/4001/p2p/<peer>
/ip6/2a01:4f8:1c19:a81c::1/tcp/4001/p2p/<peer>
--------------------------------------------------
Press CTRL+C to stop the server.
```

4. **Copy and note down** the primary public address (e.g., `/ip4/<ip>/tcp/4001/p2p/<peer>`). You will need this for your local setup.

#### Step 2: Setting up your Local Environment

Now, configure your local machine to connect to your dedicated testing relay.

1. Create or edit the `constants.go` file located at `agentfm-go/internal/network/constants.go`.
2. Update the `PublicLighthouse` constant with the value you got from your cloud VM. Change the `<task>`, `<telemetrytopic>`, `<version>`, etc., to match your testing environment.

Your file should look like this:

```go
package network

const (
	// PubSub Topics
	TelemetryTopic = "agentfm-<telemetrytopic>-<version>"

	// Stream Protocols
	TaskProtocol     = "/agentfm/<task>/<version>"
	FeedbackProtocol = "/agentfm/feedback/<version>"
	ArtifactProtocol = "/agentfm/artifacts/<version>"

	// Discovery Strings
	RendezvousString = "agentfm-rendezvous"
	MDNSServiceTag   = "agentfm-local"
	
	// YOUR CUSTOM RELAY ADDRESS
	PublicLighthouse = "/ip4/<ip>/tcp/4001/p2p/<peer>"
)
```

3. Build the core agent application locally to test your changes:

```sh
make build-agentfm
```

*(Note: Remember not to commit your custom `constants.go` IP overrides if they are hardcoded to your personal VM).*

---

### 3. Write Clean Commit Messages

Clear commit messages help us track the history of the network. Use the imperative mood and be highly descriptive about what the commit changes.

* **Good:** `Add mDNS local discovery fallback`
* **Good:** `Fix memory leak in artifact streaming protocol`
* **Bad:** `fixed the mdns thing`
* **Bad:** `wip updates`

---

### 4. Submit a Pull Request

Once you have tested your changes locally and they are working perfectly, open a Pull Request (PR) against the `main` branch of the official AgentFM repository. 

When submitting your PR, please ensure you:
* **Provide a clear summary:** Explain exactly what the PR does and why it is necessary.
* **Include testing steps:** Provide the exact commands or steps required to test your changes locally.
* **Link related issues:** Mention any open issues your PR resolves (e.g., `Fixes #42` or `Resolves #105`).

---

### 5. Building Agent Templates

Not a Go developer? You can still contribute massively to the ecosystem! 

We are always looking for high-quality, pre-configured AI agent templates to add to the `agent-example/` directory. If you are skilled with Python, Node.js, or AI frameworks, you can build templates for:
* CrewAI
* AutoGen
* LangChain / LlamaIndex
* Image generators (Flux, Stable Diffusion)

**Requirements for Templates:**
Just make sure your template includes a clean, well-documented `Dockerfile` and strictly follows the AgentFM artifact routing standards.

---

### 6. Code of Conduct

By participating in this project, you agree to abide by the [AgentFM Code of Conduct](CODE_OF_CONDUCT.md). We are committed to fostering a welcoming, respectful, and harassment-free environment for all contributors.
