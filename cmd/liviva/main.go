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
	useVoice := true // Default to true
	llmDebug := false
	args := os.Args[1:]

	var newArgs []string
	for _, arg := range args {
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

	// Command Registry
	type CommandHandler func(args []string)
	commands := make(map[string]CommandHandler)

	// Voice Process Management
	var (
		voiceCmd   *exec.Cmd
		voiceStdin io.WriteCloser // For TTS
		// voiceInput io.Writer      // This needs to be declared here for the agent
		// voiceStdout io.ReadCloser  // For STT (handled in goroutine)
	)

	startVoice := func() error {
		if voiceCmd != nil && voiceCmd.Process != nil {
			return nil // Already running
		}

		pythonPath := "python3"
		if _, err := os.Stat(".venv/bin/python"); err == nil {
			pythonPath = ".venv/bin/python"
		}

		cmd := exec.Command(pythonPath, "scripts/listen.py")

		// Pipe for TTS (Go -> Python)
		stdin, err := cmd.StdinPipe()
		if err != nil {
			return fmt.Errorf("stdin pipe error: %w", err)
		}
		voiceStdin = stdin
		// voiceInput = stdin // Removed

		// Pipe for STT (Python -> Go)
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			return fmt.Errorf("stdout pipe error: %w", err)
		}

		// Pipe for Logs (Python Stderr -> Go Filtering)
		stderr, err := cmd.StderrPipe()
		if err != nil {
			return fmt.Errorf("stderr pipe error: %w", err)
		}

		// Start Process
		if err := cmd.Start(); err != nil {
			return fmt.Errorf("start error: %w", err)
		}
		voiceCmd = cmd

		// Handle STT (Forward recognized text to ADK)
		go func() {
			scanner := bufio.NewScanner(stdout)
			for scanner.Scan() {
				text := scanner.Text()
				// Only forward if voice is enabled (redundant check but safe)
				// Actually, if process is running, voice is effectively "on".
				fmt.Fprintln(adkWriter, text)
			}
		}()

		// Handle Stderr Filtering (Clean Console)
		go func() {
			scanner := bufio.NewScanner(stderr)
			for scanner.Scan() {
				line := scanner.Text()
				// Filter out noisy logs
				if strings.Contains(line, "Listening...") ||
					strings.Contains(line, "Processing audio...") ||
					strings.Contains(line, "Adjusting for ambient noise") ||
					strings.Contains(line, "Speaking:") ||
					strings.Contains(line, "[TTS]") {
					continue
				}
				// Print critical errors or other logs
				log.Printf("[Voice] %s", line)
			}
		}()

		return nil
	}

	stopVoice := func() {
		if voiceCmd != nil && voiceCmd.Process != nil {
			voiceCmd.Process.Kill()
			voiceCmd.Wait()
		}
		voiceCmd = nil
		voiceStdin = nil
	}

	// Register /voice command
	commands["/voice"] = func(args []string) {
		if len(args) < 1 {
			log.Println("[System] Usage: /voice [on|off]")
			return
		}
		action := strings.ToLower(args[0])

		inputMutex.Lock()
		defer inputMutex.Unlock()

		switch action {
		case "on":
			if voiceEnabled {
				log.Println("[System] Voice is already ON.")
				return
			}
			log.Println("[System] Starting voice engine...")
			if err := startVoice(); err != nil {
				log.Printf("[System] Failed to start voice: %v", err)
				return
			}
			voiceEnabled = true
			log.Println("[System] Voice Input ENABLED (Microphone ON)")
		case "off":
			if !voiceEnabled {
				log.Println("[System] Voice is already OFF.")
				return
			}
			stopVoice()
			voiceEnabled = false
			log.Println("[System] Voice Input DISABLED (Microphone OFF)")
		default:
			log.Printf("[System] Unknown voice action: '%s'. Use 'on' or 'off'.", action)
		}
	}

	go func() {
		scanner := bufio.NewScanner(keyboardInput)
		for scanner.Scan() {
			line := scanner.Text()
			trimmed := strings.TrimSpace(line)

			// Intercept Slash Commands
			if strings.HasPrefix(trimmed, "/") {
				parts := strings.Fields(trimmed)
				if len(parts) > 0 {
					cmdName := strings.ToLower(parts[0])
					if handler, ok := commands[cmdName]; ok {
						handler(parts[1:])
						continue
					} else {
						log.Printf("[System] Unknown command: %s", cmdName)
						continue
					}
				}
			}

			// Forward normal text to ADK
			fmt.Fprintln(adkWriter, line)
		}
	}()

	// Cleanup on exit
	defer stopVoice()

	// Initial Voice State (Keep OFF by default, as requested)
	if useVoice {
		// Just log instruction, don't start yet
		log.Println("Voice module ready. Type '/voice on' to start interactions.")
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
	// Note: voiceStdin may be nil initially if voice is off.
	// The Coordinator needs to handle a dynamic writer or checks.
	// Ideally, we wrap it in a struct that writes to the current voiceStdin.

	// Dynamic Voice Writer wrapper
	voiceWriter := &DynamicWriter{
		Writer: func() io.Writer {
			if voiceEnabled && voiceStdin != nil {
				return voiceStdin
			}
			return nil
		},
	}

	coordinator, err := agents.NewCoordinator(model, useVoice, voiceWriter)
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

// DynamicWriter allows switching the underlying writer at runtime
type DynamicWriter struct {
	Writer func() io.Writer
}

func (dw *DynamicWriter) Write(p []byte) (n int, err error) {
	w := dw.Writer()
	if w == nil {
		return len(p), nil // Silently discard if no writer
	}
	return w.Write(p)
}
