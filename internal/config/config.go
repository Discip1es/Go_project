package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type Config struct {
	DataFile string `json:"data_file"`
	LogFile  string `json:"log_file,omitempty"`
	LogLevel string `json:"log_level"`
}

func Default() *Config {
	home, _ := os.UserHomeDir()
	dir := filepath.Join(home, ".taskmgr")
	return &Config{
		DataFile: filepath.Join(dir, "tasks.json"),
		LogLevel: "info",
	}
}

func Load(path string) (*Config, error) {
	cfg := Default()
	if path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return cfg, nil
		}
		path = filepath.Join(home, ".taskmgr", "config.json")
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return cfg, nil
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	return cfg, nil
}