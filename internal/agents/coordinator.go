package agents

import (
	"fmt"
	"io"

	"github.com/kalt/liviva/internal/tools"
	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/model"
	"google.golang.org/adk/tool"
)

// NewCoordinator creates the root agent for LIVIVA
func NewCoordinator(model model.LLM, voiceMode bool, voiceOutput io.Writer) (agent.Agent, error) {
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
2. If the user asks a general question or greets you, you can answer directly.`

	toolsList := []tool.Tool{tools.GetSystemTool(), tools.GetExecuteCommandTool()}

	if voiceMode {
		instruction += `

VOICE MODE PROTOCOL:
You have access to a tool named 'speak' for voice output.

DEFAULT BEHAVIOR:
- Use the 'speak' tool for conversational responses to the user.

EXCEPTIONS (Use Standard Text Only):
- Do NOT use 'speak' if the user explicitly requests silence or "text-only".
- Do NOT use 'speak' for long lists, code blocks, or structured data (just summarize verbally).
- Do NOT use 'speak' for purely technical logging or internal thoughts.

If you do not call 'speak', the users hears NOTHING (silence).`

		if voiceOutput != nil {
			toolsList = append(toolsList, tools.NewVoiceTool(voiceOutput))
		}
	}

	config := llmagent.Config{
		Name:        "coordinator",
		Model:       model,
		Description: "Root agent that coordinates tasks and delegates to specialized sub-agents.",
		Instruction: instruction,
		SubAgents:   []agent.Agent{nlpAgent},
		Tools:       toolsList,
	}

	return llmagent.New(config)
}
