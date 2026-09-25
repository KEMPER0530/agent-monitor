package store

import (
	"testing"

	"github.com/KEMPER0530/agent-monitor/internal/model"
)

// JSONL追記後にスナップショット集計へ反映されることを確認します。
func TestJSONLStoreAppendAndSnapshot(t *testing.T) {
	s := NewJSONLStore(t.TempDir())

	event := model.NewEvent(model.EventTask, model.StatusRunning, "Implement dashboard")
	event.CostUSD = 0.25
	event.Tokens = 1200
	event.ToolCalls = 3

	if err := s.Append(event); err != nil {
		t.Fatal(err)
	}

	snapshot, err := s.Snapshot()
	if err != nil {
		t.Fatal(err)
	}

	if snapshot.Summary.TotalEvents != 1 {
		t.Fatalf("total events = %d, want 1", snapshot.Summary.TotalEvents)
	}
	if snapshot.Summary.RunningTasks != 1 {
		t.Fatalf("running tasks = %d, want 1", snapshot.Summary.RunningTasks)
	}
	if snapshot.Summary.TotalTokens != 1200 {
		t.Fatalf("tokens = %d, want 1200", snapshot.Summary.TotalTokens)
	}
}

// 必須項目がないイベントは保存せず、壊れた監視データを混入させません。
func TestJSONLStoreRejectsInvalidEvent(t *testing.T) {
	s := NewJSONLStore(t.TempDir())

	if err := s.Append(model.Event{}); err == nil {
		t.Fatal("expected validation error")
	}
}
