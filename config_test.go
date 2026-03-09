package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// ---------------------------------------------------------------------------
// getConfigPath
// ---------------------------------------------------------------------------

func TestGetConfigPath(t *testing.T) {
	path, err := getConfigPath()
	if err != nil {
		t.Fatalf("getConfigPath() returned error: %v", err)
	}
	if path == "" {
		t.Error("getConfigPath() returned empty path")
	}
	if filepath.Base(path) != "config.json" {
		t.Errorf("expected filename config.json, got %q", filepath.Base(path))
	}
}

// ---------------------------------------------------------------------------
// loadConfig
// ---------------------------------------------------------------------------

func TestLoadConfig_NoFile(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir()+"/nonexistent")

	cfg := loadConfig()
	if cfg == nil {
		t.Fatal("loadConfig() returned nil")
	}
	if cfg.DefaultTeam != "" {
		t.Errorf("DefaultTeam = %q, want empty", cfg.DefaultTeam)
	}
	if cfg.GithubToken != "" {
		t.Errorf("GithubToken = %q, want empty", cfg.GithubToken)
	}
}

func TestLoadConfig_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	// Write a malformed JSON file directly.
	cfgDir := filepath.Join(tmpDir, "ghMdsolGo")
	if err := os.MkdirAll(cfgDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "config.json"), []byte("{invalid"), 0644); err != nil {
		t.Fatal(err)
	}

	// loadConfig should not panic or crash; it returns an empty config on error.
	cfg := loadConfig()
	if cfg == nil {
		t.Fatal("loadConfig() returned nil for malformed JSON")
	}
}

// ---------------------------------------------------------------------------
// saveConfig / loadConfig round-trip
// ---------------------------------------------------------------------------

func TestSaveAndLoadConfig_RoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	want := &Config{
		DefaultTeam: "My Test Team",
		GithubToken: "test-token-123",
	}
	if err := saveConfig(want); err != nil {
		t.Fatalf("saveConfig() error: %v", err)
	}

	got := loadConfig()
	if got.DefaultTeam != want.DefaultTeam {
		t.Errorf("DefaultTeam = %q, want %q", got.DefaultTeam, want.DefaultTeam)
	}
	if got.GithubToken != want.GithubToken {
		t.Errorf("GithubToken = %q, want %q", got.GithubToken, want.GithubToken)
	}
}

func TestSaveConfig_CreatesNestedDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmpDir, "nested", "path"))

	cfg := &Config{DefaultTeam: "Team X"}
	if err := saveConfig(cfg); err != nil {
		t.Fatalf("saveConfig() error: %v", err)
	}

	// Verify the file exists and contains valid JSON.
	path, _ := getConfigPath()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read saved config: %v", err)
	}
	var loaded Config
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("saved config is not valid JSON: %v", err)
	}
	if loaded.DefaultTeam != "Team X" {
		t.Errorf("DefaultTeam = %q, want %q", loaded.DefaultTeam, "Team X")
	}
}

func TestSaveConfig_OmitsEmptyToken(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	cfg := &Config{DefaultTeam: "SomeTeam"}
	if err := saveConfig(cfg); err != nil {
		t.Fatalf("saveConfig() error: %v", err)
	}

	path, _ := getConfigPath()
	data, _ := os.ReadFile(path)

	// github_token is tagged `omitempty`, so it must not appear in JSON when empty.
	var raw map[string]interface{}
	_ = json.Unmarshal(data, &raw)
	if _, ok := raw["github_token"]; ok {
		t.Error("github_token should be omitted from JSON when empty")
	}
}

// ---------------------------------------------------------------------------
// getDefaultTeam
// ---------------------------------------------------------------------------

func TestGetDefaultTeam_FallsBackToHardcoded(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir()+"/nothing")

	team := getDefaultTeam()
	if team != TeamMedidata {
		t.Errorf("getDefaultTeam() = %q, want %q", team, TeamMedidata)
	}
}

func TestGetDefaultTeam_ReadsFromConfig(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	if err := saveConfig(&Config{DefaultTeam: "Custom Team"}); err != nil {
		t.Fatalf("saveConfig() error: %v", err)
	}

	team := getDefaultTeam()
	if team != "Custom Team" {
		t.Errorf("getDefaultTeam() = %q, want %q", team, "Custom Team")
	}
}

// ---------------------------------------------------------------------------
// getGithubToken
// ---------------------------------------------------------------------------

func TestGetGithubToken_Empty(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir()+"/nothing")

	token := getGithubToken()
	if token != "" {
		t.Errorf("getGithubToken() = %q, want empty", token)
	}
}

func TestGetGithubToken_ReadsFromConfig(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	if err := saveConfig(&Config{GithubToken: "my-secret-token"}); err != nil {
		t.Fatalf("saveConfig() error: %v", err)
	}

	token := getGithubToken()
	if token != "my-secret-token" {
		t.Errorf("getGithubToken() = %q, want %q", token, "my-secret-token")
	}
}
