package callbacks

import (
	"fmt"
	"strings"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/model"
	"google.golang.org/genai"
)

// NewContextInjector returns a BeforeModel callback that injects specific
// session state keys into the system prompt, based on the current phase.
//
// Pattern from Symposion: injectContext(roleName, requiredKeys, phase)
// Each key from requiredKeys is read from the session state and appended
// to the system instruction so the agent has the context it needs.
func NewContextInjector(roleName string, requiredKeys []string, phaseHint string) func(ctx agent.CallbackContext, req *model.LLMRequest) (*model.LLMResponse, error) {
	return func(ctx agent.CallbackContext, req *model.LLMRequest) (*model.LLMResponse, error) {
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("### CONTEXT FOR %s\n\n", strings.ToUpper(roleName)))

		for _, key := range requiredKeys {
			val, _ := ctx.ReadonlyState().Get(key)
			if val == nil {
				val = "(Not Available)"
			}
			sb.WriteString(fmt.Sprintf("**%s**: %v\n\n", strings.ToUpper(key), val))
		}

		sb.WriteString(fmt.Sprintf("\n### PHASE: %s\n", strings.ToUpper(phaseHint)))

		// BEHAVIORAL RULES (Symposion Pattern)
		sb.WriteString("\n### BEHAVIORAL RULES\n")
		sb.WriteString("- **NO CHATTER**: Do not end with a question or offer additional help. Do not ask for user feedback.\n")
		switch strings.ToLower(phaseHint) {
		case "research", "gathering":
			sb.WriteString("- **EVIDENCE FIRST**: Prioritize using tools to gather external facts over relying on internal knowledge.\n")
		case "execution":
			sb.WriteString("- **STEP-BY-STEP**: Execute exactly what is planned. Verify after each destructive action.\n")
		case "verification", "synthesis":
			sb.WriteString("- **RIGOR**: Be critical. Ensure the results match the execution plan and the user intent.\n")
		}
		sb.WriteString("- **CONCISE**: Be extremely brief unless a detailed report is explicitly requested.\n")

		contextBlock := sb.String()

		// Append to system instruction (same pattern as InjectSystemStats)
		if req.Config != nil && req.Config.SystemInstruction != nil {
			if len(req.Config.SystemInstruction.Parts) > 0 {
				original := req.Config.SystemInstruction.Parts[0].Text
				req.Config.SystemInstruction.Parts[0].Text = original + "\n" + contextBlock
			} else {
				req.Config.SystemInstruction.Parts = append(
					req.Config.SystemInstruction.Parts,
					&genai.Part{Text: contextBlock},
				)
			}
		}

		fmt.Printf("[Callback] Injected context for %s (phase: %s, keys: %v)\n", roleName, phaseHint, requiredKeys)
		return nil, nil
	}
}
