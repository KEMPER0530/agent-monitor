package store

import (
	"bufio"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/KEMPER0530/agent-monitor/internal/model"
)

const eventsFile = "events.jsonl"

// EventStore は保存先をJSONL以外へ差し替えやすくする境界です。
type EventStore interface {
	Append(model.Event) error
	List() ([]model.Event, error)
	Snapshot() (model.Snapshot, error)
}

// JSONLStore はMVP用のファイルベース永続化です。
type JSONLStore struct {
	dir string
	mu  sync.Mutex
}

// NewJSONLStore は保存ディレクトリだけを受け取り、呼び出し側を単純に保ちます。
func NewJSONLStore(dir string) *JSONLStore {
	return &JSONLStore{dir: dir}
}

// Append はJSONLへ追記し、ダッシュボード用のstate.jsonも更新します。
func (s *JSONLStore) Append(event model.Event) error {
	if err := event.Validate(); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.MkdirAll(s.dir, 0755); err != nil {
		return err
	}
	file, err := os.OpenFile(filepath.Join(s.dir, eventsFile), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	encoded, err := json.Marshal(event)
	if err != nil {
		return err
	}
	if _, err := file.Write(append(encoded, '\n')); err != nil {
		return err
	}
	return s.writeStateLocked()
}

// List は保存済みイベントを作成時刻順で返します。
func (s *JSONLStore) List() ([]model.Event, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.listLocked()
}

// Snapshot は現在のイベント列からUI向け集計を作ります。
func (s *JSONLStore) Snapshot() (model.Snapshot, error) {
	events, err := s.List()
	if err != nil {
		return model.Snapshot{}, err
	}
	return BuildSnapshot(events), nil
}

// listLocked は呼び出し元がmutexを持っている前提の読み込み処理です。
func (s *JSONLStore) listLocked() ([]model.Event, error) {
	path := filepath.Join(s.dir, eventsFile)
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return []model.Event{}, nil
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var events []model.Event
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var event model.Event
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	sort.Slice(events, func(i, j int) bool {
		return events[i].CreatedAt.Before(events[j].CreatedAt)
	})
	return events, nil
}

// writeStateLocked は外部ツールが読みやすいJSONスナップショットを併せて出力します。
func (s *JSONLStore) writeStateLocked() error {
	events, err := s.listLocked()
	if err != nil {
		return err
	}
	snapshot := BuildSnapshot(events)
	encoded, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(s.dir, "state.json"), encoded, 0644)
}

// BuildSnapshot は保存層に依存しない集計ロジックとしてテストしやすくしています。
func BuildSnapshot(events []model.Event) model.Snapshot {
	summary := model.Summary{TotalEvents: len(events)}
	latestTasks := map[string]model.Event{}
	latestQuestions := map[string]model.Event{}
	for _, event := range events {
		if event.Status == model.StatusFailed {
			summary.FailedEvents++
		}
		if event.Type == model.EventTask {
			// taskIdがあるイベントは同じ作業の最新状態だけを「実行中」判定に使います。
			taskKey := event.TaskID
			if taskKey == "" && event.Status == model.StatusRunning {
				taskKey = event.ID
			}
			if taskKey != "" {
				latestTasks[taskKey] = event
			}
		}
		if event.Type == model.EventQuestion {
			// 質問も同じtaskIdの最新状態を見て、回答済みを未対応として残しません。
			questionKey := event.TaskID
			if questionKey == "" && event.Status != model.StatusSuccess {
				questionKey = event.ID
			}
			if questionKey != "" {
				latestQuestions[questionKey] = event
			}
		}
		summary.TotalCostUSD += event.CostUSD
		summary.TotalTokens += event.Tokens
		summary.TotalToolCalls += event.ToolCalls
		if event.Message != "" {
			summary.LastEventMessage = event.Message
		} else {
			summary.LastEventMessage = event.Title
		}
	}
	for _, event := range latestTasks {
		if event.Status == model.StatusRunning {
			summary.RunningTasks++
		}
	}
	for _, event := range latestQuestions {
		if event.Status != model.StatusSuccess {
			summary.OpenQuestions++
		}
	}
	return model.Snapshot{
		GeneratedAt: time.Now().UTC(),
		Summary:     summary,
		Events:      events,
	}
}
