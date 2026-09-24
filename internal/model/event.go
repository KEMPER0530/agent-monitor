package model

import (
	"strings"
	"time"
)

type EventType string

const (
	EventTask     EventType = "task"
	EventTool     EventType = "tool"
	EventTest     EventType = "test"
	EventQuestion EventType = "question"
	EventError    EventType = "error"
)

type Status string

const (
	StatusInfo    Status = "info"
	StatusRunning Status = "running"
	StatusSuccess Status = "success"
	StatusFailed  Status = "failed"
	StatusBlocked Status = "blocked"
)

type Event struct {
	ID        string            `json:"id"`
	Type      EventType         `json:"type"`
	Status    Status            `json:"status"`
	Title     string            `json:"title"`
	Message   string            `json:"message,omitempty"`
	Agent     string            `json:"agent,omitempty"`
	TaskID    string            `json:"taskId,omitempty"`
	CostUSD   float64           `json:"costUsd,omitempty"`
	Tokens    int               `json:"tokens,omitempty"`
	ToolCalls int               `json:"toolCalls,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	CreatedAt time.Time         `json:"createdAt"`
}

type Snapshot struct {
	GeneratedAt time.Time `json:"generatedAt"`
	Summary     Summary   `json:"summary"`
	Events      []Event   `json:"events"`
}

type Summary struct {
	TotalEvents     int     `json:"totalEvents"`
	RunningTasks    int     `json:"runningTasks"`
	FailedEvents    int     `json:"failedEvents"`
	OpenQuestions   int     `json:"openQuestions"`
	TotalCostUSD    float64 `json:"totalCostUsd"`
	TotalTokens     int     `json:"totalTokens"`
	TotalToolCalls  int     `json:"totalToolCalls"`
	LastEventMessage string  `json:"lastEventMessage,omitempty"`
}

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

type ErrInvalidEvent string

func (e ErrInvalidEvent) Error() string {
	return string(e)
}

