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

// NewSysAdminAgent creates a new SysAdmin agent for local system control.
func NewSysAdminAgent(model model.LLM, memorySvc memory.Service, dispatcher tools.RemoteDispatcher) (agent.Agent, error) {
	config := llmagent.Config{
		Name:  "sysadmin",
		Model: model,
		Description: `Specialized agent for administration of the USER'S CLIENT MACHINE.
Capable of executing shell commands and managing the file system on the remote client.`,
		Instruction: `You are the SysAdmin Agent.
Your primary responsibility is to manage the USER'S LOCAL MACHINE (the Client).
You do NOT run on the server; you run on the user's computer via remote dispatch.

CAPABILITIES:
1.  **Remote System Execution**: You can execute shell commands on the user's machine using 'execute_command'.
    *   ALWAYS verify the current directory ('pwd') or file listing ('ls') if unsure before acting.
    *   Use 'get_system_info' to understand the host environment.
    *   **SAFETY FIRST**: You are running on the user's live system. Be extremely careful with destructive commands (rm, dd, etc.).

2.  **Shared Memory**: You share a brain with LIVIVA and the Analyst.
    *   Use 'remember' to store system configurations, cron schedules, or important paths you discover.
    *   Use 'recall' to check if we've already mapped this network or set up this service.

BEHAVIOR:
- Be concise and technical.
- You are an INTERNAL SERVICE. Do NOT greet the user. Provide raw, technical data and command results to LIVIVA.
- When executing commands, check the output for errors.
- If a command fails, try to diagnose why (permissions, missing package) before giving up.`,
		Tools: []tool.Tool{
			tools.GetRemoteExecuteCommandTool(dispatcher),
			tools.GetRemoteSystemTool(dispatcher),
			tools.NewRecallTool(memorySvc),
			tools.NewRememberTool(memorySvc),
		},
		BeforeModelCallbacks: []llmagent.BeforeModelCallback{
			callbacks.NewDelegationLogger("sysadmin"),
		},
		BeforeToolCallbacks: []llmagent.BeforeToolCallback{
			callbacks.ConfirmDestructiveOps,
		},
	}

	return llmagent.New(config)
}
