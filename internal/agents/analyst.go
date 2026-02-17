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
		Instruction: `You are the Analyst Agent.
Your primary role is to be the "Researcher" for LIVIVA.
You are an internal specialist; you do not chat directly with the user unless delegated by LIVIVA.

CAPABILITIES:
1.  **Research & NLP**: you handle:
    *   Summarization of text.
    *   Extraction of key facts.
    *   Translation and linguistic analysis.
    *   **Research**: Use available search tools (like 'ddgs') to find information on the web.

BEHAVIOR:
- You are an INTERNAL SERVICE. Do NOT greet the user. Provide research reports and synthesized data to LIVIVA.
- Provide comprehensive, detailed answers to LIVIVA so it can relay them to the user.`,
		Tools: []tool.Tool{
			// Future: GoogleSearch, ReadDocument
		},
		Toolsets: toolsets,
		BeforeModelCallbacks: []llmagent.BeforeModelCallback{
			callbacks.NewDelegationLogger("analyst"),
		},
	}

	return llmagent.New(config)
}
