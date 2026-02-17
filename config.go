package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Config represents the user configuration
type Config struct {
	DefaultTeam string `json:"default_team"`
	GithubToken string `json:"github_token,omitempty"`
}

// getConfigDir returns the appropriate config directory based on the OS
func getConfigDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	var configDir string
	switch runtime.GOOS {
	case "windows":
		// On Windows, use %APPDATA%
		appData := os.Getenv("APPDATA")
		if appData != "" {
			configDir = filepath.Join(appData, "ghMdsolGo")
		} else {
			configDir = filepath.Join(homeDir, "AppData", "Roaming", "ghMdsolGo")
		}
	case "darwin", "linux":
		// On macOS and Linux, use XDG Base Directory specification
		xdgConfigHome := os.Getenv("XDG_CONFIG_HOME")
		if xdgConfigHome != "" {
			configDir = filepath.Join(xdgConfigHome, "ghMdsolGo")
		} else {
			configDir = filepath.Join(homeDir, ".config", "ghMdsolGo")
		}
	default:
		// Fallback for other Unix-like systems
		configDir = filepath.Join(homeDir, ".config", "ghMdsolGo")
	}

	return configDir, nil
}

// getConfigPath returns the full path to the config file
func getConfigPath() (string, error) {
	configDir, err := getConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, "config.json"), nil
}

// loadConfig loads the configuration from the config file
// Returns the config if found, or a Config with empty values if not found or error
func loadConfig() *Config {
	config := &Config{}

	configPath, err := getConfigPath()
	if err != nil {
		// Unable to determine config path, return empty config
		return config
	}

	// Check if config file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// Config file doesn't exist, return empty config
		return config
	}

	// Read the config file
	data, err := os.ReadFile(configPath)
	if err != nil {
		log.Printf("Warning: Unable to read config file at %s: %v", configPath, err)
		return config
	}

	// Parse JSON
	if err := json.Unmarshal(data, config); err != nil {
		log.Printf("Warning: Unable to parse config file at %s: %v", configPath, err)
		return config
	}

	return config
}

// saveConfig saves the configuration to the config file
func saveConfig(config *Config) error {
	configPath, err := getConfigPath()
	if err != nil {
		return err
	}

	// Create config directory if it doesn't exist
	configDir := filepath.Dir(configPath)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return err
	}

	// Marshal config to JSON with indentation
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	// Write to file
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return err
	}

	return nil
}

// getDefaultTeam returns the default team name from config or the hardcoded default
func getDefaultTeam() string {
	config := loadConfig()
	if config.DefaultTeam != "" {
		return config.DefaultTeam
	}
	return TeamMedidata
}

// getGithubToken returns the GitHub token from config or empty string if not set
func getGithubToken() string {
	config := loadConfig()
	return config.GithubToken
}

// initConfig interactively creates a configuration file
func initConfig() error {
	reader := bufio.NewReader(os.Stdin)

	// Check if config file already exists
	configPath, err := getConfigPath()
	if err != nil {
		return fmt.Errorf("unable to determine config path: %w", err)
	}

	if _, err := os.Stat(configPath); err == nil {
		// Config file exists, ask user if they want to overwrite
		fmt.Printf("Configuration file already exists at: %s\n", configPath)
		fmt.Print("Do you want to overwrite it? (y/N): ")
		response, _ := reader.ReadString('\n')
		response = strings.TrimSpace(strings.ToLower(response))
		if response != "y" && response != "yes" {
			fmt.Println("Configuration initialization canceled.")
			return nil
		}
	}

	fmt.Println("Configuration Initialization")
	fmt.Println("============================")
	fmt.Println()

	config := &Config{}

	// Prompt for default team
	fmt.Printf("Enter default team name [%s]: ", TeamMedidata)
	teamName, _ := reader.ReadString('\n')
	teamName = strings.TrimSpace(teamName)
	if teamName != "" {
		config.DefaultTeam = teamName
	} else {
		config.DefaultTeam = TeamMedidata
	}

	// Prompt for GitHub token
	fmt.Println()
	fmt.Println("Enter GitHub personal access token (optional):")
	fmt.Println("  Leave empty to use GITHUB_AUTH_TOKEN environment variable or .netrc")
	fmt.Print("Token: ")
	token, _ := reader.ReadString('\n')
	token = strings.TrimSpace(token)
	if token != "" {
		config.GithubToken = token
	}

	// Save the configuration
	if err := saveConfig(config); err != nil {
		return fmt.Errorf("failed to save configuration: %w", err)
	}

	// Set secure permissions on Unix-like systems
	if runtime.GOOS != "windows" {
		if err := os.Chmod(configPath, 0600); err != nil {
			log.Printf("Warning: Unable to set secure permissions on config file: %v", err)
		}
	}

	fmt.Println()
	fmt.Printf("✓ Configuration saved to: %s\n", configPath)
	if runtime.GOOS != "windows" {
		fmt.Println("✓ File permissions set to 600 (user read/write only)")
	}
	fmt.Println()
	fmt.Println("Configuration summary:")
	fmt.Printf("  Default Team: %s\n", config.DefaultTeam)
	if config.GithubToken != "" {
		fmt.Println("  GitHub Token: ***configured***")
	} else {
		fmt.Println("  GitHub Token: (not set)")
	}

	return nil
}

// rotateToken updates the GitHub token in the existing configuration
func rotateToken() error {
	reader := bufio.NewReader(os.Stdin)

	// Check if config file exists
	configPath, err := getConfigPath()
	if err != nil {
		return fmt.Errorf("unable to determine config path: %w", err)
	}

	// Load existing configuration
	config := loadConfig()

	// Check if config file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		fmt.Println("No configuration file found.")
		fmt.Printf("Run 'ghMdsolGo --init' to create a configuration file first.\n")
		return nil
	}

	fmt.Println("Rotate GitHub Token")
	fmt.Println("===================")
	fmt.Println()

	if config.GithubToken != "" {
		fmt.Println("Current configuration has a token set.")
	} else {
		fmt.Println("Current configuration does not have a token set.")
	}

	fmt.Println()
	fmt.Println("Enter new GitHub personal access token:")
	fmt.Println("  Leave empty to remove the token from config")
	fmt.Print("New Token: ")
	token, _ := reader.ReadString('\n')
	token = strings.TrimSpace(token)

	// Update token (or remove it if empty)
	config.GithubToken = token

	// Save the updated configuration
	if err := saveConfig(config); err != nil {
		return fmt.Errorf("failed to save configuration: %w", err)
	}

	// Set secure permissions on Unix-like systems
	if runtime.GOOS != "windows" {
		if err := os.Chmod(configPath, 0600); err != nil {
			log.Printf("Warning: Unable to set secure permissions on config file: %v", err)
		}
	}

	fmt.Println()
	fmt.Printf("✓ Token updated in: %s\n", configPath)
	if runtime.GOOS != "windows" {
		fmt.Println("✓ File permissions verified (600)")
	}
	fmt.Println()
	if config.GithubToken != "" {
		fmt.Println("✓ GitHub Token: ***configured***")
	} else {
		fmt.Println("✓ GitHub Token: (removed)")
	}

	return nil
}
