package agents

import (
	"fmt"
	"io"

	"github.com/kalt/liviva/internal/agents/callbacks"
	"github.com/kalt/liviva/internal/mcp"
	"github.com/kalt/liviva/internal/tools"
	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/model"
	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/agenttool"
)

// NewCoordinator creates the root agent for LIVIVA
func NewCoordinator(model model.LLM, voiceOutput io.Writer, uiLogger io.Writer, dispatcher tools.RemoteDispatcher, mcpHost *mcp.Host) (agent.Agent, error) {
	// --- Sub-agents ---
	clientAdmin, err := NewClientAdminAgent(model, dispatcher)
	if err != nil {
		return nil, fmt.Errorf("failed to create client_admin agent: %w", err)
	}

	analyst, err := NewAnalystAgent(model, mcpHost.GetToolsets())
	if err != nil {
		return nil, fmt.Errorf("failed to create analyst agent: %w", err)
	}

	// --- Workflow Agents ---
	deepResearch, err := NewDeepResearchWorkflow(model, mcpHost.GetToolsets())
	if err != nil {
		return nil, fmt.Errorf("failed to create deep_research workflow: %w", err)
	}

	verifiedExec, err := NewVerifiedExecutionWorkflow(model, dispatcher)
	if err != nil {
		return nil, fmt.Errorf("failed to create verified_execution workflow: %w", err)
	}

	// --- Instruction ---
	instruction := CoordinatorInstruction

	// --- Tools ---
	logPath := "liviva-client.log"

	toolsList := []tool.Tool{
		// Self-introspection (kept on coordinator)
		tools.GetRuntimeStatusTool(),
		tools.GetRuntimeConfigTool(model.Name()),
		tools.GetRuntimeLogsTool(logPath),

		// Quick actions (kept for low-latency single-step ops)
		tools.GetRemoteSystemTool(dispatcher),
		tools.GetRemoteExecuteCommandTool(dispatcher),
		tools.GetRemoteScreenCaptureTool(dispatcher),

		// Artifact management
		tools.GetListArtifactsTool(),
		tools.GetLoadArtifactTool(),

		// Workflow delegates (structured multi-phase pipelines)
		agenttool.New(deepResearch, nil),
		agenttool.New(verifiedExec, nil),

		// Legacy delegates (single-step specialist dispatch)
		agenttool.New(clientAdmin, nil),
		agenttool.New(analyst, nil),
	}

	if voiceOutput != nil {
		toolsList = append(toolsList, tools.NewVoiceTool(voiceOutput))
		instruction += VoiceCapabilityAddon
	}

	// --- Create Agent (using callback stacks) ---
	config := llmagent.Config{
		Name:                 "LIVIVA",
		Model:                model,
		Description:          "Root agent (LIVIVA) that coordinates tasks via workflows and specialist tools.",
		Instruction:          instruction,
		Tools:                toolsList,
		BeforeToolCallbacks:  callbacks.CoordinatorBeforeTool(uiLogger),
		BeforeModelCallbacks: callbacks.CoordinatorBeforeModel(),
		Toolsets:             mcpHost.GetToolsets(),
	}

	return llmagent.New(config)
}
