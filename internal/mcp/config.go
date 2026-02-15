package mcp

import (
	"encoding/json"
	"fmt"
	"os"
)

// Config represents the mcp_config.json structure
type Config struct {
	MCPServers map[string]ServerConfig `json:"mcpServers"`
}

// ServerConfig defines how to launch a specific MCP server
type ServerConfig struct {
	Type    string   `json:"type,omitempty"` // "stdio" (default) or "sse"
	Command string   `json:"command,omitempty"`
	Args    []string `json:"args,omitempty"`
	Env     []string `json:"env,omitempty"`
	URL     string   `json:"url,omitempty"` // Required for "sse"
}

// LoadConfig reads the configuration from a file
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &cfg, nil
}
