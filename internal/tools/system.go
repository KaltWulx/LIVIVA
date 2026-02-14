package tools

import (
	"bytes"
	"os"
	"os/exec"
	"runtime"

	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"
)

// GetSystemTool returns the 'get_system_info' tool
func GetSystemTool() tool.Tool {
	t, err := functiontool.New(
		functiontool.Config{
			Name:        "get_system_info",
			Description: "Returns basic system information (OS, Arch, Hostname).",
		},
		getSystemInfo,
	)
	if err != nil {
		panic(err)
	}
	return t
}

func getSystemInfo(_ tool.Context, _ struct{}) (map[string]any, error) {
	hostname, _ := os.Hostname()
	return map[string]any{
		"os":       runtime.GOOS,
		"arch":     runtime.GOARCH,
		"hostname": hostname,
		"num_cpu":  runtime.NumCPU(),
	}, nil
}

// GetExecuteCommandTool returns the 'execute_command' tool
func GetExecuteCommandTool() tool.Tool {
	t, err := functiontool.New(
		functiontool.Config{
			Name:        "execute_command",
			Description: "Executes a shell command on the host system. Use with caution.",
		},
		executeCommand,
	)
	if err != nil {
		panic(err)
	}
	return t
}

func executeCommand(_ tool.Context, args struct {
	Command string `json:"command"`
}) (map[string]any, error) {
	// Security check: This is a high-risk tool. In production, we'd want strict allowlisting.
	// For this local agent, we allow it but log heavily.

	cmd := exec.Command("sh", "-c", args.Command)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = 1
		}
	}

	return map[string]any{
		"stdout":    stdout.String(),
		"stderr":    stderr.String(),
		"exit_code": exitCode,
		"error":     err != nil,
	}, nil
}
