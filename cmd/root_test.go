package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigFilePathCLIOverridesEnvironment(t *testing.T) {
	t.Setenv("AGENT_CONFIG_FILE", filepath.Join(t.TempDir(), "environment.json"))

	cliPath := filepath.Join(t.TempDir(), "command-line.json")
	if got := configFilePath(cliPath); got != cliPath {
		t.Fatalf("configFilePath() = %q, want command-line path %q", got, cliPath)
	}
}

func TestConfigFilePathUsesEnvironmentWithoutCLIValue(t *testing.T) {
	envPath := filepath.Join(t.TempDir(), "environment.json")
	t.Setenv("AGENT_CONFIG_FILE", envPath)

	if got := configFilePath(""); got != envPath {
		t.Fatalf("configFilePath() = %q, want environment path %q", got, envPath)
	}
}

func TestLoadConfigFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"token":"from-file"}`), 0600); err != nil {
		t.Fatal(err)
	}

	original := flags.Token
	t.Cleanup(func() { flags.Token = original })
	flags.Token = ""

	if err := loadConfigFile(path); err != nil {
		t.Fatalf("loadConfigFile() error = %v", err)
	}
	if flags.Token != "from-file" {
		t.Fatalf("flags.Token = %q, want %q", flags.Token, "from-file")
	}
}
