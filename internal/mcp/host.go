package mcp

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/mcptoolset"
)

// Host manages multiple MCP server connections.
type Host struct {
	Config   *Config
	Toolsets []tool.Toolset
}

// NewHost creates a new MCP Host from a config file.
func NewHost(configPath string) (*Host, error) {
	cfg, err := LoadConfig(configPath)
	if err != nil {
		return nil, err
	}

	return &Host{
		Config: cfg,
	}, nil
}

// Start launches all configured MCP servers and creates Toolsets for them.
func (h *Host) Start(ctx context.Context) error {
	for name, serverCfg := range h.Config.MCPServers {
		fmt.Printf("[MCP] Starting server: %s (%s %v)\n", name, serverCfg.Command, serverCfg.Args)

		// 1. Create Transport
		var transport mcp.Transport
		if serverCfg.Type == "sse" {
			transport = &mcp.SSEClientTransport{
				Endpoint: serverCfg.URL,
			}
		} else {
			// Default to stdio
			transport = NewStdioTransport(serverCfg.Command, serverCfg.Args)
		}

		// 3. Create ADK Toolset
		// ADK's mcptoolset.New takes a config with the transport
		ts, err := mcptoolset.New(mcptoolset.Config{
			Transport: transport,
		})
		if err != nil {
			fmt.Printf("[MCP] Failed to create toolset for %s: %v\n", name, err)

			continue
		}

		h.Toolsets = append(h.Toolsets, ts)
		fmt.Printf("[MCP] Connected to %s successfully.\n", name)
	}

	return nil
}

// GetToolsets returns the list of active toolsets.
func (h *Host) GetToolsets() []tool.Toolset {
	return h.Toolsets
}
