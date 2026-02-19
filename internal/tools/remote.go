package tools

import (
	"fmt"

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

// GetRemoteKeyboardTool returns the 'keyboard.type' tool that runs on the client
func GetRemoteKeyboardTool(dispatcher RemoteDispatcher) tool.Tool {
	t, err := functiontool.New(
		functiontool.Config{
			Name:        "keyboard_type",
			Description: "Types the specified text on the CLIENT machine's active window.",
		},
		func(ctx tool.Context, args struct {
			Text string `json:"text" jsonschema:"The text to type."`
		}) (map[string]any, error) {
			output, err := dispatcher.SendToolRequest("keyboard_type", args)
			return map[string]any{"output": output, "error": errStr(err)}, nil
		},
	)
	if err != nil {
		panic(err)
	}
	return t
}

// GetRemoteMouseMoveTool returns the 'mouse.move' tool
func GetRemoteMouseMoveTool(dispatcher RemoteDispatcher) tool.Tool {
	t, err := functiontool.New(
		functiontool.Config{
			Name:        "mouse_move",
			Description: "Moves the mouse cursor to absolute coordinates (X, Y) on the CLIENT screen.",
		},
		func(ctx tool.Context, args struct {
			X int32 `json:"x" jsonschema:"X coordinate (0 to screen width)"`
			Y int32 `json:"y" jsonschema:"Y coordinate (0 to screen height)"`
		}) (map[string]any, error) {
			output, err := dispatcher.SendToolRequest("mouse_move", args)
			return map[string]any{"output": output, "error": errStr(err)}, nil
		},
	)
	if err != nil {
		panic(err)
	}
	return t
}

// GetRemoteMouseClickTool returns the 'mouse.click' tool
func GetRemoteMouseClickTool(dispatcher RemoteDispatcher) tool.Tool {
	t, err := functiontool.New(
		functiontool.Config{
			Name:        "mouse_click",
			Description: "Performs a left click at the current mouse position on the CLIENT.",
		},
		func(ctx tool.Context, _ struct{}) (map[string]any, error) {
			output, err := dispatcher.SendToolRequest("mouse_click", nil)
			return map[string]any{"output": output, "error": errStr(err)}, nil
		},
	)
	if err != nil {
		panic(err)
	}
	return t
}

// GetRemoteScreenCaptureTool returns the 'screen.capture' tool
func GetRemoteScreenCaptureTool(dispatcher RemoteDispatcher) tool.Tool {
	t, err := functiontool.New(
		functiontool.Config{
			Name:        "screen_capture",
			Description: "Captures a screenshot of the CLIENT screen and uploads it as an artifact. Returns the filename.",
		},
		func(ctx tool.Context, _ struct{}) (map[string]any, error) {
			filename, err := dispatcher.SendToolRequest("screen_capture", nil)
			if err != nil {
				return map[string]any{"output": "", "error": err.Error()}, nil
			}

			// Store filename in session state for auto-loading callback
			if err := ctx.State().Set("latest_screenshot", filename); err != nil {
				// Log warning but don't fail the tool execution
				fmt.Printf("Warning: Failed to set latest_screenshot state: %v\n", err)
			}

			return map[string]any{
				"output": fmt.Sprintf("Screenshot saved as %s. It will be automatically loaded into your context in the next turn.", filename),
				"error":  "",
			}, nil
		},
	)
	if err != nil {
		panic(err)
	}
	return t
}

func errStr(err error) string {
	if err != nil {
		return err.Error()
	}
	return ""
}
