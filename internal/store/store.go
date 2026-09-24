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

type EventStore interface {
	Append(model.Event) error
	List() ([]model.Event, error)
	Snapshot() (model.Snapshot, error)
}

type JSONLStore struct {
	dir string
	mu  sync.Mutex
}

func NewJSONLStore(dir string) *JSONLStore {
	return &JSONLStore{dir: dir}
}

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

func (s *JSONLStore) List() ([]model.Event, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.listLocked()
}

func (s *JSONLStore) Snapshot() (model.Snapshot, error) {
	events, err := s.List()
	if err != nil {
		return model.Snapshot{}, err
	}
	return BuildSnapshot(events), nil
}

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

func BuildSnapshot(events []model.Event) model.Snapshot {
	summary := model.Summary{TotalEvents: len(events)}
	for _, event := range events {
		if event.Status == model.StatusRunning && event.Type == model.EventTask {
			summary.RunningTasks++
		}
		if event.Status == model.StatusFailed {
			summary.FailedEvents++
		}
		if event.Type == model.EventQuestion && event.Status != model.StatusSuccess {
			summary.OpenQuestions++
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
	return model.Snapshot{
		GeneratedAt: time.Now().UTC(),
		Summary:     summary,
		Events:      events,
	}
}

