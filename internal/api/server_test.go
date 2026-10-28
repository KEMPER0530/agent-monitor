package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/KEMPER0530/agent-monitor/internal/store"
)

// IPの偽装を上書きし、1件の削除後も他の履歴と集計を保持します。
func TestSourceIPAndDelete(t *testing.T) {
	server := NewServer(store.NewJSONLStore(t.TempDir()), "testdata")
	for _, id := range []string{"one", "two"} {
		req := httptest.NewRequest(http.MethodPost, "/api/events", strings.NewReader(`{"id":"`+id+`","type":"task","status":"success","title":"Example","sourceIp":"forged"}`))
		req.RemoteAddr = "[2001:db8::1]:1234"
		req.Header.Set("X-Forwarded-For", "198.51.100.1")
		rec := httptest.NewRecorder()
		server.ServeHTTP(rec, req)
		if rec.Code != 201 {
			t.Fatal(rec.Body.String())
		}
	}
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/api/events?eventKey=one", nil))
	if rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/snapshot", nil))
	var body struct {
		Events []struct {
			ID       string
			SourceIP string `json:"sourceIp"`
		}
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Events) != 1 || body.Events[0].ID != "two" || body.Events[0].SourceIP != "2001:db8::1" {
		t.Fatalf("snapshot: %s", rec.Body.String())
	}
	server.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodDelete, "/api/events?eventKey=two", nil))
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/snapshot", nil))
	if !strings.Contains(rec.Body.String(), `"events":[]`) {
		t.Fatalf("empty snapshot: %s", rec.Body.String())
	}
}

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
