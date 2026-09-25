package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/KEMPER0530/agent-monitor/internal/store"
)

// HTTP経由でイベント登録し、同じサーバーから集計を取得できることを確認します。
func TestPostEventAndSnapshot(t *testing.T) {
	server := NewServer(store.NewJSONLStore(t.TempDir()), "testdata")

	body := `{"type":"task","status":"running","title":"Build MVP"}`
	req := httptest.NewRequest(http.MethodPost, "/api/events", strings.NewReader(body))
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("POST /api/events status = %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/snapshot", nil)
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/snapshot status = %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"totalEvents":1`) {
		t.Fatalf("snapshot body = %s", rec.Body.String())
	}
}
