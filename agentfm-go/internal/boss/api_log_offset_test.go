package boss

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"agentfm/test/testutil"
)

// M3: a huge ?offset= (uint64 that overflows int) must not panic the
// handler via a negative slice bound.
func TestHandleLog_HugeOffset_NoPanic(t *testing.T) {
	b, _ := newBossForWorkersTest(t)
	subj := testutil.NewHost(t).ID()

	req := httptest.NewRequest(http.MethodGet,
		"/v1/peers/"+subj.String()+"/log?offset=18446744073709551615&limit=10", nil)
	rec := httptest.NewRecorder()

	b.handleLog(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
}
