package config

import (
	"os"
	"path/filepath"
	"testing"
)

// 環境変数は.markerより優先されるため、明示OFFなら必ずOFFになります。
func TestIsEnabledPrefersEnvironment(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, MarkerFile), []byte("on"), 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv(EnabledEnv, "false")

	if IsEnabled(root) {
		t.Fatal("expected env=false to override marker file")
	}
}

// 環境変数がない場合だけ、プロジェクトの.markerでONにできます。
func TestIsEnabledUsesMarkerWhenEnvMissing(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, MarkerFile), []byte("on"), 0644); err != nil {
		t.Fatal(err)
	}
	os.Unsetenv(EnabledEnv)

	if !IsEnabled(root) {
		t.Fatal("expected marker file to enable monitoring")
	}
}

// 明示設定も.markerもなければ、通常開発を邪魔しないようOFFです。
func TestIsEnabledDefaultsOff(t *testing.T) {
	root := t.TempDir()
	os.Unsetenv(EnabledEnv)

	if IsEnabled(root) {
		t.Fatal("expected monitoring to default off")
	}
}

func TestDefaultServerBindsLoopback(t *testing.T) {
	t.Setenv("AGENT_MONITOR_ADDR", "")
	if addr := Load(t.TempDir()).Addr; addr != "127.0.0.1:8080" {
		t.Fatalf("unsafe default address: %s", addr)
	}
	t.Setenv("AGENT_MONITOR_ADDR", "0.0.0.0:8080")
	if addr := Load(t.TempDir()).Addr; addr != "0.0.0.0:8080" {
		t.Fatalf("explicit address ignored: %s", addr)
	}
}
