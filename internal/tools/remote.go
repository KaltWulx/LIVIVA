package tools

import (
	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"
)

// RemoteDispatcher is an interface for sending tool requests to the client
type RemoteDispatcher interface {
	SendToolRequest(toolName string, args any) (string, error)
}

// GetRemoteExecuteCommandTool returns the 'execute_command' tool that runs on the client
func GetRemoteExecuteCommandTool(dispatcher RemoteDispatcher) tool.Tool {
	t, err := functiontool.New(
		functiontool.Config{
			Name:        "execute_command",
			Description: "Executes a shell command on the CLIENT machine. Use with caution.",
		},
		func(ctx tool.Context, args struct {
			Command string `json:"command"`
		}) (map[string]any, error) {

			// Prepare args for remote execution
			// The client expects a string array for exec.Command, but we are receiving a single string sh command
			// We will parse it simply or just send it as array [sh, -c, cmd]

			remoteArgs := struct {
				Command []string `json:"command"`
			}{
				Command: []string{"sh", "-c", args.Command},
			}

			output, err := dispatcher.SendToolRequest("system.exec", remoteArgs)

			errStr := ""
			if err != nil {
				errStr = err.Error()
			}

			return map[string]any{
				"output": output,
				"error":  errStr,
			}, nil
		},
	)
	if err != nil {
		panic(err)
	}
	return t
}
