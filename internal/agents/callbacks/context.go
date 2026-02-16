package callbacks

import (
	"fmt"
	"runtime"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"
	"google.golang.org/adk/agent"
	"google.golang.org/adk/model"
	"google.golang.org/genai"
)

// InjectSystemStats is a BeforeModel callback that injects current system stats
// and time into the system prompt.
func InjectSystemStats(ctx agent.CallbackContext, req *model.LLMRequest) (*model.LLMResponse, error) {
	// 1. Gather Stats
	now := time.Now().Format(time.RFC1123)
	osInfo := fmt.Sprintf("%s / %s", runtime.GOOS, runtime.GOARCH)

	v, _ := mem.VirtualMemory()
	c, _ := cpu.Percent(0, false)

	cpuUsage := "N/A"
	if len(c) > 0 {
		cpuUsage = fmt.Sprintf("%.1f%%", c[0])
	}

	ramUsage := "N/A"
	if v != nil {
		ramUsage = fmt.Sprintf("%.1f%% (%.1f GB / %.1f GB)", v.UsedPercent, float64(v.Used)/1e9, float64(v.Total)/1e9)
	}

	// 2. Format Context Block
	contextBlock := fmt.Sprintf(`
[SYSTEM CONTEXT]
Time: %s
OS: %s
CPU: %s
RAM: %s
----------------`, now, osInfo, cpuUsage, ramUsage)

	// 3. Inject into System Instruction
	// Check if Config exists, if not create it (defensive)
	if req.Config == nil {
		return nil, nil // Should not happen in normal flow
	}

	// We append the context to the existing instruction or create a new one
	if req.Config.SystemInstruction == nil {
		req.Config.SystemInstruction = &genai.Content{
			Role:  "system",
			Parts: []*genai.Part{{Text: contextBlock}},
		}
	} else {
		// Prepend to the first part for highest visibility
		if len(req.Config.SystemInstruction.Parts) > 0 {
			original := req.Config.SystemInstruction.Parts[0].Text
			req.Config.SystemInstruction.Parts[0].Text = contextBlock + "\n" + original
		} else {
			req.Config.SystemInstruction.Parts = append(req.Config.SystemInstruction.Parts, &genai.Part{Text: contextBlock})
		}
	}

	// Log for debugging
	fmt.Printf("[Callback] Injected System Context: %s\n", now)

	return nil, nil // Continue with modified request
}

// InjectUserState is a BeforeModel callback that injects persistent user/app preferences
// from the ADK state into the system prompt.
func InjectUserState(ctx agent.CallbackContext, req *model.LLMRequest) (*model.LLMResponse, error) {
	var prefsBlock string
	found := false

	// Gather all user: preferences (exclude transient app state)
	for k, v := range ctx.ReadonlyState().All() {
		if strings.HasPrefix(k, "user:") {
			if !found {
				prefsBlock = "\n[USER PREFERENCES]\n"
				found = true
			}
			prefsBlock += fmt.Sprintf("- %s: %v\n", k, v)
		}
	}

	if !found {
		return nil, nil
	}
	prefsBlock += "----------------\n"

	// Inject into System Instruction
	if req.Config != nil && req.Config.SystemInstruction != nil {
		if len(req.Config.SystemInstruction.Parts) > 0 {
			original := req.Config.SystemInstruction.Parts[0].Text
			req.Config.SystemInstruction.Parts[0].Text = original + "\n" + prefsBlock
		}
	}

	return nil, nil
}
