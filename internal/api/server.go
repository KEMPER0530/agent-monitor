package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/KEMPER0530/agent-monitor/internal/model"
	"github.com/KEMPER0530/agent-monitor/internal/store"
)

// Server は静的ファイル配信と監視APIを同じHTTPサーバーで扱います。
type Server struct {
	store  store.EventStore
	static http.Handler
}

// NewServer は依存する保存先を外から渡し、HTTP層を薄く保ちます。
func NewServer(eventStore store.EventStore, staticDir string) http.Handler {
	return &Server{
		store:  eventStore,
		static: http.FileServer(http.Dir(staticDir)),
	}
}

// ServeHTTP はAPIパスとダッシュボード配信をルーティングします。
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.URL.Path == "/api/health":
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	case r.URL.Path == "/api/snapshot" && r.Method == http.MethodGet:
		s.handleSnapshot(w, r)
	case r.URL.Path == "/api/events" && r.Method == http.MethodPost:
		s.handleEvent(w, r)
	case strings.HasPrefix(r.URL.Path, "/api/"):
		http.NotFound(w, r)
	default:
		s.static.ServeHTTP(w, r)
	}
}

// handleSnapshot はUIがポーリングする現在状態を返します。
func (s *Server) handleSnapshot(w http.ResponseWriter, r *http.Request) {
	snapshot, err := s.store.Snapshot()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, snapshot)
}

// handleEvent は外部から届いたイベントを保存します。
func (s *Server) handleEvent(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	var event model.Event
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if event.ID == "" {
		event.ID = model.NewEvent(event.Type, event.Status, event.Title).ID
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = model.NewEvent(event.Type, event.Status, event.Title).CreatedAt
	}
	if err := s.store.Append(event); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusCreated, event)
}

// writeJSON はAPIレスポンス形式を統一します。
func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

// writeError はエラーもJSONで返してUIやCLIから扱いやすくします。
func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}
