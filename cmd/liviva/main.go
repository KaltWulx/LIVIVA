package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"

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

	// Create a pipe for the ADK launcher to read from
	adkReader, adkWriter, err := os.Pipe()
	if err != nil {
		log.Fatalf("Failed to create ADK input pipe: %v", err)
	}

	// We replace os.Stdin for the launcher so it reads from our mixed stream
	os.Stdin = adkReader

	// InputManager Control State
	// 0 = Voice Off, 1 = Voice On
	voiceEnabled := false
	var inputMutex sync.Mutex

	// Goroutine to handle Keyboard Input (os.Stdin)
	// We read from the ORIGINAL os.Stdin (fd 0), which we haven't closed,
	// but we need to reference it carefully since we swapped the variable 'os.Stdin'.
	// Actually, os.Stdin is just a var. The new 'os.Stdin' is 'adkReader'.
	// We can use os.NewFile(0, "stdin") to get the original stdin back or just read from it before swapping?
	// Better: Keep a reference to the original stdin BEFORE swapping.
	keyboardInput := os.NewFile(0, "stdin")

	go func() {
		scanner := bufio.NewScanner(keyboardInput)
		for scanner.Scan() {
			line := scanner.Text()

			// Intercept Slash Commands
			if strings.TrimSpace(line) == "/voice on" {
				inputMutex.Lock()
				voiceEnabled = true
				inputMutex.Unlock()
				log.Println("[System] Voice Input ENABLED (Microphone ON)")
				continue
			}
			if strings.TrimSpace(line) == "/voice off" {
				inputMutex.Lock()
				voiceEnabled = false
				inputMutex.Unlock()
				log.Println("[System] Voice Input DISABLED (Microphone OFF)")
				continue
			}

			// Forward normal text to ADK
			fmt.Fprintln(adkWriter, line)
		}
	}()

	var voiceInput io.Writer

	if useVoice {
		log.Println("Starting voice interface (default: OFF). Type '/voice on' to enable microphone.")
		// Determine python executable
		pythonPath := "python3"
		if _, err := os.Stat(".venv/bin/python"); err == nil {
			pythonPath = ".venv/bin/python"
		}

		// Launch python listener
		cmd := exec.Command(pythonPath, "scripts/listen.py")

		// Python stdin is where we write text to speak (TTS) -> This remains same
		pyStdin, err := cmd.StdinPipe()
		if err != nil {
			log.Fatalf("Failed to pipe python stdin: %v", err)
		}
		voiceInput = pyStdin

		// Python stdout is where we read spoken text (STT)
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

		// Goroutine to handle Voice Input (STT)
		go func() {
			scanner := bufio.NewScanner(pyStdout)
			for scanner.Scan() {
				text := scanner.Text()

				// Check if voice is enabled
				inputMutex.Lock()
				enabled := voiceEnabled
				inputMutex.Unlock()

				if enabled {
					// Forward voice text to ADK
					fmt.Fprintln(adkWriter, text)
				}
				// If disabled, we just drop the voice text (it's still processed by Python but ignored here)
			}
		}()
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
