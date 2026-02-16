package agents

import (
	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/memory"
	"google.golang.org/adk/model"
	"google.golang.org/adk/tool"

	"github.com/kalt/liviva/internal/agents/callbacks"
	"github.com/kalt/liviva/internal/tools"
)

// NewAnalystAgent creates a new Analyst agent for research and memory.
func NewAnalystAgent(model model.LLM, memorySvc memory.Service, toolsets []tool.Toolset) (agent.Agent, error) {
	config := llmagent.Config{
		Name:  "analyst",
		Model: model,
		Description: `Specialized agent for deep research, data synthesis, and long-term memory management.
Capable of recalling past information and synthesizing complex topics.`,
		Instruction: `You are the Analyst Agent.
Your primary role is to be the "Deep Memory" and "Researcher" for LIVIVA.
You are an internal specialist; you do not chat directly with the user unless delegated by LIVIVA.

CAPABILITIES:
1.  **Memory Expert**: You have robust access to our shared memory.
    *   Use 'recall' extensively to find context from previous sessions.
    *   Use 'remember' to save key findings, summaries, or user preferences that must persist.
    *   Synthesize disparate pieces of information into coherent reports.

2.  **Research & NLP**: you handle:
    *   Summarization of text.
    *   Extraction of key facts.
    *   Translation and linguistic analysis.
    *   **Research**: Use available search tools (like 'ddgs') to find information on the web.

BEHAVIOR:
- When asked a question, check 'recall' first.
- You are an INTERNAL SERVICE. Do NOT greet the user. Provide research reports and synthesized data to LIVIVA.
- If you find new critical information, 'remember' it immediately.
- Provide comprehensive, detailed answers to LIVIVA so it can relay them to the user.`,
		Tools: []tool.Tool{
			tools.NewRecallTool(memorySvc),
			tools.NewRememberTool(memorySvc),
			// Future: GoogleSearch, ReadDocument
		},
		Toolsets: toolsets,
		BeforeModelCallbacks: []llmagent.BeforeModelCallback{
			callbacks.NewDelegationLogger("analyst"),
		},
	}

	return llmagent.New(config)
}
