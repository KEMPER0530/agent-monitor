package main

import (
	"testing"

	"github.com/KEMPER0530/agent-monitor/internal/model"
)

// ベースURLを渡した場合でも、AWS APIのイベント登録パスへ正規化します。
func TestEventEndpointFromBaseURL(t *testing.T) {
	got, err := eventEndpoint("https://monitor.example.com", "codex")
	if err != nil {
		t.Fatal(err)
	}
	want := "https://monitor.example.com/api/events?agent=codex"
	if got != want {
		t.Fatalf("endpoint = %s, want %s", got, want)
	}
}

// すでに/api/eventsまで指定されている場合は、パスを二重に付けません。
func TestEventEndpointFromEventsURL(t *testing.T) {
	got, err := eventEndpoint("https://monitor.example.com/api/events?debug=true", "claude")
	if err != nil {
		t.Fatal(err)
	}
	want := "https://monitor.example.com/api/events?agent=claude&debug=true"
	if got != want {
		t.Fatalf("endpoint = %s, want %s", got, want)
	}
}

// export形式も読めるようにして、AGENTS.mdの設定例と実装を揃えます。
func TestParseEnvLine(t *testing.T) {
	key, value, ok := parseEnvLine(`export AGENT_MONITOR_AGENT="codex"`)
	if !ok {
		t.Fatal("expected env line to be parsed")
	}
	if key != "AGENT_MONITOR_AGENT" {
		t.Fatalf("key = %s", key)
	}
	if value != "codex" {
		t.Fatalf("value = %s", value)
	}
}

// ツール名はカンマ区切りで受け取り、空白と重複を除去します。
func TestParseList(t *testing.T) {
	got := parseList("rg, go test, rg,  npm run build ")
	want := []string{"rg", "go test", "npm run build"}
	if len(got) != len(want) {
		t.Fatalf("items = %#v", got)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("items = %#v, want %#v", got, want)
		}
	}
}

// 使用量が未指定でも、ダッシュボードが0固定にならない推定値を付与します。
func TestApplyUsageDefaultsEstimatesMissingValues(t *testing.T) {
	event := model.NewEvent(model.EventTask, model.StatusSuccess, "AI Agent Mission Control.")
	event.Message = "API送信とDynamoDB蓄積を確認しています。"

	applyUsageDefaults(&event)

	if event.Tokens <= 0 {
		t.Fatalf("tokens = %d, want positive", event.Tokens)
	}
	if event.ToolCalls <= 0 {
		t.Fatalf("toolCalls = %d, want positive", event.ToolCalls)
	}
	if event.CostUSD <= 0 {
		t.Fatalf("costUsd = %f, want positive", event.CostUSD)
	}
}

// 明示的に渡された使用量は、推定値で上書きしません。
func TestApplyUsageDefaultsKeepsExplicitValues(t *testing.T) {
	event := model.NewEvent(model.EventTask, model.StatusSuccess, "明示値")
	event.Tokens = 123
	event.ToolCalls = 4
	event.CostUSD = 0.25

	applyUsageDefaults(&event)

	if event.Tokens != 123 {
		t.Fatalf("tokens = %d", event.Tokens)
	}
	if event.ToolCalls != 4 {
		t.Fatalf("toolCalls = %d", event.ToolCalls)
	}
	if event.CostUSD != 0.25 {
		t.Fatalf("costUsd = %f", event.CostUSD)
	}
}

// ローカル運用では環境変数で実測値に近い値を注入できます。
func TestApplyUsageDefaultsUsesEnvironmentValues(t *testing.T) {
	t.Setenv("AGENT_MONITOR_TOKENS", "987")
	t.Setenv("AGENT_MONITOR_TOOL_CALLS", "6")
	t.Setenv("AGENT_MONITOR_COST_USD", "0.1234")
	event := model.NewEvent(model.EventTask, model.StatusSuccess, "環境変数")

	applyUsageDefaults(&event)

	if event.Tokens != 987 {
		t.Fatalf("tokens = %d", event.Tokens)
	}
	if event.ToolCalls != 6 {
		t.Fatalf("toolCalls = %d", event.ToolCalls)
	}
	if event.CostUSD != 0.1234 {
		t.Fatalf("costUsd = %f", event.CostUSD)
	}
}
