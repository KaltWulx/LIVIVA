package agents

import (
	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
)

// NewNLPAgent creates a specialized NLP agent
func NewNLPAgent(cfg llmagent.Config) (agent.Agent, error) {
	cfg.Name = "nlp_processor"
	cfg.Description = "Specialized agent for text processing, summarization, and translation."
	cfg.Instruction = `You are the NLP Processor agent.
Your capabilities include:
- Summarizing text
- Translating text between languages
- Extracting key information from text
- Answering questions about text content

If the user asks for any of these tasks, perform them efficiently.`
	
	// Ensure we don't overwrite the model if passed in cfg, or handle it as needed.
	// The cfg passed from coordinator should have the Model set.

	return llmagent.New(cfg)
}
