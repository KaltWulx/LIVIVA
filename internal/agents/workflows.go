package agents

import (
	"fmt"

	"github.com/kalt/liviva/internal/agents/callbacks"
	"github.com/kalt/liviva/internal/tools"
	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/agent/workflowagents/loopagent"
	"google.golang.org/adk/agent/workflowagents/parallelagent"
	"google.golang.org/adk/agent/workflowagents/sequentialagent"
	"google.golang.org/adk/model"
	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/exitlooptool"
)

// NewDeepResearchWorkflow creates a multi-phase research pipeline.
//
// Pipeline: intent_analyzer → data_gathering (parallel) → synthesis
//
// Each phase writes its output to session state via OutputKey, and the next
// phase reads it via NewContextInjector.
func NewDeepResearchWorkflow(llm model.LLM, toolsets []tool.Toolset) (agent.Agent, error) {
	// Phase 1: Analyze intent and create plan
	analyzer, err := llmagent.New(llmagent.Config{
		Name:        "intent_analyzer",
		Model:       llm,
		Description: "Analyzes user intent and creates a structured research plan.",
		Instruction: IntentAnalyzerInstruction,
		OutputKey:   "research_plan",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create intent_analyzer: %w", err)
	}

	// Phase 2: Gather data in parallel
	webResearcher, err := llmagent.New(llmagent.Config{
		Name:        "web_researcher",
		Model:       llm,
		Description: "Searches the web for information based on the research plan.",
		Instruction: WebResearcherInstruction,
		Toolsets:    toolsets,
		OutputKey:   "web_findings",
		BeforeModelCallbacks: []llmagent.BeforeModelCallback{
			callbacks.NewContextInjector("Web Researcher", []string{"research_plan"}, "gathering"),
			callbacks.NewDelegationLogger("web_researcher"),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create web_researcher: %w", err)
	}

	gathering, err := parallelagent.New(parallelagent.Config{
		AgentConfig: agent.Config{
			Name:        "data_gathering",
			Description: "Gathers data in parallel from multiple sources.",
			SubAgents:   []agent.Agent{webResearcher},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create data_gathering: %w", err)
	}

	// Phase 3: Synthesize
	synthesizer, err := llmagent.New(llmagent.Config{
		Name:        "synthesis",
		Model:       llm,
		Description: "Synthesizes research findings into a comprehensive report.",
		Instruction: SynthesizerInstruction,
		OutputKey:   "final_report",
		BeforeModelCallbacks: []llmagent.BeforeModelCallback{
			callbacks.NewContextInjector("Synthesizer", []string{"research_plan", "web_findings"}, "synthesis"),
			callbacks.NewDelegationLogger("synthesis"),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create synthesis: %w", err)
	}

	return sequentialagent.New(sequentialagent.Config{
		AgentConfig: agent.Config{
			Name:        "deep_research",
			Description: "Multi-phase research pipeline: analyze intent → gather data → synthesize report.",
			SubAgents:   []agent.Agent{analyzer, gathering, synthesizer},
		},
	})
}

// NewVerifiedExecutionWorkflow creates a plan-execute-verify loop.
//
// Pipeline: planner → LoopAgent(executor → verifier with exit_loop)
//
// The loop runs up to 3 iterations. The verifier calls exit_loop when done.
func NewVerifiedExecutionWorkflow(llm model.LLM, dispatcher tools.RemoteDispatcher) (agent.Agent, error) {
	// Phase 1: Plan
	planner, err := llmagent.New(llmagent.Config{
		Name:        "planner",
		Model:       llm,
		Description: "Creates an execution plan for the requested task.",
		Instruction: ExecutionPlannerInstruction,
		OutputKey:   "execution_plan",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create planner: %w", err)
	}

	// Phase 2: Execute (inside loop)
	executor, err := llmagent.New(llmagent.Config{
		Name:        "executor",
		Model:       llm,
		Description: "Executes steps from the plan using system tools.",
		Instruction: ExecutorInstruction,
		Tools: []tool.Tool{
			tools.GetRemoteExecuteCommandTool(dispatcher),
			tools.GetRemoteKeyboardTool(dispatcher),
			tools.GetRemoteMouseMoveTool(dispatcher),
			tools.GetRemoteMouseClickTool(dispatcher),
		},
		BeforeModelCallbacks: []llmagent.BeforeModelCallback{
			callbacks.NewContextInjector("Executor", []string{"execution_plan"}, "execution"),
			callbacks.NewDelegationLogger("executor"),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create executor: %w", err)
	}

	// Phase 3: Verify (inside loop, has exit_loop tool)
	exitTool, err := exitlooptool.New()
	if err != nil {
		return nil, fmt.Errorf("failed to create exit_loop tool: %w", err)
	}

	verifier, err := llmagent.New(llmagent.Config{
		Name:        "verifier",
		Model:       llm,
		Description: "Verifies execution results and decides whether to continue or stop.",
		Instruction: VerifierInstruction,
		Tools: []tool.Tool{
			tools.GetRemoteScreenCaptureTool(dispatcher),
			exitTool,
		},
		BeforeModelCallbacks: []llmagent.BeforeModelCallback{
			callbacks.NewContextInjector("Verifier", []string{"execution_plan"}, "verification"),
			callbacks.NewDelegationLogger("verifier"),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create verifier: %w", err)
	}

	// Execution loop (max 3 iterations)
	executionLoop, err := loopagent.New(loopagent.Config{
		AgentConfig: agent.Config{
			Name:        "execution_loop",
			Description: "Iteratively executes and verifies task steps.",
			SubAgents:   []agent.Agent{executor, verifier},
		},
		MaxIterations: 3,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create execution_loop: %w", err)
	}

	return sequentialagent.New(sequentialagent.Config{
		AgentConfig: agent.Config{
			Name:        "verified_execution",
			Description: "Plan-execute-verify pipeline with iterative loop for robust task execution.",
			SubAgents:   []agent.Agent{planner, executionLoop},
		},
	})
}
