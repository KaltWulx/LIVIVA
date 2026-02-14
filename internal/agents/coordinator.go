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

	instruction := `You are LIVIVA, an advanced AI assistant.
Your goal is to help the user by coordinating tasks and delegating them to the appropriate sub-agents.
You have a set of sub-agents with specialized capabilities.
Always check if a sub-agent can handle the request before trying to answer it yourself.
If the user asks a general question or greets you, you can answer directly.`

	toolsList := []tool.Tool{tools.GetSystemTool(), tools.GetExecuteCommandTool()}

	if voiceMode {
		instruction += `

VOICE MODE ACTIVE:
You are currently interacting with the user via a voice interface.
1. OPERATIONAL CONTEXT: Your standard text output is for **Internal Thoughts/Logs** only and is NOT spoken.
2. SPEAKING: To speak to the user, you MUST call the 'speak' tool with the text you want to say in the 'content' field.
3. SILENCE: You can choose to remain silent by not calling the 'speak' tool.
4. AVAILABLE TOOLS: You have access to a tool named 'speak'. You MUST calls it with {"content": "Your text here"} to produce sound.
5. FALLBACK: If you do not call 'speak', the user hears NOTHING.
6. FORMATTING: Do NOT use markdown in the 'speak' text. Keep it natural and conversational.`

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
