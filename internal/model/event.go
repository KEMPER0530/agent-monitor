package model

import (
	"strings"
	"time"
)

// EventType はエージェントから届く監視イベントの種類を表します。
type EventType string

const (
	EventTask     EventType = "task"
	EventTool     EventType = "tool"
	EventTest     EventType = "test"
	EventQuestion EventType = "question"
	EventError    EventType = "error"
)

// Status はイベントの進行状態です。UIの状態色にもこの値を使います。
type Status string

const (
	StatusInfo    Status = "info"
	StatusRunning Status = "running"
	StatusSuccess Status = "success"
	StatusFailed  Status = "failed"
	StatusBlocked Status = "blocked"
)

// Event はCLI、ローカルAPI、AWS Lambdaで共通利用する監視データの単位です。
type Event struct {
	ID          string            `json:"id"`
	Type        EventType         `json:"type"`
	Status      Status            `json:"status"`
	Title       string            `json:"title"`
	Message     string            `json:"message,omitempty"`
	Agent       string            `json:"agent,omitempty"`
	TaskID      string            `json:"taskId,omitempty"`
	CostUSD     float64           `json:"costUsd,omitempty"`
	Tokens      int               `json:"tokens,omitempty"`
	ToolCalls   int               `json:"toolCalls,omitempty"`
	ToolDetails []string          `json:"toolDetails,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	CreatedAt   time.Time         `json:"createdAt"`
}

// Snapshot はダッシュボードが一度に描画する集計済み状態です。
type Snapshot struct {
	GeneratedAt time.Time `json:"generatedAt"`
	Summary     Summary   `json:"summary"`
	Events      []Event   `json:"events"`
}

// Summary はイベント列から算出するKPIです。
type Summary struct {
	TotalEvents      int     `json:"totalEvents"`
	RunningTasks     int     `json:"runningTasks"`
	FailedEvents     int     `json:"failedEvents"`
	OpenQuestions    int     `json:"openQuestions"`
	TotalCostUSD     float64 `json:"totalCostUsd"`
	TotalTokens      int     `json:"totalTokens"`
	TotalToolCalls   int     `json:"totalToolCalls"`
	LastEventMessage string  `json:"lastEventMessage,omitempty"`
}

// NewEvent は作成時刻とIDをUTCで揃えて、保存形式を安定させます。
func NewEvent(eventType EventType, status Status, title string) Event {
	now := time.Now().UTC()
	return Event{
		ID:        now.Format("20060102T150405.000000000Z"),
		Type:      eventType,
		Status:    status,
		Title:     strings.TrimSpace(title),
		CreatedAt: now,
	}
}

// Validate は保存前の最低限の必須項目を確認します。
func (e Event) Validate() error {
	if e.Type == "" {
		return ErrInvalidEvent("type is required")
	}
	if e.Status == "" {
		return ErrInvalidEvent("status is required")
	}
	if strings.TrimSpace(e.Title) == "" {
		return ErrInvalidEvent("title is required")
	}
	return nil
}

// ErrInvalidEvent は入力不備を呼び出し側へそのまま返す軽量なエラー型です。
type ErrInvalidEvent string

func (e ErrInvalidEvent) Error() string {
	return string(e)
}
