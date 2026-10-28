package api

import (
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
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
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if origin := r.Header.Get("Origin"); origin != "" {
		parsed, err := url.Parse(origin)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || !strings.EqualFold(parsed.Host, r.Host) {
			http.Error(w, "cross-origin request forbidden", http.StatusForbidden)
			return
		}
	}
	switch {
	case r.URL.Path == "/api/health":
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	case r.URL.Path == "/api/snapshot" && r.Method == http.MethodGet:
		s.handleSnapshot(w, r)
	case r.URL.Path == "/api/events" && r.Method == http.MethodPost:
		s.handleEvent(w, r)
	case r.URL.Path == "/api/events" && r.Method == http.MethodDelete:
		id := r.URL.Query().Get("eventKey")
		if id == "" {
			writeError(w, http.StatusBadRequest, errors.New("eventKey is required"))
			return
		}
		if err := s.store.Delete(id); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"deleted": true})
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
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var event model.Event
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&event); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			writeError(w, http.StatusRequestEntityTooLarge, err)
		} else {
			writeError(w, http.StatusBadRequest, err)
		}
		return
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		writeError(w, http.StatusBadRequest, errors.New("expected a single JSON event"))
		return
	}
	if event.ID == "" {
		event.ID = model.NewEvent(event.Type, event.Status, event.Title).ID
	}
	// ローカルでは直接接続した相手のIPを保存し、送信本文の自己申告値は上書きします。
	host, _, _ := net.SplitHostPort(r.RemoteAddr)
	event.SourceIP = ""
	if ip := net.ParseIP(host); ip != nil {
		event.SourceIP = ip.String()
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
