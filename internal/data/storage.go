package data

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// LoadState loads the application state from data.json.
// If the file doesn't exist, it returns an initialized empty state.
func LoadState(dataPath string, cfg Config) (AppState, error) {
	var state AppState
	state.Config = cfg // Attach loaded config
	state.WeeklySurplus = make(map[string]int)
	state.Logs = []Day{}
	state.UndoStack = []UndoItem{}

	if _, err := os.Stat(dataPath); os.IsNotExist(err) {
		// Data file doesn't exist, return a fresh state
		return state, nil
	} else if err != nil {
		return state, fmt.Errorf("❌ error checking data file '%s': %w", dataPath, err)
	}

	bytes, err := os.ReadFile(dataPath)
	if err != nil {
		return state, fmt.Errorf("❌ could not read data file '%s': %w", dataPath, err)
	}

	// If the file is empty, return the fresh state
	if len(bytes) == 0 {
		return state, nil
	}

	if err := json.Unmarshal(bytes, &state); err != nil {
		return state, fmt.Errorf("❌ could not parse data file '%s': %w", dataPath, err)
	}

	// Ensure maps/slices are initialized if they were null in the JSON
	if state.WeeklySurplus == nil {
		state.WeeklySurplus = make(map[string]int)
	}
	if state.Logs == nil {
		state.Logs = []Day{}
	}
	if state.UndoStack == nil {
		state.UndoStack = []UndoItem{}
	}

	state.Config = cfg // Re-attach config as it's not saved in JSON
	return state, nil
}

// SaveState saves the application state to data.json.
func SaveState(dataPath string, state *AppState) error {
	// Ensure Config is not marshalled into the JSON data
	tempCfg := state.Config
	state.Config = Config{} // Zero out before marshalling

	bytes, err := json.MarshalIndent(state, "", "  ")
	state.Config = tempCfg // Restore config
	if err != nil {
		return fmt.Errorf("❌ could not marshal app state: %w", err)
	}

	if err := os.WriteFile(dataPath, bytes, 0644); err != nil {
		return fmt.Errorf("❌ could not write data file '%s': %w", dataPath, err)
	}
	return nil
}

// BackupData creates a timestamped backup of the current data.json.
func BackupData(dataPath, backupDir string) (string, error) {
	if _, err := os.Stat(dataPath); os.IsNotExist(err) {
		return "", fmt.Errorf("data file '%s' does not exist, nothing to back up", dataPath)
	}

	backupFileName := fmt.Sprintf("backup_%s.json", time.Now().Format("2006-01-06_15-04-05"))
	backupFilePath := filepath.Join(backupDir, backupFileName)

	input, err := os.ReadFile(dataPath)
	if err != nil {
		return "", fmt.Errorf("❌ could not read data file for backup: %w", err)
	}

	if err = os.WriteFile(backupFilePath, input, 0644); err != nil {
		return "", fmt.Errorf("❌ could not write backup file '%s': %w", backupFilePath, err)
	}

	return backupFilePath, nil
}

// RestoreData replaces the current data.json with the contents of a backup file.
func RestoreData(dataPath, backupFilePath string) error {
	if _, err := os.Stat(backupFilePath); os.IsNotExist(err) {
		return fmt.Errorf("backup file '%s' does not exist", backupFilePath)
	}

	input, err := os.ReadFile(backupFilePath)
	if err != nil {
		return fmt.Errorf("❌ could not read backup file '%s': %w", backupFilePath, err)
	}

	// Validate JSON structure before overwriting
	var tempState AppState
	if err := json.Unmarshal(input, &tempState); err != nil {
		return fmt.Errorf("❌ backup file '%s' is not valid JSON: %w", backupFilePath, err)
	}

	if err = os.WriteFile(dataPath, input, 0644); err != nil {
		return fmt.Errorf("❌ could not write data file '%s' from backup: %w", dataPath, err)
	}

	return nil
}
