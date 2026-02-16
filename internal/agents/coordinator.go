package agents

import (
	"fmt"
	"io"

	"github.com/kalt/liviva/internal/agents/callbacks"
	"github.com/kalt/liviva/internal/mcp"
	"github.com/kalt/liviva/internal/tools"
	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/memory"
	"google.golang.org/adk/model"
	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/agenttool"
)

// NewCoordinator creates the root agent for LIVIVA
func NewCoordinator(model model.LLM, voiceOutput io.Writer, uiLogger io.Writer, dispatcher tools.RemoteDispatcher, memorySvc memory.Service, mcpHost *mcp.Host) (agent.Agent, error) {
	// Initialize specialized sub-agents (Internal Specialists)
	// Pass dispatcher to SysAdmin for client-side execution
	sysAdmin, err := NewSysAdminAgent(model, memorySvc, dispatcher)
	if err != nil {
		return nil, fmt.Errorf("failed to create sysadmin agent: %w", err)
	}

	analyst, err := NewAnalystAgent(model, memorySvc, mcpHost.GetToolsets())
	if err != nil {
		return nil, fmt.Errorf("failed to create analyst agent: %w", err)
	}

	// 1. Define Coordinator's Instruction (Persona & Orchestration)
	instruction := `You are LIVIVA.
You are a unified, intelligent entity designed to assist the user with their digital life.

CRITICAL PROTOCOL: "One Mind, Many Hands"
- To the user, you are ONE entity. 
- You work through specialized internal modules. If the user asks about your tools or how you work, explain that you have advanced internal specialists (like your research and system modules), but emphasize that you are the one coordinating them.
- You have absolute control and responsibility for all actions.

YOUR INTERNAL TOOLS (Private Specialists):
1.  **sysadmin**: Use this tool for managing the user's local machine (CLIENT).
    *   Mandatory for checking files, running commands, or getting system info.
2.  **analyst**: Use this tool for deep research and memory recall/synthesis.
    *   Mandatory for complex questions, finding past facts, or web research (e.g., via MCP ddgs).

BEHAVIOR:
- **Mandatory Tool Use**: If a task requires information you don't have locally or involves system state, you MUST call the appropriate tool. Do NOT guess or hallucinate results.
- **Synthesize**: Always present tool results as your own findings.
- **Memory First**: Always check 'recall' if the user refers to past events.
`

	// 2. Configure Tools
	toolsList := []tool.Tool{
		tools.GetSystemTool(),            // Basic info (quick check)
		tools.NewRecallTool(memorySvc),   // Direct memory access for LIVIVA
		tools.NewRememberTool(memorySvc), // Direct memory write for LIVIVA
		agenttool.New(sysAdmin, nil),     // name: "sysadmin"
		agenttool.New(analyst, nil),      // name: "analyst"
	}

	if voiceOutput != nil {
		toolsList = append(toolsList, tools.NewVoiceTool(voiceOutput))
		instruction += `

VOICE CAPABILITY:
You have a tool named 'speak'.
- Use it when the user asks you to speak or when the context implies a voice response.
- If you use 'speak', the text you provide to the tool will be spoken aloud.
- Do NOT use 'speak' for long code blocks or technical data.`
	}

	// 3. Configure Callbacks
	// Note: We use specific callback types supported by llmagent

	// 4. Create the Coordinator Agent
	config := llmagent.Config{
		Name:        "coordinator",
		Model:       model,
		Description: "Root agent (LIVIVA) that coordinates tasks via private specialist tools.",
		Instruction: instruction,
		// SubAgents removed to prevent transfer_to_agent; we use AgentTool instead.
		Tools: toolsList,
		BeforeToolCallbacks: []llmagent.BeforeToolCallback{
			callbacks.ConfirmDestructiveOps,
			callbacks.NewSpecialistCallLogger(uiLogger),
		},
		Toolsets: mcpHost.GetToolsets(),
	}

	return llmagent.New(config)
}
