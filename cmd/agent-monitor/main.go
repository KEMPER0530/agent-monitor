package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/KEMPER0530/agent-monitor/internal/config"
	"github.com/KEMPER0530/agent-monitor/internal/model"
	"github.com/KEMPER0530/agent-monitor/internal/store"
)

func main() {
	rootDir, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	cfg := config.Load(rootDir)

	eventType := flag.String("type", string(model.EventTask), "event type: task, tool, test, question, error")
	status := flag.String("status", string(model.StatusInfo), "status: info, running, success, failed, blocked")
	title := flag.String("title", "", "event title")
	message := flag.String("message", "", "event message")
	agent := flag.String("agent", envOrDefault("AGENT_MONITOR_AGENT", "codex"), "agent name: codex or claude")
	taskID := flag.String("task-id", "", "task id")
	tokens := flag.Int("tokens", 0, "used tokens")
	toolCalls := flag.Int("tool-calls", 0, "tool call count")
	costUSD := flag.Float64("cost-usd", 0, "estimated cost in USD")
	flag.Parse()

	// 無効時はCIや開発作業を止めないため、成功終了のno-opにします。
	if !cfg.Enabled {
		fmt.Println("agent-monitor is disabled")
		return
	}

	event := model.NewEvent(model.EventType(*eventType), model.Status(*status), *title)
	event.Message = *message
	event.Agent = *agent
	event.TaskID = *taskID
	event.Tokens = *tokens
	event.ToolCalls = *toolCalls
	event.CostUSD = *costUSD

	if err := store.NewJSONLStore(cfg.DataDir).Append(event); err != nil {
		log.Fatal(err)
	}
	if err := postRemote(event); err != nil {
		log.Printf("remote monitor post skipped: %v", err)
	}
	fmt.Printf("recorded %s event for %s: %s\n", event.Type, event.Agent, event.Title)
}

// postRemote はAWS APIのURLが指定されている場合だけ、Cognito IDトークン付きでイベントを送ります。
func postRemote(event model.Event) error {
	apiURL := strings.TrimSpace(os.Getenv("AGENT_MONITOR_API_URL"))
	if apiURL == "" {
		return nil
	}
	idToken := strings.TrimSpace(os.Getenv("AGENT_MONITOR_ID_TOKEN"))
	if idToken == "" {
		return fmt.Errorf("AGENT_MONITOR_ID_TOKEN is empty")
	}

	endpoint, err := eventEndpoint(apiURL, event.Agent)
	if err != nil {
		return err
	}
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+idToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("remote status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}

// eventEndpoint はベースURLでも/api/events直指定でも受け付け、agentクエリを必ず付与します。
func eventEndpoint(apiURL string, agent string) (string, error) {
	if !strings.Contains(apiURL, "/api/events") {
		apiURL = strings.TrimRight(apiURL, "/") + "/api/events"
	}
	parsed, err := url.Parse(apiURL)
	if err != nil {
		return "", err
	}
	query := parsed.Query()
	query.Set("agent", envOrDefaultValue(agent, "codex"))
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func envOrDefault(key string, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func envOrDefaultValue(value string, fallback string) string {
	if strings.TrimSpace(value) != "" {
		return value
	}
	return fallback
}
