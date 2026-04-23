package boss

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"agentfm/internal/types"
)

// --- writeOpenAIError ------------------------------------------------------

func TestWriteOpenAIError_Shape(t *testing.T) {
	t.Parallel()
	rec := httptest.NewRecorder()
	writeOpenAIError(rec, 418, errTypeInvalidRequest, "teapot", "i am a teapot")

	if rec.Code != 418 {
		t.Fatalf("status = %d, want 418", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", got)
	}

	var body openAIErrorEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v (raw=%s)", err, rec.Body.String())
	}
	want := openAIErrorBody{Message: "i am a teapot", Type: errTypeInvalidRequest, Code: "teapot"}
	if body.Error != want {
		t.Errorf("error body = %+v, want %+v", body.Error, want)
	}
}

// --- renderChatPrompt ------------------------------------------------------

func TestRenderChatPrompt_TableCases(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		messages []ChatMessage
		want     string
	}{
		{
			name:     "empty messages still cues assistant",
			messages: nil,
			want:     "Assistant:",
		},
		{
			name: "system + user",
			messages: []ChatMessage{
				{Role: "system", Content: "be terse"},
				{Role: "user", Content: "hi"},
			},
			want: "System: be terse\n\nUser: hi\n\nAssistant:",
		},
		{
			name: "multi-turn round-trip",
			messages: []ChatMessage{
				{Role: "user", Content: "1"},
				{Role: "assistant", Content: "one"},
				{Role: "user", Content: "2"},
			},
			want: "User: 1\n\nAssistant: one\n\nUser: 2\n\nAssistant:",
		},
		{
			name: "unknown role passthrough",
			messages: []ChatMessage{
				{Role: "tool", Content: "result"},
			},
			want: "tool: result\n\nAssistant:",
		},
		{
			name: "empty content stripped",
			messages: []ChatMessage{
				{Role: "user", Content: ""},
				{Role: "user", Content: "real"},
			},
			want: "User: real\n\nAssistant:",
		},
		{
			name: "empty role defaults to User",
			messages: []ChatMessage{
				{Role: "", Content: "hello"},
			},
			want: "User: hello\n\nAssistant:",
		},
		{
			name: "case-insensitive role canonicalization",
			messages: []ChatMessage{
				{Role: "SYSTEM", Content: "x"},
				{Role: "User", Content: "y"},
			},
			want: "System: x\n\nUser: y\n\nAssistant:",
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := renderChatPrompt(tc.messages)
			if got != tc.want {
				t.Errorf("renderChatPrompt:\n got:  %q\n want: %q", got, tc.want)
			}
		})
	}
}

// --- selectWorkerForModel --------------------------------------------------

func TestSelectWorkerForModel_NoneAdvertised(t *testing.T) {
	t.Parallel()
	_, err := selectWorkerForModel("llama3.2", map[string]types.WorkerProfile{})
	if !errors.Is(err, errModelNotFound) {
		t.Fatalf("err = %v, want errModelNotFound", err)
	}
}

func TestSelectWorkerForModel_EmptyModel(t *testing.T) {
	t.Parallel()
	pool := map[string]types.WorkerProfile{
		"a": {PeerID: "a", Model: "llama3.2", MaxTasks: 4},
	}
	_, err := selectWorkerForModel("   ", pool)
	if !errors.Is(err, errModelNotFound) {
		t.Errorf("err = %v, want errModelNotFound", err)
	}
}

func TestSelectWorkerForModel_PeerIDExactWins(t *testing.T) {
	t.Parallel()
	target := "12D3KooWPeerExact"
	pool := map[string]types.WorkerProfile{
		"loaded": {PeerID: target, AgentName: "x", Model: "x", MaxTasks: 4, CurrentTasks: 3},
		"name":   {PeerID: "12D3KooWOther1", AgentName: target, Model: "x", MaxTasks: 4, CurrentTasks: 0},
		"model":  {PeerID: "12D3KooWOther2", AgentName: "z", Model: target, MaxTasks: 4, CurrentTasks: 0},
	}
	got, err := selectWorkerForModel(target, pool)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if got.PeerID != target {
		t.Errorf("selected PeerID = %q, want exact peer match %q", got.PeerID, target)
	}
}

func TestSelectWorkerForModel_AgentNameWinsOverModel(t *testing.T) {
	t.Parallel()
	pool := map[string]types.WorkerProfile{
		"name":  {PeerID: "p1", AgentName: "research-agent", Model: "llama3.2", MaxTasks: 4},
		"model": {PeerID: "p2", AgentName: "other", Model: "research-agent", MaxTasks: 4},
	}
	got, err := selectWorkerForModel("research-agent", pool)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if got.PeerID != "p1" {
		t.Errorf("selected = %q, want p1 (AgentName tier beats Model tier)", got.PeerID)
	}
}

func TestSelectWorkerForModel_ModelTier_PicksLeastLoaded(t *testing.T) {
	t.Parallel()
	pool := map[string]types.WorkerProfile{
		"busy":  {PeerID: "busy", Model: "llama3.2", MaxTasks: 4, CurrentTasks: 3, CPUUsagePct: 10},
		"idle":  {PeerID: "idle", Model: "llama3.2", MaxTasks: 4, CurrentTasks: 1, CPUUsagePct: 50},
		"mid":   {PeerID: "mid", Model: "llama3.2", MaxTasks: 4, CurrentTasks: 2, CPUUsagePct: 5},
		"other": {PeerID: "other", Model: "flux", MaxTasks: 4, CurrentTasks: 0},
	}
	got, err := selectWorkerForModel("llama3.2", pool)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if got.PeerID != "idle" {
		t.Errorf("selected = %q, want idle (lowest load ratio)", got.PeerID)
	}
}

func TestSelectWorkerForModel_ModelTier_TieBreakOnCPU(t *testing.T) {
	t.Parallel()
	pool := map[string]types.WorkerProfile{
		"hi":  {PeerID: "hi", Model: "llama3.2", MaxTasks: 4, CurrentTasks: 1, CPUUsagePct: 80},
		"low": {PeerID: "low", Model: "llama3.2", MaxTasks: 4, CurrentTasks: 1, CPUUsagePct: 5},
	}
	got, err := selectWorkerForModel("llama3.2", pool)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if got.PeerID != "low" {
		t.Errorf("selected = %q, want low (lower CPU on tie)", got.PeerID)
	}
}

func TestSelectWorkerForModel_AllBusy_AcrossTier(t *testing.T) {
	t.Parallel()
	pool := map[string]types.WorkerProfile{
		"a": {PeerID: "a", Model: "llama3.2", MaxTasks: 2, CurrentTasks: 2},
		"b": {PeerID: "b", Model: "llama3.2", MaxTasks: 1, CurrentTasks: 1},
	}
	_, err := selectWorkerForModel("llama3.2", pool)
	if !errors.Is(err, errMeshOverloaded) {
		t.Errorf("err = %v, want errMeshOverloaded", err)
	}
}

func TestSelectWorkerForModel_CaseInsensitive_AgentNameAndModel(t *testing.T) {
	t.Parallel()
	pool := map[string]types.WorkerProfile{
		"a": {PeerID: "a", AgentName: "ResearchAgent", Model: "Llama3.2", MaxTasks: 4},
	}
	if _, err := selectWorkerForModel("researchagent", pool); err != nil {
		t.Errorf("AgentName match should be case-insensitive: %v", err)
	}
	if _, err := selectWorkerForModel("LLAMA3.2", pool); err != nil {
		t.Errorf("Model match should be case-insensitive: %v", err)
	}
}

func TestSelectWorkerForModel_PeerIDIsCaseSensitive(t *testing.T) {
	t.Parallel()
	pool := map[string]types.WorkerProfile{
		"a": {PeerID: "12D3KooWPeerCase", Model: "x", MaxTasks: 4},
	}
	_, err := selectWorkerForModel("12d3koowpeercase", pool)
	if !errors.Is(err, errModelNotFound) {
		t.Errorf("err = %v, want errModelNotFound (PeerID match must be exact-case)", err)
	}
}

func TestSelectWorkerForModel_MaxTasksZeroTreatedAsUncapped(t *testing.T) {
	t.Parallel()
	pool := map[string]types.WorkerProfile{
		"a": {PeerID: "a", Model: "llama3.2", MaxTasks: 0, CurrentTasks: 99},
	}
	got, err := selectWorkerForModel("llama3.2", pool)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if got.PeerID != "a" {
		t.Errorf("selected = %q, want a", got.PeerID)
	}
}

// --- sentinelFilterReader --------------------------------------------------

func TestSentinelFilter_PassesThroughCleanText(t *testing.T) {
	t.Parallel()
	out, err := io.ReadAll(newSentinelFilter(strings.NewReader("Hello world\n")))
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if got := string(out); got != "Hello world\n" {
		t.Errorf("got %q, want %q", got, "Hello world\n")
	}
}

func TestSentinelFilter_StripsFilesIncoming(t *testing.T) {
	t.Parallel()
	src := "alpha\n[AGENTFM: FILES_INCOMING]\nbeta\n"
	out, err := io.ReadAll(newSentinelFilter(strings.NewReader(src)))
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if got := string(out); got != "alpha\nbeta\n" {
		t.Errorf("got %q, want %q", got, "alpha\nbeta\n")
	}
}

func TestSentinelFilter_StripsNoFiles(t *testing.T) {
	t.Parallel()
	src := "first\n[AGENTFM: NO_FILES]\nlast\n"
	out, err := io.ReadAll(newSentinelFilter(strings.NewReader(src)))
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if got := string(out); got != "first\nlast\n" {
		t.Errorf("got %q, want %q", got, "first\nlast\n")
	}
}

func TestSentinelFilter_PassesThroughErrorMarker(t *testing.T) {
	t.Parallel()
	src := "❌ ERROR: Worker is at max capacity (3/3). Try another worker.\n"
	out, err := io.ReadAll(newSentinelFilter(strings.NewReader(src)))
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if got := string(out); got != src {
		t.Errorf("error marker should pass through verbatim:\n got:  %q\n want: %q", got, src)
	}
}

func TestSentinelFilter_AdjacentSentinels(t *testing.T) {
	t.Parallel()
	src := "[AGENTFM: NO_FILES]\n[AGENTFM: FILES_INCOMING]\nrest\n"
	out, err := io.ReadAll(newSentinelFilter(strings.NewReader(src)))
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if got := string(out); got != "rest\n" {
		t.Errorf("got %q, want %q", got, "rest\n")
	}
}

func TestSentinelFilter_OnlySentinelsYieldsEmpty(t *testing.T) {
	t.Parallel()
	src := "[AGENTFM: NO_FILES]\n"
	out, err := io.ReadAll(newSentinelFilter(strings.NewReader(src)))
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if len(out) != 0 {
		t.Errorf("got %q, want empty", string(out))
	}
}

func TestSentinelFilter_LineSpansSmallReads(t *testing.T) {
	t.Parallel()
	src := "alpha bravo charlie delta echo foxtrot\n[AGENTFM: NO_FILES]\nfinal\n"
	r := newSentinelFilter(strings.NewReader(src))

	var collected bytes.Buffer
	buf := make([]byte, 3)
	for {
		n, err := r.Read(buf)
		if n > 0 {
			collected.Write(buf[:n])
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("read: %v", err)
		}
	}
	want := "alpha bravo charlie delta echo foxtrot\nfinal\n"
	if got := collected.String(); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestSentinelFilter_LeadingWhitespaceSentinel(t *testing.T) {
	t.Parallel()
	src := "  [AGENTFM: NO_FILES]\nkeep\n"
	out, err := io.ReadAll(newSentinelFilter(strings.NewReader(src)))
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if got := string(out); got != "keep\n" {
		t.Errorf("got %q, want %q", got, "keep\n")
	}
}

// --- handleModels ----------------------------------------------------------

func TestHandleModels_MethodNotAllowed(t *testing.T) {
	t.Parallel()
	b := newTestBoss(t)
	req := httptest.NewRequest("POST", "/v1/models", nil)
	rec := httptest.NewRecorder()
	b.handleModels(rec, req)
	if rec.Code != 405 {
		t.Errorf("status = %d, want 405", rec.Code)
	}
	var env openAIErrorEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if env.Error.Code != errCodeMethodNotAllowed {
		t.Errorf("code = %q, want %q", env.Error.Code, errCodeMethodNotAllowed)
	}
}

func TestHandleModels_Empty(t *testing.T) {
	t.Parallel()
	b := newTestBoss(t)
	req := httptest.NewRequest("GET", "/v1/models", nil)
	rec := httptest.NewRecorder()
	b.handleModels(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var resp modelsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v (raw=%s)", err, rec.Body.String())
	}
	if resp.Object != "list" {
		t.Errorf("object = %q, want list", resp.Object)
	}
	if resp.Data == nil {
		t.Error("data must be a non-nil array")
	}
	if len(resp.Data) != 0 {
		t.Errorf("len(data) = %d, want 0", len(resp.Data))
	}
}

func TestHandleModels_OneEntryPerPeer(t *testing.T) {
	t.Parallel()
	b := newTestBoss(t)
	b.activeWorkers["a"] = types.WorkerProfile{
		PeerID: "12D3KooWAlpha", AgentName: "research-agent", Model: "llama3.2", Author: "alice",
	}
	b.activeWorkers["b"] = types.WorkerProfile{
		PeerID: "12D3KooWBravo", AgentName: "research-agent", Model: "llama3.2", Author: "bob",
	}
	b.activeWorkers["c"] = types.WorkerProfile{
		PeerID: "12D3KooWCharlie", AgentName: "image-gen", Model: "flux", Author: "carol",
	}

	req := httptest.NewRequest("GET", "/v1/models", nil)
	rec := httptest.NewRecorder()
	b.handleModels(rec, req)

	var resp modelsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp.Data) != 3 {
		t.Fatalf("len(data) = %d, want 3 (one per peer, no grouping)", len(resp.Data))
	}

	byID := make(map[string]modelEntry, len(resp.Data))
	for _, e := range resp.Data {
		byID[e.ID] = e
	}
	for _, want := range []string{"12D3KooWAlpha", "12D3KooWBravo", "12D3KooWCharlie"} {
		if _, ok := byID[want]; !ok {
			t.Errorf("missing entry id=%q (id must be the peer id)", want)
		}
	}

	for _, listed := range []string{"research-agent", "image-gen", "llama3.2", "flux"} {
		if _, found := byID[listed]; found {
			t.Errorf("id=%q listed; AgentName/Model strings must NOT appear as listed entries (no aliases)", listed)
		}
	}
}

func TestHandleModels_EntryFieldsTraceToProfile(t *testing.T) {
	t.Parallel()
	b := newTestBoss(t)
	b.activeWorkers["a"] = types.WorkerProfile{
		PeerID: "12D3KooWAlpha", AgentName: "research-agent", AgentDesc: "Multi-source research", Model: "llama3.2", Author: "alice",
		Status: "AVAILABLE", CPUCores: 12, CPUUsagePct: 14.2, RAMFreeGB: 12.5,
		CurrentTasks: 1, MaxTasks: 10,
	}
	b.activeWorkers["b"] = types.WorkerProfile{
		PeerID: "12D3KooWBravo", AgentName: "research-agent", AgentDesc: "Bob's variant", Model: "llama3.2", Author: "bob",
		Status: "BUSY", CPUCores: 8, CPUUsagePct: 95.0, RAMFreeGB: 2.1,
		HasGPU: true, GPUUsedGB: 6.4, GPUTotalGB: 24.0, GPUUsagePct: 26.7,
		CurrentTasks: 8, MaxTasks: 10,
	}

	req := httptest.NewRequest("GET", "/v1/models", nil)
	rec := httptest.NewRecorder()
	b.handleModels(rec, req)

	var resp modelsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	byID := make(map[string]modelEntry, len(resp.Data))
	for _, e := range resp.Data {
		byID[e.ID] = e
	}

	alpha, ok := byID["12D3KooWAlpha"]
	if !ok {
		t.Fatal("alpha entry missing")
	}
	if alpha.OwnedBy != "alice" {
		t.Errorf("alpha owned_by = %q, want alice (each entry uses its OWN peer's Author)", alpha.OwnedBy)
	}
	if alpha.Description != "research-agent · llama3.2 — Multi-source research" {
		t.Errorf("alpha description = %q", alpha.Description)
	}
	if alpha.AgentName != "research-agent" || alpha.Engine != "llama3.2" {
		t.Errorf("alpha name/engine = %q/%q", alpha.AgentName, alpha.Engine)
	}
	if alpha.Status != "AVAILABLE" {
		t.Errorf("alpha status = %q", alpha.Status)
	}
	if alpha.Hardware != "llama3.2 (CPU: 12 Cores)" {
		t.Errorf("alpha hardware = %q", alpha.Hardware)
	}
	if alpha.CurrentTasks != 1 || alpha.MaxTasks != 10 {
		t.Errorf("alpha tasks = %d/%d", alpha.CurrentTasks, alpha.MaxTasks)
	}
	if alpha.HasGPU {
		t.Error("alpha should not have GPU")
	}

	bravo, ok := byID["12D3KooWBravo"]
	if !ok {
		t.Fatal("bravo entry missing")
	}
	if bravo.OwnedBy != "bob" {
		t.Errorf("bravo owned_by = %q, want bob (independent from alpha — no consensus)", bravo.OwnedBy)
	}
	if bravo.Description != "research-agent · llama3.2 — Bob's variant" {
		t.Errorf("bravo description = %q", bravo.Description)
	}
	if !bravo.HasGPU {
		t.Error("bravo should have GPU")
	}
	if bravo.Hardware != "llama3.2 (GPU VRAM: 6.4/24.0 GB)" {
		t.Errorf("bravo hardware = %q", bravo.Hardware)
	}
}

func TestHandleModels_OwnedByFallsBackToAgentfmWhenAuthorEmpty(t *testing.T) {
	t.Parallel()
	b := newTestBoss(t)
	b.activeWorkers["a"] = types.WorkerProfile{
		PeerID: "12D3KooWAnon", AgentName: "research-agent", Model: "llama3.2",
	}
	req := httptest.NewRequest("GET", "/v1/models", nil)
	rec := httptest.NewRecorder()
	b.handleModels(rec, req)

	var resp modelsResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if len(resp.Data) != 1 {
		t.Fatalf("len(data) = %d, want 1", len(resp.Data))
	}
	if resp.Data[0].OwnedBy != "agentfm" {
		t.Errorf("owned_by = %q, want agentfm (Author was empty)", resp.Data[0].OwnedBy)
	}
}

func TestHandleModels_CollidingNamesProduceTwoEntries(t *testing.T) {
	t.Parallel()
	b := newTestBoss(t)
	b.activeWorkers["a"] = types.WorkerProfile{
		PeerID: "12D3KooWA", AgentName: "research-agent", Model: "llama3.2", Author: "alice",
	}
	b.activeWorkers["b"] = types.WorkerProfile{
		PeerID: "12D3KooWB", AgentName: "research-agent", Model: "llama3.2", Author: "bob",
	}
	req := httptest.NewRequest("GET", "/v1/models", nil)
	rec := httptest.NewRecorder()
	b.handleModels(rec, req)

	var resp modelsResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if len(resp.Data) != 2 {
		t.Fatalf("len(data) = %d, want 2 (no grouping; colliding names ARE separate entries)", len(resp.Data))
	}
	authors := []string{resp.Data[0].OwnedBy, resp.Data[1].OwnedBy}
	if !((authors[0] == "alice" && authors[1] == "bob") || (authors[0] == "bob" && authors[1] == "alice")) {
		t.Errorf("authors = %v, want one alice and one bob (each peer keeps its OWN author)", authors)
	}
}

func TestHandleModels_NoAgentfmPeersField(t *testing.T) {
	t.Parallel()
	b := newTestBoss(t)
	b.activeWorkers["a"] = types.WorkerProfile{
		PeerID: "12D3KooWZ", AgentName: "research-agent", Model: "llama3.2", Author: "alice",
	}
	req := httptest.NewRequest("GET", "/v1/models", nil)
	rec := httptest.NewRecorder()
	b.handleModels(rec, req)
	if strings.Contains(rec.Body.String(), `"agentfm_peers"`) {
		t.Errorf("response should NOT carry agentfm_peers anymore; body=%s", rec.Body.String())
	}
}

func TestComposeModelDescription_TableCases(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		p    types.WorkerProfile
		want string
	}{
		{"name+engine+desc", types.WorkerProfile{AgentName: "research-agent", Model: "llama3.2", AgentDesc: "Multi-source"}, "research-agent · llama3.2 — Multi-source"},
		{"name+desc only", types.WorkerProfile{AgentName: "research-agent", AgentDesc: "Multi-source"}, "research-agent — Multi-source"},
		{"engine+desc only", types.WorkerProfile{Model: "llama3.2", AgentDesc: "Multi-source"}, "llama3.2 — Multi-source"},
		{"name only", types.WorkerProfile{AgentName: "research-agent"}, "research-agent"},
		{"engine only", types.WorkerProfile{Model: "llama3.2"}, "llama3.2"},
		{"desc only", types.WorkerProfile{AgentDesc: "just a desc"}, "just a desc"},
		{"all empty", types.WorkerProfile{}, ""},
		{"name+engine no desc", types.WorkerProfile{AgentName: "research-agent", Model: "llama3.2"}, "research-agent · llama3.2"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := composeModelDescription(tc.p)
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestHandleModels_CORSHeadersWhenWrapped(t *testing.T) {
	t.Parallel()
	b := newTestBoss(t)
	wrapped := corsMiddleware(b.handleModels)

	req := httptest.NewRequest("OPTIONS", "/v1/models", nil)
	rec := httptest.NewRecorder()
	wrapped(rec, req)

	if rec.Code != 200 {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Errorf("CORS origin = %q, want *", got)
	}
}

// --- handleChatCompletions (pre-dial validation) ---------------------------

func TestHandleChatCompletions_MethodNotAllowed(t *testing.T) {
	t.Parallel()
	b := newTestBoss(t)
	req := httptest.NewRequest("GET", "/v1/chat/completions", nil)
	rec := httptest.NewRecorder()
	b.handleChatCompletions(rec, req)
	if rec.Code != 405 {
		t.Errorf("status = %d, want 405", rec.Code)
	}
}

func TestHandleChatCompletions_InvalidJSON(t *testing.T) {
	t.Parallel()
	b := newTestBoss(t)
	req := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader("not json"))
	rec := httptest.NewRecorder()
	b.handleChatCompletions(rec, req)
	if rec.Code != 400 {
		t.Errorf("status = %d, want 400", rec.Code)
	}
	var env openAIErrorEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if env.Error.Type != errTypeInvalidRequest {
		t.Errorf("type = %q, want %q", env.Error.Type, errTypeInvalidRequest)
	}
}

func TestHandleChatCompletions_EmptyModel(t *testing.T) {
	t.Parallel()
	b := newTestBoss(t)
	body, _ := json.Marshal(map[string]any{
		"model":    "  ",
		"messages": []map[string]string{{"role": "user", "content": "hi"}},
	})
	req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	b.handleChatCompletions(rec, req)
	if rec.Code != 400 {
		t.Errorf("status = %d, want 400", rec.Code)
	}
	var env openAIErrorEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if env.Error.Code != errCodeModelRequired {
		t.Errorf("code = %q, want %q", env.Error.Code, errCodeModelRequired)
	}
}

func TestHandleChatCompletions_EmptyMessages(t *testing.T) {
	t.Parallel()
	b := newTestBoss(t)
	body, _ := json.Marshal(map[string]any{
		"model":    "llama3.2",
		"messages": []any{},
	})
	req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	b.handleChatCompletions(rec, req)
	if rec.Code != 400 {
		t.Errorf("status = %d, want 400", rec.Code)
	}
	var env openAIErrorEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if env.Error.Code != errCodePromptRequired {
		t.Errorf("code = %q, want %q", env.Error.Code, errCodePromptRequired)
	}
}

func TestHandleChatCompletions_ModelNotFound(t *testing.T) {
	t.Parallel()
	b := newTestBoss(t)
	body, _ := json.Marshal(map[string]any{
		"model":    "no-such-model",
		"messages": []map[string]string{{"role": "user", "content": "hi"}},
	})
	req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	b.handleChatCompletions(rec, req)
	if rec.Code != 404 {
		t.Errorf("status = %d, want 404", rec.Code)
	}
	var env openAIErrorEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if env.Error.Code != errCodeModelNotFound {
		t.Errorf("code = %q, want %q", env.Error.Code, errCodeModelNotFound)
	}
}

func TestHandleChatCompletions_AllWorkersBusy(t *testing.T) {
	t.Parallel()
	b := newTestBoss(t)
	b.activeWorkers["a"] = types.WorkerProfile{
		PeerID: "a", Model: "llama3.2", MaxTasks: 1, CurrentTasks: 1,
	}
	body, _ := json.Marshal(map[string]any{
		"model":    "llama3.2",
		"messages": []map[string]string{{"role": "user", "content": "hi"}},
	})
	req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	b.handleChatCompletions(rec, req)
	if rec.Code != 503 {
		t.Errorf("status = %d, want 503", rec.Code)
	}
	var env openAIErrorEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if env.Error.Code != errCodeMeshOverloaded {
		t.Errorf("code = %q, want %q", env.Error.Code, errCodeMeshOverloaded)
	}
}

func TestHandleChatCompletions_StreamModelNotFound(t *testing.T) {
	t.Parallel()
	b := newTestBoss(t)
	body, _ := json.Marshal(map[string]any{
		"model":    "no-such-model",
		"messages": []map[string]string{{"role": "user", "content": "hi"}},
		"stream":   true,
	})
	req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	b.handleChatCompletions(rec, req)
	if rec.Code != 404 {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestHandleChatCompletions_InvalidWorkerPeerID(t *testing.T) {
	t.Parallel()
	b := newTestBoss(t)
	b.activeWorkers["bogus"] = types.WorkerProfile{
		PeerID: "bogus-peer-id", Model: "llama3.2", MaxTasks: 4,
	}
	body, _ := json.Marshal(map[string]any{
		"model":    "llama3.2",
		"messages": []map[string]string{{"role": "user", "content": "hi"}},
	})
	req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	b.handleChatCompletions(rec, req)
	if rec.Code != 500 {
		t.Errorf("status = %d, want 500", rec.Code)
	}
	var env openAIErrorEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if env.Error.Code != errCodeInternalError {
		t.Errorf("code = %q, want %q", env.Error.Code, errCodeInternalError)
	}
}

// --- handleCompletions (pre-dial validation) -------------------------------

func TestHandleCompletions_MethodNotAllowed(t *testing.T) {
	t.Parallel()
	b := newTestBoss(t)
	req := httptest.NewRequest("GET", "/v1/completions", nil)
	rec := httptest.NewRecorder()
	b.handleCompletions(rec, req)
	if rec.Code != 405 {
		t.Errorf("status = %d, want 405", rec.Code)
	}
}

func TestHandleCompletions_InvalidJSON(t *testing.T) {
	t.Parallel()
	b := newTestBoss(t)
	req := httptest.NewRequest("POST", "/v1/completions", strings.NewReader("not json"))
	rec := httptest.NewRecorder()
	b.handleCompletions(rec, req)
	if rec.Code != 400 {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestHandleCompletions_EmptyModel(t *testing.T) {
	t.Parallel()
	b := newTestBoss(t)
	body, _ := json.Marshal(map[string]any{"model": "", "prompt": "hi"})
	req := httptest.NewRequest("POST", "/v1/completions", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	b.handleCompletions(rec, req)
	if rec.Code != 400 {
		t.Errorf("status = %d, want 400", rec.Code)
	}
	var env openAIErrorEnvelope
	_ = json.Unmarshal(rec.Body.Bytes(), &env)
	if env.Error.Code != errCodeModelRequired {
		t.Errorf("code = %q, want %q", env.Error.Code, errCodeModelRequired)
	}
}

func TestHandleCompletions_PromptArrayRejected(t *testing.T) {
	t.Parallel()
	b := newTestBoss(t)
	body, _ := json.Marshal(map[string]any{
		"model":  "llama3.2",
		"prompt": []string{"a", "b"},
	})
	req := httptest.NewRequest("POST", "/v1/completions", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	b.handleCompletions(rec, req)
	if rec.Code != 400 {
		t.Errorf("status = %d, want 400", rec.Code)
	}
	var env openAIErrorEnvelope
	_ = json.Unmarshal(rec.Body.Bytes(), &env)
	if env.Error.Code != errCodeUnsupportedPrompt {
		t.Errorf("code = %q, want %q", env.Error.Code, errCodeUnsupportedPrompt)
	}
}

func TestHandleCompletions_EmptyPromptRejected(t *testing.T) {
	t.Parallel()
	b := newTestBoss(t)
	body, _ := json.Marshal(map[string]any{
		"model":  "llama3.2",
		"prompt": "   ",
	})
	req := httptest.NewRequest("POST", "/v1/completions", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	b.handleCompletions(rec, req)
	if rec.Code != 400 {
		t.Errorf("status = %d, want 400", rec.Code)
	}
	var env openAIErrorEnvelope
	_ = json.Unmarshal(rec.Body.Bytes(), &env)
	if env.Error.Code != errCodePromptRequired {
		t.Errorf("code = %q, want %q", env.Error.Code, errCodePromptRequired)
	}
}

func TestHandleCompletions_MissingPromptRejected(t *testing.T) {
	t.Parallel()
	b := newTestBoss(t)
	body, _ := json.Marshal(map[string]any{"model": "llama3.2"})
	req := httptest.NewRequest("POST", "/v1/completions", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	b.handleCompletions(rec, req)
	if rec.Code != 400 {
		t.Errorf("status = %d, want 400", rec.Code)
	}
	var env openAIErrorEnvelope
	_ = json.Unmarshal(rec.Body.Bytes(), &env)
	if env.Error.Code != errCodePromptRequired {
		t.Errorf("code = %q, want %q", env.Error.Code, errCodePromptRequired)
	}
}

func TestHandleCompletions_ModelNotFound(t *testing.T) {
	t.Parallel()
	b := newTestBoss(t)
	body, _ := json.Marshal(map[string]any{
		"model":  "no-such-model",
		"prompt": "hi",
	})
	req := httptest.NewRequest("POST", "/v1/completions", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	b.handleCompletions(rec, req)
	if rec.Code != 404 {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

// --- newCompletionID -------------------------------------------------------

func TestNewCompletionID_PrefixAndShape(t *testing.T) {
	t.Parallel()
	id := newCompletionID("chatcmpl-")
	if !strings.HasPrefix(id, "chatcmpl-") {
		t.Errorf("id = %q, missing prefix", id)
	}
	if len(id) != len("chatcmpl-")+24 {
		t.Errorf("id length = %d, want %d", len(id), len("chatcmpl-")+24)
	}
	other := newCompletionID("chatcmpl-")
	if other == id {
		t.Errorf("two consecutive ids collided: %q", id)
	}
}
