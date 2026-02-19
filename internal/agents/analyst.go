package agents

import (
	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/model"
	"google.golang.org/adk/tool"

	"github.com/kalt/liviva/internal/agents/callbacks"
)

// NewAnalystAgent creates a new Analyst agent for research and memory.
func NewAnalystAgent(model model.LLM, toolsets []tool.Toolset) (agent.Agent, error) {
	config := llmagent.Config{
		Name:  "analyst",
		Model: model,
		Description: `Specialized agent for deep research, data synthesis, and informational reports.
Capable of synthesizing complex topics from search results.`,
		Instruction: AnalystInstruction,
		Tools:       []tool.Tool{
			// Future: GoogleSearch, ReadDocument
		},
		Toolsets: toolsets,
		BeforeModelCallbacks: []llmagent.BeforeModelCallback{
			callbacks.NewDelegationLogger("analyst"),
		},
	}

	return llmagent.New(config)
}
