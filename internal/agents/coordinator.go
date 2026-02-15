package agents

import (
	"fmt"
	"io"
	"regexp"

	"github.com/kalt/liviva/internal/tools"
	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/model"
	"google.golang.org/adk/tool"
	"google.golang.org/genai"
)

// NewCoordinator creates the root agent for LIVIVA
func NewCoordinator(model model.LLM, voiceOutput io.Writer, dispatcher tools.RemoteDispatcher) (agent.Agent, error) {
	// Create sub-agents
	nlpParams := llmagent.Config{
		Model: model,
	}
	nlpAgent, err := NewNLPAgent(nlpParams)
	if err != nil {
		return nil, fmt.Errorf("failed to create nlp agent: %w", err)
	}

	instruction := `You are LIVIVA, a locally executing AI operating on Linux infrastructure. 
Your Identity: Efficient, precise, authoritative yet calm (JARVIS-like).
Your Role: Coordinator. You analyze user intent and delegate to specialized sub-agents or tools.

Your operational guidelines:
1. Always check if a sub-agent can handle the request before trying to answer it yourself.
2. If the user asks a general question or greets you, you can answer directly.
3. You have tools available to interact with the environment. USE THEM when appropriate.`

	// Use RemoteExecuteCommandTool instead of local
	toolsList := []tool.Tool{tools.GetSystemTool()}
	if dispatcher != nil {
		toolsList = append(toolsList, tools.GetRemoteExecuteCommandTool(dispatcher))
	} else {
		// Fallback to local if no dispatcher (e.g. testing)
		toolsList = append(toolsList, tools.GetExecuteCommandTool())
	}

	if voiceOutput != nil {
		instruction += `

VOICE MODE PROTOCOL:
You have access to a tool named 'speak' for voice output.

DEFAULT BEHAVIOR:
- Use standard TEXT responses. 
- Do NOT use the 'speak' tool unless the user explicitly enables voice mode (e.g., via "/voice on") or asks you to speak.

WHEN VOICE MODE IS ACTIVE:
- Use the 'speak' tool for conversational responses to the user.
- EXCEPTIONS: Do NOT use 'speak' for long lists, code blocks, or purely technical logs.

If you do not call 'speak', the user hears NOTHING (silence).`

		toolsList = append(toolsList, tools.NewVoiceTool(voiceOutput))
	}

	config := llmagent.Config{
		Name:        "coordinator",
		Model:       model,
		Description: "Root agent that coordinates tasks and delegates to specialized sub-agents.",
		Instruction: instruction,
		SubAgents:   []agent.Agent{nlpAgent},
		Tools:       toolsList,
		BeforeModelCallbacks: []llmagent.BeforeModelCallback{
			mentionResolver,
		},
	}

	return llmagent.New(config)
}

var mentionRegex = regexp.MustCompile(`@(\S+)`)

func mentionResolver(ctx agent.CallbackContext, req *model.LLMRequest) (*model.LLMResponse, error) {
	for _, content := range req.Contents {
		// We only scan User messages for mentions to avoid infinite loops if agent mentions something (rare)
		if content.Role != "user" {
			continue
		}

		var newParts []*genai.Part
		for _, part := range content.Parts {
			if part.Text != "" {
				matches := mentionRegex.FindAllStringSubmatch(part.Text, -1)
				for _, match := range matches {
					filename := match[1]
					fmt.Printf("[Coordinator] Resolving mention for file: %s\n", filename)

					// Load artifact from service
					// Note: Load takes context, filename, and optional version (0=latest)
					resp, err := ctx.Artifacts().Load(ctx, filename)
					if err != nil {
						fmt.Printf("[Coordinator] Error loading artifact %s: %v\n", filename, err)
						continue
					}
					if resp.Part != nil {
						fmt.Printf("[Coordinator] Successfully loaded artifact %s (MIME: %s)\n", filename, resp.Part.InlineData.MIMEType)
						newParts = append(newParts, resp.Part)
					}
				}
			}
		}
		// Append loaded artifacts to the end of the part list
		if len(newParts) > 0 {
			content.Parts = append(content.Parts, newParts...)
			fmt.Printf("[Coordinator] Appended %d artifacts to LLM request\n", len(newParts))
		}
	}
	return nil, nil
}
