package boss

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"agentfm/internal/network"
	"agentfm/internal/types"
	"agentfm/test/testutil"

	netcore "github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peerstore"
)

func newBossLinkedToWorker(t *testing.T, workerProfile types.WorkerProfile, handler func(netcore.Stream)) *Boss {
	t.Helper()
	hosts := testutil.NewConnectedMesh(t, 2)
	workerHost, bossHost := hosts[0], hosts[1]

	workerHost.SetStreamHandler(network.TaskProtocol, handler)
	t.Cleanup(func() { workerHost.RemoveStreamHandler(network.TaskProtocol) })

	bossHost.Peerstore().AddAddrs(workerHost.ID(), workerHost.Addrs(), peerstore.PermanentAddrTTL)

	workerProfile.PeerID = workerHost.ID().String()
	if workerProfile.MaxTasks == 0 {
		workerProfile.MaxTasks = 4
	}

	b := New(&network.MeshNode{Host: bossHost})
	b.activeWorkers[workerProfile.PeerID] = workerProfile
	return b
}

func cannedWorkerHandler(t *testing.T, replyChunks ...string) func(netcore.Stream) {
	t.Helper()
	return func(s netcore.Stream) {
		defer func() { _ = s.Close() }()

		if err := s.SetDeadline(time.Now().Add(network.TaskPayloadReadTimeout)); err != nil {
			return
		}
		var payload types.TaskPayload
		limited := io.LimitReader(s, 1*1024*1024)
		if err := json.NewDecoder(limited).Decode(&payload); err != nil {
			return
		}
		if err := s.SetDeadline(time.Now().Add(network.TaskExecutionTimeout)); err != nil {
			return
		}
		for _, chunk := range replyChunks {
			if _, err := s.Write([]byte(chunk)); err != nil {
				return
			}
			time.Sleep(5 * time.Millisecond)
		}
	}
}

func TestOpenAI_ChatCompletions_NonStreaming_RoundTrip_Integration(t *testing.T) {
	const reply = "Hello from the worker.\n"
	b := newBossLinkedToWorker(t,
		types.WorkerProfile{Model: "llama3.2", AgentName: "stub"},
		cannedWorkerHandler(t, reply),
	)

	body := strings.NewReader(`{"model":"llama3.2","messages":[{"role":"user","content":"hi"}]}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", body).
		WithContext(testCtx(t, 10*time.Second))
	rec := httptest.NewRecorder()
	b.handleChatCompletions(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}

	var resp chatCompletionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v (raw=%s)", err, rec.Body.String())
	}
	if resp.Object != "chat.completion" {
		t.Errorf("object = %q, want chat.completion", resp.Object)
	}
	if resp.Model != "llama3.2" {
		t.Errorf("model = %q, want llama3.2", resp.Model)
	}
	if len(resp.Choices) != 1 {
		t.Fatalf("choices = %d, want 1", len(resp.Choices))
	}
	c := resp.Choices[0]
	if c.Message.Role != "assistant" {
		t.Errorf("role = %q, want assistant", c.Message.Role)
	}
	if c.Message.Content != reply {
		t.Errorf("content = %q, want %q", c.Message.Content, reply)
	}
	if c.FinishReason != "stop" {
		t.Errorf("finish_reason = %q, want stop", c.FinishReason)
	}
	if !strings.HasPrefix(resp.ID, "chatcmpl-") {
		t.Errorf("id = %q, want chatcmpl- prefix", resp.ID)
	}
}

func TestOpenAI_ChatCompletions_StripsSentinels_Integration(t *testing.T) {
	b := newBossLinkedToWorker(t,
		types.WorkerProfile{Model: "llama3.2", AgentName: "stub"},
		cannedWorkerHandler(t, "hello\n", "[AGENTFM: NO_FILES]\n", "world\n"),
	)

	body := strings.NewReader(`{"model":"llama3.2","messages":[{"role":"user","content":"hi"}]}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", body).
		WithContext(testCtx(t, 10*time.Second))
	rec := httptest.NewRecorder()
	b.handleChatCompletions(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var resp chatCompletionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	got := resp.Choices[0].Message.Content
	want := "hello\nworld\n"
	if got != want {
		t.Errorf("content = %q, want %q", got, want)
	}
}

func TestOpenAI_ChatCompletions_RoutesByAgentName_Integration(t *testing.T) {
	b := newBossLinkedToWorker(t,
		types.WorkerProfile{Model: "llama3.2", AgentName: "research-agent"},
		cannedWorkerHandler(t, "research result\n"),
	)

	body := strings.NewReader(`{"model":"research-agent","messages":[{"role":"user","content":"go"}]}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", body).
		WithContext(testCtx(t, 10*time.Second))
	rec := httptest.NewRecorder()
	b.handleChatCompletions(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var resp chatCompletionResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.Choices[0].Message.Content != "research result\n" {
		t.Errorf("content = %q, want %q", resp.Choices[0].Message.Content, "research result\n")
	}
}

func TestOpenAI_ChatCompletions_Streaming_RoundTrip_Integration(t *testing.T) {
	b := newBossLinkedToWorker(t,
		types.WorkerProfile{Model: "llama3.2", AgentName: "stub"},
		cannedWorkerHandler(t, "alpha\n", "bravo\n", "[AGENTFM: NO_FILES]\n", "charlie\n"),
	)

	srv := httptest.NewServer(http.HandlerFunc(b.handleChatCompletions))
	t.Cleanup(srv.Close)

	body := strings.NewReader(`{"model":"llama3.2","messages":[{"role":"user","content":"hi"}],"stream":true}`)
	req, err := http.NewRequestWithContext(testCtx(t, 10*time.Second), http.MethodPost, srv.URL, body)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })

	if resp.StatusCode != http.StatusOK {
		buf, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d, want 200; body=%s", resp.StatusCode, string(buf))
	}
	if got := resp.Header.Get("Content-Type"); got != "text/event-stream" {
		t.Errorf("Content-Type = %q, want text/event-stream", got)
	}

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	bodyStr := string(raw)

	if !strings.HasSuffix(bodyStr, "data: [DONE]\n\n") {
		t.Errorf("body must end with data: [DONE]; tail=%q", bodyStr[max(0, len(bodyStr)-80):])
	}

	var contentDeltas []string
	var sawAssistantRole, sawStop bool
	for _, frame := range strings.Split(bodyStr, "\n\n") {
		frame = strings.TrimSpace(frame)
		if frame == "" || frame == "data: [DONE]" {
			continue
		}
		if !strings.HasPrefix(frame, "data: ") {
			t.Errorf("frame missing data: prefix: %q", frame)
			continue
		}
		var chunk chatCompletionChunk
		if err := json.Unmarshal([]byte(strings.TrimPrefix(frame, "data: ")), &chunk); err != nil {
			t.Errorf("decode chunk: %v (raw=%s)", err, frame)
			continue
		}
		if chunk.Object != "chat.completion.chunk" {
			t.Errorf("chunk.object = %q, want chat.completion.chunk", chunk.Object)
		}
		if len(chunk.Choices) != 1 {
			t.Errorf("chunk.choices = %d, want 1", len(chunk.Choices))
			continue
		}
		c := chunk.Choices[0]
		if c.Delta.Role == "assistant" {
			sawAssistantRole = true
		}
		if c.Delta.Content != "" {
			contentDeltas = append(contentDeltas, c.Delta.Content)
		}
		if c.FinishReason != nil && *c.FinishReason == "stop" {
			sawStop = true
		}
	}

	if !sawAssistantRole {
		t.Error("missing initial assistant-role bootstrap frame")
	}
	if !sawStop {
		t.Error("missing terminal frame with finish_reason=stop")
	}

	full := strings.Join(contentDeltas, "")
	want := "alpha\nbravo\ncharlie\n"
	if full != want {
		t.Errorf("reassembled content = %q, want %q (sentinel must be filtered)", full, want)
	}
}

func TestOpenAI_Completions_NonStreaming_RoundTrip_Integration(t *testing.T) {
	const reply = "completion text\n"
	b := newBossLinkedToWorker(t,
		types.WorkerProfile{Model: "llama3.2", AgentName: "stub"},
		cannedWorkerHandler(t, reply),
	)

	body := strings.NewReader(`{"model":"llama3.2","prompt":"finish this"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/completions", body).
		WithContext(testCtx(t, 10*time.Second))
	rec := httptest.NewRecorder()
	b.handleCompletions(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var resp completionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v (raw=%s)", err, rec.Body.String())
	}
	if resp.Object != "text_completion" {
		t.Errorf("object = %q, want text_completion", resp.Object)
	}
	if resp.Model != "llama3.2" {
		t.Errorf("model = %q, want llama3.2", resp.Model)
	}
	if len(resp.Choices) != 1 {
		t.Fatalf("choices = %d, want 1", len(resp.Choices))
	}
	if resp.Choices[0].Text != reply {
		t.Errorf("text = %q, want %q", resp.Choices[0].Text, reply)
	}
	if !strings.HasPrefix(resp.ID, "cmpl-") {
		t.Errorf("id = %q, want cmpl- prefix", resp.ID)
	}
}

func TestOpenAI_Completions_Streaming_RoundTrip_Integration(t *testing.T) {
	b := newBossLinkedToWorker(t,
		types.WorkerProfile{Model: "llama3.2", AgentName: "stub"},
		cannedWorkerHandler(t, "alpha\n", "[AGENTFM: NO_FILES]\n", "bravo\n"),
	)

	srv := httptest.NewServer(http.HandlerFunc(b.handleCompletions))
	t.Cleanup(srv.Close)

	body := strings.NewReader(`{"model":"llama3.2","prompt":"go","stream":true}`)
	req, err := http.NewRequestWithContext(testCtx(t, 10*time.Second), http.MethodPost, srv.URL, body)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })

	if resp.StatusCode != http.StatusOK {
		buf, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d, want 200; body=%s", resp.StatusCode, string(buf))
	}

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	bodyStr := string(raw)
	if !strings.HasSuffix(bodyStr, "data: [DONE]\n\n") {
		t.Errorf("body must end with data: [DONE]; tail=%q", bodyStr[max(0, len(bodyStr)-80):])
	}

	var texts []string
	var sawStop bool
	for _, frame := range strings.Split(bodyStr, "\n\n") {
		frame = strings.TrimSpace(frame)
		if frame == "" || frame == "data: [DONE]" {
			continue
		}
		if !strings.HasPrefix(frame, "data: ") {
			t.Errorf("frame missing data: prefix: %q", frame)
			continue
		}
		var chunk completionChunk
		if err := json.Unmarshal([]byte(strings.TrimPrefix(frame, "data: ")), &chunk); err != nil {
			t.Errorf("decode chunk: %v", err)
			continue
		}
		if chunk.Object != "text_completion" {
			t.Errorf("chunk.object = %q, want text_completion", chunk.Object)
		}
		if len(chunk.Choices) != 1 {
			continue
		}
		c := chunk.Choices[0]
		if c.Text != "" {
			texts = append(texts, c.Text)
		}
		if c.FinishReason != nil && *c.FinishReason == "stop" {
			sawStop = true
		}
	}
	if !sawStop {
		t.Error("missing terminal frame with finish_reason=stop")
	}
	full := strings.Join(texts, "")
	want := "alpha\nbravo\n"
	if full != want {
		t.Errorf("reassembled text = %q, want %q", full, want)
	}
}

func testCtx(t *testing.T, d time.Duration) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), d)
	t.Cleanup(cancel)
	return ctx
}
