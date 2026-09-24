package config

import (
	"os"
	"path/filepath"
	"testing"
)

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

func TestIsEnabledDefaultsOff(t *testing.T) {
	root := t.TempDir()
	os.Unsetenv(EnabledEnv)

	if IsEnabled(root) {
		t.Fatal("expected monitoring to default off")
	}
}

