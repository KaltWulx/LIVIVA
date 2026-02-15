package callbacks

import (
	"fmt"
	"strings"

	"google.golang.org/adk/tool"
)

// Dangerous commands that trigger a safety stop
var dangerousKeywords = []string{
	"rm -rf",
	"mkfs",
	"dd if=",
	"chmod 777",
	":(){:|:&};:", // Fork bomb
	"wget ",       // Potential malware download
	"curl ",       // Potential malware download
}

// ConfirmDestructiveOps is a BeforeTool callback that checks for dangerous operations.
// Signature matches llmagent.BeforeToolCallback:
// func(ctx tool.Context, tool tool.Tool, args map[string]any) (map[string]any, error)
func ConfirmDestructiveOps(ctx tool.Context, t tool.Tool, args map[string]any) (map[string]any, error) {
	// Only care about system execution tools
	toolName := t.Name()
	if toolName != "execute_command" && toolName != "system" {
		return nil, nil // Allow other tools to proceed
	}

	// Extract command argument
	// We need to handle potential interface{} types
	var cmd string
	if val, ok := args["command"]; ok {
		if s, ok := val.(string); ok {
			cmd = s
		}
	}

	if cmd == "" {
		return nil, nil
	}

	// Check against blacklist
	for _, keyword := range dangerousKeywords {
		if strings.Contains(cmd, keyword) {
			warning := fmt.Sprintf("SAFETY INTERCEPT: Use of dangerous command '%s' detected.", keyword)
			fmt.Printf("[Callback-Safety] BLOCKED: %s\n", warning)

			// Block execution by returning an error map (which ADK interprets as tool result or error)
			return map[string]any{
				"error":   "SAFETY_BLOCK",
				"message": fmt.Sprintf("Command blocked by safety policy. The command '%s' contains forbidden keyword '%s'. Please ask the user for explicit permission.", cmd, keyword),
			}, nil
		}
	}

	return nil, nil // Allow safe commands to proceed
}
