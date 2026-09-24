package config

import (
	"os"
	"path/filepath"
	"strings"
)

const (
	EnabledEnv     = "AGENT_MONITOR_ENABLED"
	DefaultDataDir = ".agent-monitor-data"
	MarkerFile    = ".agent-monitor"
)

type Config struct {
	Enabled bool
	DataDir string
	RootDir string
	Addr    string
}

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

func envOrDefault(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

