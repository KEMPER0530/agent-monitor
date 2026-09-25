package config

import (
	"os"
	"path/filepath"
	"strings"
)

const (
	EnabledEnv     = "AGENT_MONITOR_ENABLED"
	DefaultDataDir = ".agent-monitor-data"
	MarkerFile     = ".agent-monitor"
)

// Config はローカル実行時に必要な設定をまとめた値です。
type Config struct {
	Enabled bool
	DataDir string
	RootDir string
	Addr    string
}

// Load は環境変数とカレントプロジェクトから実行設定を作ります。
func Load(rootDir string) Config {
	if rootDir == "" {
		rootDir, _ = os.Getwd()
	}
	return Config{
		Enabled: IsEnabled(rootDir),
		DataDir: envOrDefault("AGENT_MONITOR_DATA_DIR", filepath.Join(rootDir, DefaultDataDir)),
		RootDir: rootDir,
		Addr:    envOrDefault("AGENT_MONITOR_ADDR", ":8080"),
	}
}

// IsEnabled は「環境変数を最優先、次に.marker、最後はOFF」の優先順位を実装します。
func IsEnabled(rootDir string) bool {
	value, ok := os.LookupEnv(EnabledEnv)
	if ok {
		return strings.EqualFold(strings.TrimSpace(value), "true")
	}

	if rootDir == "" {
		rootDir, _ = os.Getwd()
	}
	_, err := os.Stat(filepath.Join(rootDir, MarkerFile))
	return err == nil
}

// envOrDefault は空文字を未設定扱いにして、運用時の指定漏れを避けます。
func envOrDefault(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}
