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
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/KEMPER0530/agent-monitor/internal/config"
	"github.com/KEMPER0530/agent-monitor/internal/model"
	"github.com/KEMPER0530/agent-monitor/internal/store"
)

func main() {
	rootDir, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	if err := loadDotEnv(filepath.Join(rootDir, ".agent-monitor.env")); err != nil {
		log.Printf(".agent-monitor.env skipped: %v", err)
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
	tools := flag.String("tools", "", "comma-separated tool names")
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
	event.ToolDetails = parseList(envOrDefaultValue(*tools, os.Getenv("AGENT_MONITOR_TOOL_NAMES")))
	event.CostUSD = *costUSD
	applyUsageDefaults(&event)

	if err := store.NewJSONLStore(cfg.DataDir).Append(event); err != nil {
		log.Fatal(err)
	}
	if err := postRemote(event); err != nil {
		log.Printf("remote monitor post skipped: %v", err)
	}
	fmt.Printf("recorded %s event for %s: %s\n", event.Type, event.Agent, event.Title)
}

// loadDotEnv はCodex実行時に必要なローカル専用の環境変数を読み込みます。
func loadDotEnv(path string) error {
	content, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, line := range strings.Split(string(content), "\n") {
		key, value, ok := parseEnvLine(line)
		if !ok {
			continue
		}
		if os.Getenv(key) == "" {
			if err := os.Setenv(key, value); err != nil {
				return err
			}
		}
	}
	return nil
}

// parseEnvLine はKEY=VALUEとexport KEY=VALUEのどちらも扱います。
func parseEnvLine(line string) (string, string, bool) {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "#") {
		return "", "", false
	}
	line = strings.TrimPrefix(line, "export ")
	key, value, found := strings.Cut(line, "=")
	if !found {
		return "", "", false
	}
	key = strings.TrimSpace(key)
	value = strings.Trim(strings.TrimSpace(value), `"'`)
	if key == "" {
		return "", "", false
	}
	return key, value, true
}

// postRemote はAWS APIのURLが指定されている場合だけ、API Key付きでイベントを送ります。
func postRemote(event model.Event) error {
	apiURL := strings.TrimSpace(os.Getenv("AGENT_MONITOR_API_URL"))
	if apiURL == "" {
		return nil
	}
	apiKey := strings.TrimSpace(os.Getenv("AGENT_MONITOR_API_KEY"))
	if apiKey == "" {
		return fmt.Errorf("AGENT_MONITOR_API_KEY is empty")
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
	req.Header.Set("x-api-key", apiKey)
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

// parseList は環境変数やCLI引数のカンマ区切りを重複なしの配列へ整えます。
func parseList(value string) []string {
	seen := map[string]bool{}
	var items []string
	for _, part := range strings.Split(value, ",") {
		item := strings.TrimSpace(part)
		if item == "" || seen[item] {
			continue
		}
		seen[item] = true
		items = append(items, item)
	}
	return items
}

// applyUsageDefaults は明示的な使用量がない場合でも、ダッシュボードへ0以外の推定値を送ります。
func applyUsageDefaults(event *model.Event) {
	if event.Tokens <= 0 {
		event.Tokens = envInt("AGENT_MONITOR_TOKENS", estimateTokens(event.Title, event.Message))
	}
	if event.ToolCalls <= 0 {
		event.ToolCalls = envInt("AGENT_MONITOR_TOOL_CALLS", 1)
	}
	if event.CostUSD <= 0 {
		event.CostUSD = envFloat("AGENT_MONITOR_COST_USD", estimateCostUSD(event.Tokens))
	}
}

// estimateTokens は日本語を含む短文でも少なすぎないよう、文字数ベースで概算します。
func estimateTokens(title string, message string) int {
	runes := utf8.RuneCountInString(strings.TrimSpace(title + " " + message))
	if runes == 0 {
		return 1
	}
	tokens := (runes + 3) / 4
	if tokens < 8 {
		return 8
	}
	return tokens
}

// estimateCostUSD は画面上で0固定にならない最低限の概算コストを返します。
func estimateCostUSD(tokens int) float64 {
	if tokens <= 0 {
		return 0.0001
	}
	cost := float64(tokens) * envFloat("AGENT_MONITOR_COST_PER_TOKEN_USD", 0.00001)
	if cost < 0.0001 {
		return 0.0001
	}
	return cost
}

func envInt(key string, fallback int) int {
	value, err := strconv.Atoi(strings.TrimSpace(os.Getenv(key)))
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

func envFloat(key string, fallback float64) float64 {
	value, err := strconv.ParseFloat(strings.TrimSpace(os.Getenv(key)), 64)
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}
