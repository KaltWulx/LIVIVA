package main

import (
	"context"
	"io"
	"log"
	"os"
	"os/exec"

	"github.com/joho/godotenv"
	"github.com/kalt/liviva/internal/agents"
	"github.com/kalt/liviva/internal/llm"
	"google.golang.org/adk/agent"
	"google.golang.org/adk/cmd/launcher"
	"google.golang.org/adk/cmd/launcher/full"
)

func main() {
	// Load .env file
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using system environment variables")
	}

	ctx := context.Background()

	// Handle flags
	useVoice := false
	llmDebug := false
	args := os.Args[1:]

	newArgs := []string{}
	for _, arg := range args {
		if arg == "--voice" {
			useVoice = true
			continue
		}
		if arg == "--llm-debug" {
			llmDebug = true
			continue
		}
		newArgs = append(newArgs, arg)
	}
	args = newArgs

	var voiceInput io.Writer

	if useVoice {
		log.Println("Starting voice interface...")
		// Determine python executable
		pythonPath := "python3"
		if _, err := os.Stat(".venv/bin/python"); err == nil {
			pythonPath = ".venv/bin/python"
		}

		// Launch python listener
		cmd := exec.Command(pythonPath, "scripts/listen.py")

		// Python stdin is where we write text to speak
		pyStdin, err := cmd.StdinPipe()
		if err != nil {
			log.Fatalf("Failed to pipe python stdin: %v", err)
		}
		voiceInput = pyStdin

		// Python stdout is where we read spoken text
		pyStdout, err := cmd.StdoutPipe()
		if err != nil {
			log.Fatalf("Failed to pipe python stdout: %v", err)
		}

		cmd.Stderr = os.Stderr // Pass errors through

		if err := cmd.Start(); err != nil {
			log.Fatalf("Failed to start scripts/listen.py: %v", err)
		}

		// Ensure cleanup
		defer func() {
			if cmd.Process != nil {
				cmd.Process.Kill()
			}
		}()

		// Redirect STT: Python Output -> Go Input
		// We create a pipe to replace os.Stdin so the launcher reads from the microphone text
		rIn, wIn, _ := os.Pipe()
		os.Stdin = rIn
		go func() {
			io.Copy(wIn, pyStdout)
		}()

		// NOTE: We do NOT redirect os.Stdout to Python.
		// Standard output will go to the terminal as "Inner Thoughts/Logs".
		// Speech is handled explicitly via the VoiceTool writing to voiceInput.
	}

	// Initialize OpenAI or Copilot adapter
	apiKey := os.Getenv("OPENAI_API_KEY")
	copilotKey := os.Getenv("COPILOT_API_KEY")
	modelName := os.Getenv("LIVIVA_MODEL")

	var model *llm.OpenAIModel
	if copilotKey != "" {
		if modelName == "" {
			modelName = "gpt-4"
		}
		log.Printf("Using GitHub Copilot API with model: %s", modelName)
		model = llm.NewOpenAIModel(
			copilotKey,
			modelName,
			"https://api.githubcopilot.com",
			map[string]string{
				"Editor-Version":        "vscode/1.85.1",
				"Editor-Plugin-Version": "copilot/1.143.0",
			},
		)
	} else if apiKey != "" {
		if modelName == "" {
			modelName = "gpt-4o"
		}
		log.Printf("Using OpenAI API with model: %s", modelName)
		model = llm.NewOpenAIModel(apiKey, modelName, "", nil)
	} else {
		log.Fatal("OPENAI_API_KEY or COPILOT_API_KEY is required")
	}

	if model != nil {
		model.Debug = llmDebug
	}

	// Create Coordinator agent
	coordinator, err := agents.NewCoordinator(model, useVoice, voiceInput)
	if err != nil {
		log.Fatalf("Failed to create coordinator agent: %v", err)
	}

	// Prepare launcher config
	config := &launcher.Config{
		AgentLoader: agent.NewSingleLoader(coordinator),
	}

	// Run launcher (CLI + Web UI)
	l := full.NewLauncher()
	if err := l.Execute(ctx, config, args); err != nil {
		log.Fatalf("Run failed: %v\n\n%s", err, l.CommandLineSyntax())
	}
}
