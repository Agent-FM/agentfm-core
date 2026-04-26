package boss

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"agentfm/internal/network"
	"agentfm/internal/obs"
	"agentfm/internal/types"
	"agentfm/internal/version"

	netcore "github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
)

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatCompletionRequest struct {
	Model    string        `json:"model"`
	Messages []ChatMessage `json:"messages"`
	Stream   bool          `json:"stream"`
}

type CompletionRequest struct {
	Model  string `json:"model"`
	Prompt any    `json:"prompt"`
	Stream bool   `json:"stream"`
}

type chatChoice struct {
	Index        int         `json:"index"`
	Message      ChatMessage `json:"message"`
	FinishReason string      `json:"finish_reason"`
}

type chatChoiceDelta struct {
	Index        int         `json:"index"`
	Delta        ChatMessage `json:"delta"`
	FinishReason *string     `json:"finish_reason"`
}

type usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type chatCompletionResponse struct {
	ID      string       `json:"id"`
	Object  string       `json:"object"`
	Created int64        `json:"created"`
	Model   string       `json:"model"`
	Choices []chatChoice `json:"choices"`
	Usage   usage        `json:"usage"`
}

type chatCompletionChunk struct {
	ID      string            `json:"id"`
	Object  string            `json:"object"`
	Created int64             `json:"created"`
	Model   string            `json:"model"`
	Choices []chatChoiceDelta `json:"choices"`
}

type completionChoice struct {
	Index        int    `json:"index"`
	Text         string `json:"text"`
	FinishReason string `json:"finish_reason"`
}

type completionChoiceDelta struct {
	Index        int     `json:"index"`
	Text         string  `json:"text"`
	FinishReason *string `json:"finish_reason"`
}

type completionResponse struct {
	ID      string             `json:"id"`
	Object  string             `json:"object"`
	Created int64              `json:"created"`
	Model   string             `json:"model"`
	Choices []completionChoice `json:"choices"`
	Usage   usage              `json:"usage"`
}

type completionChunk struct {
	ID      string                  `json:"id"`
	Object  string                  `json:"object"`
	Created int64                   `json:"created"`
	Model   string                  `json:"model"`
	Choices []completionChoiceDelta `json:"choices"`
}

type modelEntry struct {
	ID          string `json:"id"`
	Object      string `json:"object"`
	Created     int64  `json:"created"`
	OwnedBy     string `json:"owned_by"`
	Description string `json:"description,omitempty"`

	AgentName    string  `json:"agentfm_name,omitempty"`
	Engine       string  `json:"agentfm_engine,omitempty"`
	Status       string  `json:"agentfm_status,omitempty"`
	Hardware     string  `json:"agentfm_hardware,omitempty"`
	CurrentTasks int     `json:"agentfm_current_tasks"`
	MaxTasks     int     `json:"agentfm_max_tasks"`
	CPUUsagePct  float64 `json:"agentfm_cpu_usage_pct"`
	RAMFreeGB    float64 `json:"agentfm_ram_free_gb"`
	HasGPU       bool    `json:"agentfm_has_gpu"`
	GPUUsedGB    float64 `json:"agentfm_gpu_used_gb"`
	GPUTotalGB   float64 `json:"agentfm_gpu_total_gb"`
	GPUUsagePct  float64 `json:"agentfm_gpu_usage_pct"`
}

func profileToModelEntry(p types.WorkerProfile, created int64) modelEntry {
	aw := profileToAPIWorker(p)
	owner := p.Author
	if owner == "" {
		owner = "agentfm"
	}
	return modelEntry{
		ID:           p.PeerID,
		Object:       "model",
		Created:      created,
		OwnedBy:      owner,
		Description:  composeModelDescription(p),
		AgentName:    p.AgentName,
		Engine:       p.Model,
		Status:       p.Status,
		Hardware:     aw.Hardware,
		CurrentTasks: p.CurrentTasks,
		MaxTasks:     p.MaxTasks,
		CPUUsagePct:  p.CPUUsagePct,
		RAMFreeGB:    p.RAMFreeGB,
		HasGPU:       p.HasGPU,
		GPUUsedGB:    p.GPUUsedGB,
		GPUTotalGB:   p.GPUTotalGB,
		GPUUsagePct:  p.GPUUsagePct,
	}
}

func composeModelDescription(p types.WorkerProfile) string {
	var head string
	switch {
	case p.AgentName != "" && p.Model != "":
		head = p.AgentName + " · " + p.Model
	case p.AgentName != "":
		head = p.AgentName
	case p.Model != "":
		head = p.Model
	}
	switch {
	case head != "" && p.AgentDesc != "":
		return head + " — " + p.AgentDesc
	case head != "":
		return head
	default:
		return p.AgentDesc
	}
}

type modelsResponse struct {
	Object string       `json:"object"`
	Data   []modelEntry `json:"data"`
}

type openAIErrorBody struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    string `json:"code"`
}

type openAIErrorEnvelope struct {
	Error openAIErrorBody `json:"error"`
}

const (
	errTypeInvalidRequest = "invalid_request_error"
	errTypeServerError    = "server_error"

	errCodeMethodNotAllowed   = "method_not_allowed"
	errCodeInvalidRequest     = "invalid_request_error"
	errCodeModelRequired      = "model_required"
	errCodePromptRequired     = "prompt_required"
	errCodeUnsupportedPrompt  = "unsupported_prompt_type"
	errCodeModelNotFound      = "model_not_found"
	errCodeMeshOverloaded     = "mesh_overloaded"
	errCodeInternalError      = "internal_error"
	errCodeWorkerUnreachable  = "worker_unreachable"
	errCodeWorkerStreamFailed = "worker_stream_failed"
)

func writeOpenAIError(w http.ResponseWriter, status int, errType, code, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	body := openAIErrorEnvelope{Error: openAIErrorBody{Message: msg, Type: errType, Code: code}}
	if err := json.NewEncoder(w).Encode(body); err != nil {
		slog.Error("encode OpenAI error envelope", slog.Any(obs.FieldErr, err))
	}
}

var (
	errModelNotFound  = errors.New("openai: model not found")
	errMeshOverloaded = errors.New("openai: mesh overloaded")
)

func writeOpenAIWorkerError(w http.ResponseWriter, model string, err error) {
	switch {
	case errors.Is(err, errModelNotFound):
		writeOpenAIError(w, http.StatusNotFound, errTypeInvalidRequest, errCodeModelNotFound,
			"model '"+model+"' not available on this mesh. /v1/models lists addressable peer IDs; the model field also accepts AgentName or Model engine strings as routing shortcuts (not listed because they have no stable identity in a federated mesh).")
	case errors.Is(err, errMeshOverloaded):
		writeOpenAIError(w, http.StatusServiceUnavailable, errTypeServerError, errCodeMeshOverloaded,
			"all workers matching model '"+model+"' are at capacity")
	default:
		writeOpenAIError(w, http.StatusInternalServerError, errTypeServerError, errCodeInternalError, err.Error())
	}
}

func selectWorkerForModel(want string, workers map[string]types.WorkerProfile) (types.WorkerProfile, error) {
	want = strings.TrimSpace(want)
	if want == "" {
		return types.WorkerProfile{}, errModelNotFound
	}

	var (
		peerHits  []types.WorkerProfile
		nameHits  []types.WorkerProfile
		modelHits []types.WorkerProfile
	)

	for _, p := range workers {
		switch {
		case p.PeerID == want:
			peerHits = append(peerHits, p)
		case strings.EqualFold(p.AgentName, want):
			nameHits = append(nameHits, p)
		case strings.EqualFold(p.Model, want):
			modelHits = append(modelHits, p)
		}
	}

	for _, tier := range [][]types.WorkerProfile{peerHits, nameHits, modelHits} {
		if len(tier) == 0 {
			continue
		}
		avail := make([]types.WorkerProfile, 0, len(tier))
		for _, p := range tier {
			if p.MaxTasks <= 0 || p.CurrentTasks < p.MaxTasks {
				avail = append(avail, p)
			}
		}
		if len(avail) == 0 {
			return types.WorkerProfile{}, errMeshOverloaded
		}
		best := avail[0]
		bestRatio := loadRatio(best)
		for _, p := range avail[1:] {
			r := loadRatio(p)
			if r < bestRatio || (r == bestRatio && p.CPUUsagePct < best.CPUUsagePct) {
				best = p
				bestRatio = r
			}
		}
		return best, nil
	}
	return types.WorkerProfile{}, errModelNotFound
}

func loadRatio(p types.WorkerProfile) float64 {
	if p.MaxTasks <= 0 {
		return 0
	}
	return float64(p.CurrentTasks) / float64(p.MaxTasks)
}

func (b *Boss) pickWorker(model string) (types.WorkerProfile, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return selectWorkerForModel(model, b.activeWorkers)
}

func renderChatPrompt(messages []ChatMessage) string {
	var b strings.Builder
	for _, m := range messages {
		if m.Content == "" {
			continue
		}
		role := m.Role
		if role == "" {
			role = "user"
		}
		b.WriteString(role)
		b.WriteString(": ")
		b.WriteString(m.Content)
		b.WriteString("\n\n")
	}
	b.WriteString("assistant:")
	return b.String()
}

const sentinelPrefix = "[AGENTFM:"

type sentinelFilterReader struct {
	src     *bufio.Scanner
	pending []byte
	err     error
}

func newSentinelFilter(r io.Reader) *sentinelFilterReader {
	s := bufio.NewScanner(r)
	s.Buffer(make([]byte, 64*1024), 1024*1024)
	return &sentinelFilterReader{src: s}
}

func (f *sentinelFilterReader) Read(p []byte) (int, error) {
	if len(f.pending) == 0 && f.err == nil {
		for f.src.Scan() {
			line := f.src.Bytes()
			if isSentinelLine(line) {
				continue
			}
			f.pending = make([]byte, 0, len(line)+1)
			f.pending = append(f.pending, line...)
			f.pending = append(f.pending, '\n')
			break
		}
		if len(f.pending) == 0 {
			if err := f.src.Err(); err != nil {
				f.err = err
			} else {
				f.err = io.EOF
			}
		}
	}
	if len(f.pending) == 0 {
		return 0, f.err
	}
	n := copy(p, f.pending)
	f.pending = f.pending[n:]
	return n, nil
}

func isSentinelLine(line []byte) bool {
	return bytes.HasPrefix(bytes.TrimLeft(line, " \t"), []byte(sentinelPrefix))
}

type taskStream struct {
	s       netcore.Stream
	success bool
}

func (ts *taskStream) close() {
	if ts.success {
		_ = ts.s.Close()
	} else {
		_ = ts.s.Reset()
	}
}

func (b *Boss) openTaskStream(ctx context.Context, w http.ResponseWriter, peerID peer.ID, prompt, taskID string) *taskStream {
	s, err := b.dialWorkerStream(ctx, peerID)
	if err != nil {
		writeOpenAIError(w, http.StatusBadGateway, errTypeServerError, errCodeWorkerUnreachable, "failed to reach worker: "+err.Error())
		return nil
	}

	if err := s.SetWriteDeadline(time.Now().Add(network.TaskPayloadReadTimeout)); err != nil {
		_ = s.Reset()
		writeOpenAIError(w, http.StatusBadGateway, errTypeServerError, errCodeWorkerStreamFailed, "failed to arm worker stream deadline")
		return nil
	}

	payload := types.TaskPayload{
		Version: version.AppVersion,
		Task:    "agent_task",
		Data:    prompt,
		TaskID:  taskID,
	}
	if err := json.NewEncoder(s).Encode(&payload); err != nil {
		_ = s.Reset()
		writeOpenAIError(w, http.StatusBadGateway, errTypeServerError, errCodeWorkerStreamFailed, "failed to send prompt to worker")
		return nil
	}
	if err := s.CloseWrite(); err != nil {
		_ = s.Reset()
		writeOpenAIError(w, http.StatusBadGateway, errTypeServerError, errCodeWorkerStreamFailed, "failed to half-close worker stream")
		return nil
	}
	return &taskStream{s: s}
}

func drainTaskStream(s netcore.Stream) (string, error) {
	deadman := &timeoutReader{stream: s, timeout: network.TaskExecutionTimeout}
	filtered := newSentinelFilter(deadman)
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, filtered); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func newTaskStreamScanner(s netcore.Stream) *bufio.Scanner {
	deadman := &timeoutReader{stream: s, timeout: network.TaskExecutionTimeout}
	filtered := newSentinelFilter(deadman)
	sc := bufio.NewScanner(filtered)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	return sc
}

func setSSEHeaders(w http.ResponseWriter) func() {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flusher, _ := w.(http.Flusher)
	return func() {
		if flusher != nil {
			flusher.Flush()
		}
	}
}

func writeSSEFrame(w http.ResponseWriter, payload any, flush func()) bool {
	raw, err := json.Marshal(payload)
	if err != nil {
		return false
	}
	if _, err := w.Write([]byte("data: ")); err != nil {
		return false
	}
	if _, err := w.Write(raw); err != nil {
		return false
	}
	if _, err := w.Write([]byte("\n\n")); err != nil {
		return false
	}
	flush()
	return true
}

func writeSSEDone(w http.ResponseWriter, flush func()) {
	_, _ = w.Write([]byte("data: [DONE]\n\n"))
	flush()
}

func newCompletionID(prefix string) string {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		return prefix + strconv.FormatInt(time.Now().UnixNano(), 16)
	}
	return prefix + hex.EncodeToString(b)
}
