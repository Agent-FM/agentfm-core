package boss

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"agentfm/test/testutil"
)

func TestHandleLog_EmptyEntriesIsArrayNotNull(t *testing.T) {
	b, _ := newBossForWorkersTest(t)

	subject := testutil.NewHost(t).ID()
	req := httptest.NewRequest(http.MethodGet, "/v1/peers/"+subject.String()+"/log?limit=100", nil)
	rec := httptest.NewRecorder()
	b.handleLog(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), `"entries":null`) {
		t.Fatalf("entries serialized as null (breaks clients that .filter it): %s", rec.Body.String())
	}
	var resp struct {
		Entries []map[string]any `json:"entries"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Entries == nil {
		t.Fatalf("entries is nil; want an empty array")
	}
	if len(resp.Entries) != 0 {
		t.Fatalf("entries len = %d; want 0", len(resp.Entries))
	}
}
