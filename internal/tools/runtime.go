package tools

import (
	"fmt"
	"os"
	"runtime"
	"time"

	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"
)

var startTime = time.Now()

// GetRuntimeStatusTool returns 'liviva_runtime_status'
func GetRuntimeStatusTool() tool.Tool {
	t, err := functiontool.New(
		functiontool.Config{
			Name:        "liviva_runtime_status",
			Description: "Returns health metrics of the LIVIVA SERVER process (uptime, memory, goroutines). use strictly for self-diagnosis.",
		},
		func(ctx tool.Context, _ struct{}) (map[string]any, error) {
			var m runtime.MemStats
			runtime.ReadMemStats(&m)

			return map[string]any{
				"uptime_formatted": time.Since(startTime).String(),
				"uptime_seconds":   time.Since(startTime).Seconds(),
				"num_goroutine":    runtime.NumGoroutine(),
				"memory_alloc_mb":  m.Alloc / 1024 / 1024,
				"memory_sys_mb":    m.Sys / 1024 / 1024,
				"host_os":          runtime.GOOS,
				"host_arch":        runtime.GOARCH,
				"num_cpu":          runtime.NumCPU(),
			}, nil
		},
	)
	if err != nil {
		panic(err)
	}
	return t
}

// GetRuntimeConfigTool returns 'liviva_runtime_config'
func GetRuntimeConfigTool(modelName string) tool.Tool {
	t, err := functiontool.New(
		functiontool.Config{
			Name:        "liviva_runtime_config",
			Description: "Returns the active configuration of the LIVIVA SERVER (Model, APIs). Internal use only.",
		},
		func(ctx tool.Context, _ struct{}) (map[string]any, error) {
			// Mask keys for safety
			hasOpenAI := os.Getenv("OPENAI_API_KEY") != ""
			hasCopilot := os.Getenv("COPILOT_API_KEY") != ""

			return map[string]any{
				"model_name":       modelName,
				"provider_openai":  hasOpenAI,
				"provider_copilot": hasCopilot,
				"mcp_enabled":      true, // Implied if running
				"memory_backend":   "sqlite",
				"log_level":        "info",
			}, nil
		},
	)
	if err != nil {
		panic(err)
	}
	return t
}

// GetRuntimeLogsTool returns 'liviva_runtime_logs'
// note: This reads the client log for now as it captures stdout in our current setup
func GetRuntimeLogsTool(logPath string) tool.Tool {
	t, err := functiontool.New(
		functiontool.Config{
			Name:        "liviva_runtime_logs",
			Description: "Reads the last N lines of the LIVIVA SERVER log. Use to diagnose recent errors or recall recent actions.",
		},
		func(ctx tool.Context, args struct {
			Lines int `json:"lines"`
		}) (map[string]any, error) {
			if args.Lines <= 0 {
				args.Lines = 20 // Default
			}
			if args.Lines > 100 {
				args.Lines = 100 // Cap
			}

			content, err := os.ReadFile(logPath)
			if err != nil {
				return nil, fmt.Errorf("failed to read log file: %w", err)
			}

			lines := splitLines(string(content))
			start := len(lines) - args.Lines
			if start < 0 {
				start = 0
			}

			return map[string]any{
				"log_tail": lines[start:],
				"source":   logPath,
			}, nil
		},
	)
	if err != nil {
		panic(err)
	}
	return t
}

func splitLines(s string) []string {
	var lines []string
	var currentLink []rune
	for _, r := range s {
		if r == '\n' {
			lines = append(lines, string(currentLink))
			currentLink = []rune{}
		} else {
			currentLink = append(currentLink, r)
		}
	}
	if len(currentLink) > 0 {
		lines = append(lines, string(currentLink))
	}
	return lines
}
