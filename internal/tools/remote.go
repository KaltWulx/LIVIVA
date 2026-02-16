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

// GetRemoteSystemTool returns the 'get_system_info' tool that runs on the client
func GetRemoteSystemTool(dispatcher RemoteDispatcher) tool.Tool {
	t, err := functiontool.New(
		functiontool.Config{
			Name:        "get_system_info",
			Description: "Returns basic system information (OS, Arch, Hostname) of the CLIENT.",
		},
		func(ctx tool.Context, _ struct{}) (map[string]any, error) {
			output, err := dispatcher.SendToolRequest("system.info", nil)
			if err != nil {
				return nil, err
			}

			// Output is a JSON string, we need to decode it to return a map,
			// or just return it as a string inside a map if we want to be lazy,
			// but better to return the map directly if possible.
			// However, functiontool expects a map[string]any return.
			// ADK tool execution will serialize this back to JSON for the LLM.
			// So we can just return the raw JSON string as a field.
			// Actually, let's try to unmarshal it to be cleaner.
			// But since we don't have the struct definition easily here without duplicating,
			// returning it as a raw "info" field is safer and easier.

			return map[string]any{
				"info": output, // output is the JSON string from client
			}, nil
		},
	)
	if err != nil {
		panic(err)
	}
	return t
}
