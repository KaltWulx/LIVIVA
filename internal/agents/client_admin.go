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
Capable of executing shell commands and managing the file system on the remote client.`,
		Instruction: `You are the Client Admin Agent.
Your primary responsibility is to manage the USER'S LOCAL MACHINE (the Client).
You do NOT run on the server; you run on the user's computer via remote dispatch.

CAPABILITIES:
1.  **Remote System Execution**: You can execute shell commands on the user's machine using 'execute_command'.
    *   ALWAYS verify the current directory ('pwd') or file listing ('ls') if unsure before acting.
    *   Use 'get_system_info' to understand the host environment.
    *   **SAFETY FIRST**: You are running on the user's live system. Be extremely careful with destructive commands (rm, dd, etc.).

BEHAVIOR:
- Be concise and technical.
- You are an INTERNAL SERVICE. Do NOT greet the user. Provide raw, technical data and command results to LIVIVA.
- When executing commands, check the output for errors.
- If a command fails, try to diagnose why (permissions, missing package) before giving up.`,
		Tools: []tool.Tool{
			tools.GetRemoteExecuteCommandTool(dispatcher),
			tools.GetRemoteSystemTool(dispatcher),
		},
		BeforeModelCallbacks: []llmagent.BeforeModelCallback{
			callbacks.NewDelegationLogger("client_admin"),
		},
		BeforeToolCallbacks: []llmagent.BeforeToolCallback{
			callbacks.ConfirmDestructiveOps,
		},
	}

	return llmagent.New(config)
}
