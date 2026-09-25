package main

import "testing"

// ベースURLを渡した場合でも、AWS APIのイベント登録パスへ正規化します。
func TestEventEndpointFromBaseURL(t *testing.T) {
	got, err := eventEndpoint("https://s3-agent-monitor.kemper0530.com", "codex")
	if err != nil {
		t.Fatal(err)
	}
	want := "https://s3-agent-monitor.kemper0530.com/api/events?agent=codex"
	if got != want {
		t.Fatalf("endpoint = %s, want %s", got, want)
	}
}

// すでに/api/eventsまで指定されている場合は、パスを二重に付けません。
func TestEventEndpointFromEventsURL(t *testing.T) {
	got, err := eventEndpoint("https://s3-agent-monitor.kemper0530.com/api/events?debug=true", "claude")
	if err != nil {
		t.Fatal(err)
	}
	want := "https://s3-agent-monitor.kemper0530.com/api/events?agent=claude&debug=true"
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
