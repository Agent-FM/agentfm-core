# Graph Report - .  (2026-04-21)

## Corpus Check
- 64 files · ~117,017 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 317 nodes · 514 edges · 25 communities detected
- Extraction: 61% EXTRACTED · 39% INFERRED · 0% AMBIGUOUS · INFERRED: 202 edges (avg confidence: 0.77)
- Token cost: 0 input · 0 output

## Community Hubs (Navigation)
- [[_COMMUNITY_Python SDK & Shared Types|Python SDK & Shared Types]]
- [[_COMMUNITY_Boss & Worker Test Scenarios|Boss & Worker Test Scenarios]]
- [[_COMMUNITY_Governance & Contribution Standards|Governance & Contribution Standards]]
- [[_COMMUNITY_CLI Entrypoints & Discovery|CLI Entrypoints & Discovery]]
- [[_COMMUNITY_Worker Telemetry & UI|Worker Telemetry & UI]]
- [[_COMMUNITY_P2P Protocols & Contracts|P2P Protocols & Contracts]]
- [[_COMMUNITY_Image Generator Agent & Crypto|Image Generator Agent & Crypto]]
- [[_COMMUNITY_Swarm Key & Zip Utilities|Swarm Key & Zip Utilities]]
- [[_COMMUNITY_Boss API Server & Types|Boss API Server & Types]]
- [[_COMMUNITY_Boss API Handler Tests|Boss API Handler Tests]]
- [[_COMMUNITY_Zip Utility Tests|Zip Utility Tests]]
- [[_COMMUNITY_Artifact Zip Pipeline|Artifact Zip Pipeline]]
- [[_COMMUNITY_Sick Leave Agent Polling|Sick Leave Agent Polling]]
- [[_COMMUNITY_Graceful Degradation|Graceful Degradation]]
- [[_COMMUNITY_GenerateSwarmKey|GenerateSwarmKey]]
- [[_COMMUNITY_LoadSwarmKey|LoadSwarmKey]]
- [[_COMMUNITY_Testutil Package Docs|Testutil Package Docs]]
- [[_COMMUNITY_Integration Package Docs|Integration Package Docs]]
- [[_COMMUNITY_Network Constants|Network Constants]]
- [[_COMMUNITY_Version Metadata|Version Metadata]]
- [[_COMMUNITY_Podman Sandbox|Podman Sandbox]]
- [[_COMMUNITY_Image Gen Agent Runner|Image Gen Agent Runner]]
- [[_COMMUNITY_Boss Execute Flow|Boss Execute Flow]]
- [[_COMMUNITY_Boss Async API|Boss Async API]]
- [[_COMMUNITY_Boss Sync API Handlers|Boss Sync API Handlers]]

## God Nodes (most connected - your core abstractions)
1. `WithTimeout()` - 22 edges
2. `NewHost()` - 17 edges
3. `newTestBoss()` - 15 edges
4. `AgentFM — P2P compute grid for containerized agents` - 13 edges
5. `AgentFMClient` - 12 edges
6. `GatewayConnectionError` - 12 edges
7. `ConnectHosts()` - 12 edges
8. `main()` - 12 edges
9. `AgentFMError` - 11 edges
10. `ArtifactManager` - 11 edges

## Surprising Connections (you probably didn't know these)
- `AssertBytesEqual()` --calls--> `TestSendAndHandleArtifactStream_RoundTrip()`  [INFERRED]
  agentfm-go/test/testutil/zip.go → agentfm-go/internal/network/artifacts_test.go
- `Relay node role (lighthouse, DHT server, Circuit Relay v2)` --conceptually_related_to--> `Cloud Relay VPC setup for local dev mesh`  [INFERRED]
  README.md → CONTRIBUTING.md
- `Python SDK (AgentFMClient, LocalMeshGateway, scatter-gather)` --conceptually_related_to--> `Python SDK gateway deps (fastapi, uvicorn, pydantic, httpx, requests)`  [INFERRED]
  README.md → agentfm-python/requirements.txt
- `Security model (transport, auth, DoS, payload, resource budgets)` --conceptually_related_to--> `Security policy & supported versions (1.0.x)`  [INFERRED]
  README.md → SECURITY.md
- `Two-tier Go test suite under race detector (test, test-integration, test-race, test-coverage)` --cites--> `agentfm-go test tree layout (testutil, integration)`  [EXTRACTED]
  README.md → agentfm-go/test/README.md

## Communities

### Community 0 - "Python SDK & Shared Types"
Cohesion: 0.08
Nodes (27): ArtifactManager, Finds the most recently modified .zip file in the watch directory., Polls the directory until a fully downloaded .zip file appears.         Validate, Extracts the zip file and returns a list of paths to the newly extracted files., Deletes the original .zip file to keep the host machine clean., Handles the detection and extraction of P2P payloads downloaded by the Go daemon, AgentFMClient, Natively queues and distributes a list of prompts across the network.         No (+19 more)

### Community 1 - "Boss & Worker Test Scenarios"
Cohesion: 0.15
Nodes (30): HandleArtifactStream(), SendArtifacts(), registerSignallingHandler(), TestHandleArtifactStream_BodyShorterThanHeader(), TestHandleArtifactStream_TaskIDSanitization(), TestHandleArtifactStream_TruncatedPayloads(), TestProgressWriter_PassesThroughBytes(), TestSendAndHandleArtifactStream_RoundTrip() (+22 more)

### Community 2 - "Governance & Contribution Standards"
Cohesion: 0.06
Nodes (36): Contributor Covenant Code of Conduct, conduct@agentfm.network enforcement contact, Enforcement Guidelines (Correction, Warning, Temporary Ban, Permanent Ban), Mozilla code of conduct enforcement ladder (citation), Agent template contributions (CrewAI, AutoGen, LangChain, image gen), Branch naming conventions (feature/, bugfix/, docs/), Cloud Relay VPC setup for local dev mesh, Commit message standards (imperative mood, descriptive) (+28 more)

### Community 3 - "CLI Entrypoints & Discovery"
Cohesion: 0.12
Nodes (22): New(), connectToLighthouse(), discoverPeers(), startDiscovery(), setupHelpMenu(), createHost(), initRoutingAndPubSub(), loadOrGenerateIdentity() (+14 more)

### Community 4 - "Worker Telemetry & UI"
Cohesion: 0.12
Nodes (14): InstallFakeNvidiaSmi(), RequirePOSIX(), getGPUStats(), TestGetGPUStats_MalformedOutput(), TestGetGPUStats_NvidiaSmiErrorExit(), TestGetGPUStats_NvidiaSmiMissing(), TestGetGPUStats_TooFewFields(), TestGetGPUStats_ValidOutput() (+6 more)

### Community 5 - "P2P Protocols & Contracts"
Cohesion: 0.09
Nodes (27): /tmp/output artifact contract (automatic zip + stream), /agentfm/artifacts/1.0.0 stream protocol, Worker circuit breakers (-maxtasks, -maxcpu, -maxgpu), /agentfm/feedback/1.0.0 stream protocol, FLUX.2 image-generator agent example, flux_deploy.py host GPU FastAPI service, Rationale: FLUX.2 needs 30GB+ VRAM — keep model warm on host, use lightweight sandbox, run.py sandbox script that proxies to host via host.docker.internal (+19 more)

### Community 6 - "Image Generator Agent & Crypto"
Cohesion: 0.08
Nodes (10): BaseModel, Initialize with an existing 64-character hex key., Formats the key exactly how the Go libp2p node expects it., Saves the swarm key to disk with strict Unix permissions., Generates and manages libp2p Private Swarm Keys (PSK v1).     Used to create iso, SwarmKey, EndpointFilter, generate_image() (+2 more)

### Community 7 - "Swarm Key & Zip Utilities"
Cohesion: 0.13
Nodes (13): NewLinkedMesh(), TestLoadOrGenerateIdentity_CorruptedFile(), TestLoadOrGenerateIdentity_NewKey(), TestMDNSNotifee_HandlePeerFound_Dials(), TestMDNSNotifee_HandlePeerFound_UnreachablePeer(), GenerateSwarmKey(), LoadSwarmKey(), TestGenerateSwarmKey_UnwritablePath() (+5 more)

### Community 8 - "Boss API Server & Types"
Cohesion: 0.16
Nodes (6): corsMiddleware(), apiWorker, AsyncExecuteRequest, Boss, ExecuteRequest, timeoutReader

### Community 9 - "Boss API Handler Tests"
Cohesion: 0.2
Nodes (13): TestAsyncExecuteHandler_InvalidJSON(), TestAsyncExecuteHandler_InvalidWorkerIDFormat(), TestAsyncExecuteHandler_MethodNotAllowed(), TestAsyncExecuteHandler_WorkerNotFound(), TestHandleExecuteTask_InvalidJSON(), TestHandleExecuteTask_InvalidWorkerIDFormat(), TestHandleExecuteTask_MethodNotAllowed(), TestHandleExecuteTask_WorkerNotFound() (+5 more)

### Community 10 - "Zip Utility Tests"
Cohesion: 0.29
Nodes (0): 

### Community 11 - "Artifact Zip Pipeline"
Cohesion: 0.4
Nodes (3): copyFileIntoZip(), ZipDirectory(), isDirEmpty()

### Community 12 - "Sick Leave Agent Polling"
Cohesion: 1.0
Nodes (0): 

### Community 13 - "Graceful Degradation"
Cohesion: 1.0
Nodes (2): Failure modes & graceful degradation matrix, Rationale: no os.Exit/pterm.Fatal — every failure logs cleanly, no zombies

### Community 14 - "GenerateSwarmKey"
Cohesion: 1.0
Nodes (1): Generates a cryptographically secure 256-bit (32-byte) swarm key.

### Community 15 - "LoadSwarmKey"
Cohesion: 1.0
Nodes (1): Loads and validates an existing swarm key from disk.

### Community 16 - "Testutil Package Docs"
Cohesion: 1.0
Nodes (0): 

### Community 17 - "Integration Package Docs"
Cohesion: 1.0
Nodes (0): 

### Community 18 - "Network Constants"
Cohesion: 1.0
Nodes (0): 

### Community 19 - "Version Metadata"
Cohesion: 1.0
Nodes (0): 

### Community 20 - "Podman Sandbox"
Cohesion: 1.0
Nodes (0): 

### Community 21 - "Image Gen Agent Runner"
Cohesion: 1.0
Nodes (0): 

### Community 22 - "Boss Execute Flow"
Cohesion: 1.0
Nodes (0): 

### Community 23 - "Boss Async API"
Cohesion: 1.0
Nodes (0): 

### Community 24 - "Boss Sync API Handlers"
Cohesion: 1.0
Nodes (0): 

## Knowledge Gaps
- **58 isolated node(s):** `Config`, `Represents an active edge worker on the AgentFM P2P mesh.`, `Generates and manages libp2p Private Swarm Keys (PSK v1).     Used to create iso`, `Initialize with an existing 64-character hex key.`, `Generates a cryptographically secure 256-bit (32-byte) swarm key.` (+53 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **Thin community `Sick Leave Agent Polling`** (2 nodes): `run.py`, `poll_progress()`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Graceful Degradation`** (2 nodes): `Failure modes & graceful degradation matrix`, `Rationale: no os.Exit/pterm.Fatal — every failure logs cleanly, no zombies`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `GenerateSwarmKey`** (1 nodes): `Generates a cryptographically secure 256-bit (32-byte) swarm key.`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `LoadSwarmKey`** (1 nodes): `Loads and validates an existing swarm key from disk.`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Testutil Package Docs`** (1 nodes): `doc.go`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Integration Package Docs`** (1 nodes): `doc.go`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Network Constants`** (1 nodes): `constants.go`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Version Metadata`** (1 nodes): `version.go`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Podman Sandbox`** (1 nodes): `sandbox.go`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Image Gen Agent Runner`** (1 nodes): `run.py`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Boss Execute Flow`** (1 nodes): `execute.go`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Boss Async API`** (1 nodes): `api_async.go`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Boss Sync API Handlers`** (1 nodes): `api_handlers.go`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `WithTimeout()` connect `Boss & Worker Test Scenarios` to `Worker Telemetry & UI`?**
  _High betweenness centrality (0.095) - this node is a cross-community bridge._
- **Why does `NewHost()` connect `Boss & Worker Test Scenarios` to `Boss API Handler Tests`, `Worker Telemetry & UI`, `Swarm Key & Zip Utilities`?**
  _High betweenness centrality (0.068) - this node is a cross-community bridge._
- **Are the 21 inferred relationships involving `WithTimeout()` (e.g. with `TestTelemetry_WorkerProfileFanout_Integration()` and `TestTelemetry_MultipleWorkersFanOutToBoss_Integration()`) actually correct?**
  _`WithTimeout()` has 21 INFERRED edges - model-reasoned connections that need verification._
- **Are the 14 inferred relationships involving `NewHost()` (e.g. with `New()` and `TestTimeoutReader_ReadsNormally()`) actually correct?**
  _`NewHost()` has 14 INFERRED edges - model-reasoned connections that need verification._
- **Are the 14 inferred relationships involving `newTestBoss()` (e.g. with `NewHost()` and `New()`) actually correct?**
  _`newTestBoss()` has 14 INFERRED edges - model-reasoned connections that need verification._
- **Are the 6 inferred relationships involving `AgentFMClient` (e.g. with `WorkerProfile` and `ArtifactManager`) actually correct?**
  _`AgentFMClient` has 6 INFERRED edges - model-reasoned connections that need verification._
- **What connects `Config`, `Represents an active edge worker on the AgentFM P2P mesh.`, `Generates and manages libp2p Private Swarm Keys (PSK v1).     Used to create iso` to the rest of the system?**
  _58 weakly-connected nodes found - possible documentation gaps or missing edges._