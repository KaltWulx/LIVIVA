package callbacks

import (
	"io"

	"google.golang.org/adk/agent/llmagent"
)

// Pre-built callback stacks for different agent roles.
// Inspired by Symposion's parametrized callback stacks.

// CoordinatorBeforeModel returns the standard BeforeModel stack for the Coordinator.
func CoordinatorBeforeModel() []llmagent.BeforeModelCallback {
	return []llmagent.BeforeModelCallback{
		NewDelegationLogger("Coordinator"),
		AutoLoadScreenCapture,
		PruneHistory,
	}
}

// CoordinatorBeforeTool returns the standard BeforeTool stack for the Coordinator.
func CoordinatorBeforeTool(uiLogger io.Writer) []llmagent.BeforeToolCallback {
	return []llmagent.BeforeToolCallback{
		ConfirmDestructiveOps,
		NewSpecialistCallLogger(uiLogger),
	}
}

// ResearcherBeforeModel returns a BeforeModel stack for research-phase agents.
func ResearcherBeforeModel(role string, contextKeys []string) []llmagent.BeforeModelCallback {
	return []llmagent.BeforeModelCallback{
		NewContextInjector(role, contextKeys, "research"),
		NewDelegationLogger(role),
		PruneHistory,
	}
}

// ClientAdminBeforeModel returns the standard BeforeModel stack for the ClientAdmin agent.
func ClientAdminBeforeModel() []llmagent.BeforeModelCallback {
	return []llmagent.BeforeModelCallback{
		NewDelegationLogger("client_admin"),
		AutoLoadScreenCapture,
		PruneHistory,
	}
}

// ClientAdminBeforeTool returns the standard BeforeTool stack for the ClientAdmin agent.
func ClientAdminBeforeTool() []llmagent.BeforeToolCallback {
	return []llmagent.BeforeToolCallback{
		ConfirmDestructiveOps,
	}
}
