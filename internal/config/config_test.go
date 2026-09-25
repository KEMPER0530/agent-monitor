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
