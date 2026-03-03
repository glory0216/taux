package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

const aliasFileName = "aliases.json"

// LoadAlias reads the alias map from configDir/aliases.json.
// Returns an empty map if the file does not exist.
func LoadAlias(configDir string) map[string]string {
	path := filepath.Join(configDir, aliasFileName)
	data, err := os.ReadFile(path)
	if err != nil {
		return make(map[string]string)
	}
	m := make(map[string]string)
	if err := json.Unmarshal(data, &m); err != nil {
		return make(map[string]string)
	}
	return m
}

// SaveAlias writes the alias map to configDir/aliases.json.
func SaveAlias(configDir string, aliasMap map[string]string) error {
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(aliasMap, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(configDir, aliasFileName), data, 0o644)
}

// GetAlias returns the alias for a session ID, or empty string if none.
func GetAlias(aliasMap map[string]string, sessionID string) string {
	return aliasMap[sessionID]
}
