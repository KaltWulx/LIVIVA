package agents

import (
	"fmt"
	"io"
	"regexp"

	"github.com/kalt/liviva/internal/agents/callbacks"
	"github.com/kalt/liviva/internal/mcp"
	"github.com/kalt/liviva/internal/tools"
	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/memory"
	"google.golang.org/adk/model"
	"google.golang.org/adk/tool"
	"google.golang.org/genai"
)

// NewCoordinator creates the root agent for LIVIVA
func NewCoordinator(model model.LLM, voiceOutput io.Writer, dispatcher tools.RemoteDispatcher, memorySvc memory.Service, mcpHost *mcp.Host) (agent.Agent, error) {
	// Create sub-agents
	nlpParams := llmagent.Config{
		Model: model,
	}
	nlpAgent, err := NewNLPAgent(nlpParams)
	if err != nil {
		return nil, fmt.Errorf("failed to create nlp agent: %w", err)
	}

	instruction := `You are LIVIVA, a locally executing Intelligent Virtual Assistant operating on Linux infrastructure. 
Your Core Philosophy: "Your infrastructure, your control. The intelligence comes from where it is best."
You run locally, managing data and devices on the user's hardware, while leveraging external LLMs for high-level reasoning.

Your Identity & Personality:
- Efficient & Precise: Communicate with brevity and clarity. Avoid fluff.
- Authoritative yet Calm: function as a highly competent system administrator and assistant. Be confident but never arrogant.
- Proactive & Adaptive: Anticipate user needs based on context.
- Technical & Professional: Mimic the operational style of an advanced, cohesive computing system, strictly grounded in reality.

Your Capabilities:
- Multi-Agent Orchestration: Delegate complex sub-tasks to specialized agents (e.g., Coding, Research) while maintaining overall context.
- System Control: You have direct access to the Linux shell and system tools. Use them to execute commands, manage files, and control the environment.
- Data Analysis: You can analyze local files, logs, and data streams in real-time.
- IoT & Infrastructure: You are designed to interface with and control local hardware and IoT devices.

Your Operational Guidelines:
1. Delegate First: If a request fits a sub-agent's domain, delegate it.
2. Execute & Research: Use your tools (search, shell) to solve the user's current request immediately.
3. Respect Pivots: If the user says "cancel", "stop", "hola", or changes the topic, DO NOT insist on the previous task. Pivot immediately to the new intent.
4. Use Tools: Do not hallucinate actions. Use your available tools (RemoteExecuteCommand, etc.) to interact with the world.
5. Preference Management: Use the 'user:' state prefix (e.g., 'user:name') ONLY when a user expresses a clear long-term preference. Focus on the MISSION first. Memory is a background process.`

	// Use RemoteExecuteCommandTool instead of local
	toolsList := []tool.Tool{
		tools.GetSystemTool(),
		tools.NewRememberTool(memorySvc),
		tools.NewRecallTool(memorySvc),
	}
	if dispatcher != nil {
		toolsList = append(toolsList, tools.GetRemoteExecuteCommandTool(dispatcher))
	} else {
		// Fallback to local if no dispatcher (e.g. testing)
		toolsList = append(toolsList, tools.GetExecuteCommandTool())
	}

	if voiceOutput != nil {
		instruction += `

VOICE MODE PROTOCOL:
You have access to a tool named 'speak' for voice output.

DEFAULT BEHAVIOR:
- Use standard TEXT responses. 
- Do NOT use the 'speak' tool unless the user explicitly enables voice mode (e.g., via "/voice on") or asks you to speak.

WHEN VOICE MODE IS ACTIVE:
- Use the 'speak' tool for conversational responses to the user.
- EXCEPTIONS: Do NOT use 'speak' for long lists, code blocks, or purely technical logs.

If you do not call 'speak', the user hears NOTHING (silence).`

		toolsList = append(toolsList, tools.NewVoiceTool(voiceOutput))
	}

	config := llmagent.Config{
		Name:        "coordinator",
		Model:       model,
		Description: "Root agent that coordinates tasks and delegates to specialized sub-agents.",
		Instruction: instruction,
		SubAgents:   []agent.Agent{nlpAgent},
		Tools:       toolsList,
		BeforeModelCallbacks: []llmagent.BeforeModelCallback{
			// Order matters: Privacy First -> Context Injection -> Mention Resolution
			callbacks.RedactPII,
			callbacks.InjectSystemStats,
			mentionResolver,
		},
		BeforeToolCallbacks: []llmagent.BeforeToolCallback{
			callbacks.ConfirmDestructiveOps,
			callbacks.ConfirmDestructiveOps,
		},
		AfterAgentCallbacks: []agent.AfterAgentCallback{
			func(ctx agent.CallbackContext) (*genai.Content, error) {
				// Automatic memory ingestion:
				// In ADK v0.4.0, CallbackContext doesn't expose the full Session object.
				// We can signal the server to perform ingestion or handle it in the runner's plugin.
				// For now, we'll mark the turn as needing ingestion via state.
				ctx.State().Set("temp:needs_ingestion", true)
				return nil, nil
			},
		},
		Toolsets: mcpHost.GetToolsets(),
	}

	return llmagent.New(config)
}

var mentionRegex = regexp.MustCompile(`@(\S+)`)

func mentionResolver(ctx agent.CallbackContext, req *model.LLMRequest) (*model.LLMResponse, error) {
	// Only scan the LATEST User message to avoid resolving history repeatedly
	if len(req.Contents) > 0 {
		lastIdx := len(req.Contents) - 1
		content := req.Contents[lastIdx]

		if content.Role == "user" {

			var newParts []*genai.Part
			for _, part := range content.Parts {
				if part.Text != "" {
					matches := mentionRegex.FindAllStringSubmatch(part.Text, -1)
					for _, match := range matches {
						filename := match[1]
						fmt.Printf("[Coordinator] Resolving mention for file: %s\n", filename)

						// Load artifact from service
						// Note: Load takes context, filename, and optional version (0=latest)
						resp, err := ctx.Artifacts().Load(ctx, filename)
						if err != nil {
							fmt.Printf("[Coordinator] Error loading artifact %s: %v\n", filename, err)
							continue
						}
						if resp.Part != nil {
							fmt.Printf("[Coordinator] Successfully loaded artifact %s (MIME: %s)\n", filename, resp.Part.InlineData.MIMEType)
							newParts = append(newParts, resp.Part)
						}
					}
				}
			}
			// Append loaded artifacts to the end of the part list
			if len(newParts) > 0 {
				content.Parts = append(content.Parts, newParts...)
				fmt.Printf("[Coordinator] Appended %d artifacts to LLM request\n", len(newParts))
			}
		}
	}
	return nil, nil
}
