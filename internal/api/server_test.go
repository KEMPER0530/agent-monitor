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

func TestRejectCrossOriginAndOversizedEvents(t *testing.T) {
	server := NewServer(store.NewJSONLStore(t.TempDir()), "testdata")
	for _, tc := range []struct {
		name, body, origin string
		want               int
	}{
		{"cross-origin", `{"type":"task","status":"running","title":"Injected"}`, "https://attacker.example", http.StatusForbidden},
		{"opaque-origin", `{}`, "null", http.StatusForbidden},
		{"oversized", `{"title":"` + strings.Repeat("x", 1<<20) + `"}`, "", http.StatusRequestEntityTooLarge},
		{"multiple-events", `{} {}`, "", http.StatusBadRequest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/events", strings.NewReader(tc.body))
			if tc.origin != "" {
				req.Header.Set("Origin", tc.origin)
			}
			rec := httptest.NewRecorder()
			server.ServeHTTP(rec, req)
			if rec.Code != tc.want {
				t.Fatalf("status = %d, want %d", rec.Code, tc.want)
			}
		})
	}
	req := httptest.NewRequest(http.MethodGet, "/api/snapshot", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if !strings.Contains(rec.Body.String(), `"totalEvents":0`) {
		t.Fatalf("rejected event persisted: %s", rec.Body.String())
	}
}

func TestSameOriginEventAccepted(t *testing.T) {
	server := NewServer(store.NewJSONLStore(t.TempDir()), "testdata")
	req := httptest.NewRequest(http.MethodPost, "http://localhost:8080/api/events", strings.NewReader(`{"type":"task","status":"running","title":"Build"}`))
	req.Header.Set("Origin", "http://localhost:8080")
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d", rec.Code)
	}
}
