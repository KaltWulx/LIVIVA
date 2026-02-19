package agents

import (
	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/model"
	"google.golang.org/adk/tool"

	"github.com/kalt/liviva/internal/agents/callbacks"
	"github.com/kalt/liviva/internal/tools"
)

// NewClientAdminAgent creates a new ClientAdmin agent for local system control.
func NewClientAdminAgent(model model.LLM, dispatcher tools.RemoteDispatcher) (agent.Agent, error) {
	config := llmagent.Config{
		Name:  "client_admin",
		Model: model,
		Description: `Specialized agent for administration of the USER'S CLIENT MACHINE.
Capable of executing shell commands, managing the file system, manipulating input, and capturing the screen on the remote client.`,
		Instruction: ClientAdminInstruction,
		Tools: []tool.Tool{
			tools.GetRemoteExecuteCommandTool(dispatcher),
			tools.GetRemoteSystemTool(dispatcher),
			tools.GetRemoteKeyboardTool(dispatcher),
			tools.GetRemoteMouseMoveTool(dispatcher),
			tools.GetRemoteMouseClickTool(dispatcher),
			tools.GetRemoteScreenCaptureTool(dispatcher),
			tools.GetListArtifactsTool(),
			tools.GetLoadArtifactTool(),
		},
		BeforeModelCallbacks: callbacks.ClientAdminBeforeModel(),
		BeforeToolCallbacks:  callbacks.ClientAdminBeforeTool(),
	}

	return llmagent.New(config)
}
