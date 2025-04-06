package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"bufio"
	"grain/internal/cli"
	"grain/internal/data"
)

const (
	defaultWeeklyGoal = 90
	defaultBreakStart = 12
	configFileName    = "config.json"
	dataFileName      = "data.json"
	backupDirName     = "backups"
)

// EnsureBaseDir creates the ~/.grain directory and subdirectories if they don't exist.
func EnsureBaseDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("❌ could not get user home directory: %w", err)
	}
	baseDir := filepath.Join(homeDir, ".grain")
	backupDir := filepath.Join(baseDir, backupDirName)

	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return "", fmt.Errorf("❌ could not create base directory '%s': %w", baseDir, err)
	}
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return "", fmt.Errorf("❌ could not create backup directory '%s': %w", backupDir, err)
	}
	return baseDir, nil
}

// GetPaths returns the absolute paths for config, data, and backup files/dirs.
func GetPaths() (baseDir, configPath, dataPath, backupDir string, err error) {
	baseDir, err = EnsureBaseDir()
	if err != nil {
		return
	}
	configPath = filepath.Join(baseDir, configFileName)
	dataPath = filepath.Join(baseDir, dataFileName)
	backupDir = filepath.Join(baseDir, backupDirName)
	return
}

// LoadConfig loads the configuration from config.json or prompts for initial setup.
func LoadConfig(configPath string) (data.Config, error) {
	var cfg data.Config

	// Check if config file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		fmt.Println(cli.FormatHeader("👋 Welcome to Grain CLI!"))
		reader := bufio.NewReader(os.Stdin)

		// Get weekly goal
		fmt.Printf("Enter your study goal per week (default: %d): ", defaultWeeklyGoal)
		goalInput, _ := reader.ReadString('\n')
		goalInput = goalInput[:len(goalInput)-1] // Remove newline
		if goalInput == "" {
			cfg.WeeklyGoal = defaultWeeklyGoal
		} else {
			goal, err := strconv.Atoi(goalInput)
			if err != nil || goal <= 0 {
				fmt.Println("Invalid input, using default.")
				cfg.WeeklyGoal = defaultWeeklyGoal
			} else {
				cfg.WeeklyGoal = goal
			}
		}

		// Get initial break credits
		fmt.Printf("Set initial break credits (default: %d): ", defaultBreakStart)
		breakInput, _ := reader.ReadString('\n')
		breakInput = breakInput[:len(breakInput)-1] // Remove newline
		if breakInput == "" {
			cfg.BreakStart = defaultBreakStart
		} else {
			start, err := strconv.Atoi(breakInput)
			if err != nil || start < 0 {
				fmt.Println("Invalid input, using default.")
				cfg.BreakStart = defaultBreakStart
			} else {
				cfg.BreakStart = start
			}
		}

		if err := SaveConfig(configPath, cfg); err != nil {
			return cfg, fmt.Errorf("❌ failed to save initial config: %w", err)
		}
		fmt.Printf("✨ Configuration saved to %s\n", configPath)
		return cfg, nil
	}

	// Config file exists, load it
	bytes, err := os.ReadFile(configPath)
	if err != nil {
		return cfg, fmt.Errorf("❌ could not read config file '%s': %w", configPath, err)
	}

	if err := json.Unmarshal(bytes, &cfg); err != nil {
		return cfg, fmt.Errorf("❌ could not parse config file '%s': %w", configPath, err)
	}

	// Ensure defaults if values are missing or invalid
	if cfg.WeeklyGoal <= 0 {
		cfg.WeeklyGoal = defaultWeeklyGoal
	}
	if cfg.BreakStart < 0 {
		cfg.BreakStart = defaultBreakStart
	}

	return cfg, nil
}

// SaveConfig saves the configuration to config.json.
func SaveConfig(configPath string, cfg data.Config) error {
	bytes, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("❌ could not marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, bytes, 0644); err != nil {
		return fmt.Errorf("❌ could not write config file '%s': %w", configPath, err)
	}
	return nil
}
