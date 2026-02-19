package callbacks

import (
	"fmt"
	"io"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/model"
	"google.golang.org/adk/tool"
)

// NewDelegationLogger returns a BeforeModel callback that prints a system message to the server console.
// This is used for internal specialist "brain" activation.
func NewDelegationLogger(agentName string) func(ctx agent.CallbackContext, req *model.LLMRequest) (*model.LLMResponse, error) {
	return func(ctx agent.CallbackContext, req *model.LLMRequest) (*model.LLMResponse, error) {
		// We keep brain activation logs in the server console (stdout) to avoid TUI clutter.
		fmt.Printf("\n[LIVIVA System] 🧠 Agent Brain Activated: %s\n", agentName)
		return nil, nil // Continue processing
	}
}

// NewSpecialistCallLogger returns a BeforeTool callback that logs when an agent tool is invoked.
// It writes to the provided out writer (likely the UILogger for TUI visibility).
func NewSpecialistCallLogger(out io.Writer) func(ctx tool.Context, t tool.Tool, args map[string]any) (map[string]any, error) {
	return func(ctx tool.Context, t tool.Tool, args map[string]any) (map[string]any, error) {
		toolName := t.Name()
		// Log for the wrapped specialist agents and workflows to show delegation in the UI/Logs
		switch toolName {
		case "client_admin", "analyst", "deep_research", "verified_execution":
			msg := fmt.Sprintf("\n[LIVIVA System] 🔄 Delegating task to internal specialist: %s\n", toolName)
			if out != nil {
				_, _ = out.Write([]byte(msg))
			} else {
				// Fallback to console if no writer provided
				fmt.Print(msg)
			}
		}
		return nil, nil
	}
}
