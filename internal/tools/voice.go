package tools

import (
	"fmt"
	"io"
	"log"

	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"
)

// NewVoiceTool creates a tool that allows the agent to speak via the voice interface.
// It writes the text to the provided writer (which should be the stdin of the TTS process).
func NewVoiceTool(voiceInput io.Writer) tool.Tool {
	t, err := functiontool.New(
		functiontool.Config{
			Name:        "speak",
			Description: "Speak to the user via the voice interface. usage: speak(content='text to say'). The 'content' argument is REQUIRED.",
		},
		func(_ tool.Context, args struct {
			Content string `json:"content"`
		}) (map[string]any, error) {
			log.Printf("[VoiceTool] Executing speak: '%s'", args.Content)
			if voiceInput == nil {
				log.Println("[VoiceTool] Error: voiceInput is nil")
				return map[string]any{"error": "Voice interface not available"}, nil
			}

			// Write content to the voice process stdin
			// The python script expects line-delimited text
			_, err := fmt.Fprintln(voiceInput, args.Content)
			if err != nil {
				return map[string]any{"error": fmt.Sprintf("Failed to speak: %v", err)}, nil
			}

			return map[string]any{"status": "spoken"}, nil
		},
	)
	if err != nil {
		panic(err)
	}
	return t
}
