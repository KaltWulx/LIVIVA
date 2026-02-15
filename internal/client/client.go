package client

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/kalt/liviva/pkg/api"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	sttCmd     *exec.Cmd
	sttMu      sync.Mutex
	sttRunning bool
	sttStdin   io.WriteCloser
	p          *tea.Program
	sendMu     sync.Mutex // Mutex for gRPC stream
)

// Run starts the client TUI
func Run(addr string) {
	log.Printf("Connecting to LIVIVA Server at %s...", addr)

	// Dial Server
	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	defer conn.Close()

	c := api.NewLivivaServiceClient(conn)

	// Start Chat Session
	ctx := context.Background()
	stream, err := c.ChatSession(ctx)
	if err != nil {
		log.Fatalf("error creating stream: %v", err)
	}

	// Define sender function for TUI (returns tea.Cmd for async execution)
	sender := func(text string) tea.Cmd {
		return func() tea.Msg {
			// Local Command Detection
			if strings.HasPrefix(text, "/voice on") {
				startSTT(stream)
			} else if strings.HasPrefix(text, "/voice off") {
				stopSTT()
			} else if strings.HasPrefix(text, "/upload ") {
				path := strings.TrimSpace(strings.TrimPrefix(text, "/upload "))
				if err := sendArtifact(stream, path); err != nil {
					return errMsg(err)
				}
				// The server will send a confirmation back
				return nil
			}

			// @Mention detection
			words := strings.Fields(text)
			for _, word := range words {
				if strings.HasPrefix(word, "@") {
					path := strings.TrimPrefix(word, "@")
					// Check if file exists localy
					if _, err := os.Stat(path); err == nil {
						p.Send(serverMsg{text: fmt.Sprintf("Uploading %s...", path), isSystem: true})
						if err := sendArtifact(stream, path); err != nil {
							p.Send(errMsg(fmt.Errorf("failed to upload %s: %v", path, err)))
						}
					}
				}
			}

			sendMu.Lock()
			defer sendMu.Unlock()
			if err := stream.Send(&api.ClientMessage{
				Payload: &api.ClientMessage_Text{
					Text: text,
				},
			}); err != nil {
				return nil
			}
			return nil
		}
	}

	// Initialize TUI
	m := initialModel(sender)
	p = tea.NewProgram(m, tea.WithAltScreen())

	// Receiver Goroutine (Server -> Client)
	go func() {
		for {
			in, err := stream.Recv()
			if err == io.EOF {
				return
			}
			if err != nil {
				p.Send(errMsg(err))
				return
			}

			switch pld := in.Payload.(type) {
			case *api.ServerMessage_Text:
				p.Send(serverMsg{text: pld.Text})
			case *api.ServerMessage_SystemLog:
				p.Send(serverMsg{text: pld.SystemLog, isSystem: true})
			case *api.ServerMessage_SpeakText:
				p.Send(serverMsg{text: pld.SpeakText, isVoice: true})
				go speak(pld.SpeakText)
			case *api.ServerMessage_ToolRequest:
				go handleToolRequest(stream, pld.ToolRequest)
			case *api.ServerMessage_Artifact:
				// Handle artifact download if needed
			case *api.ServerMessage_Metrics:
				p.Send(metricsMsg{
					promptTokens:     pld.Metrics.PromptTokens,
					completionTokens: pld.Metrics.CompletionTokens,
					totalTokens:      pld.Metrics.TotalTokens,
					contextPct:       pld.Metrics.ContextPercentage,
				})
			}
		}
	}()

	if _, err := p.Run(); err != nil {
		log.Fatal(err)
	}

	stream.CloseSend()
	stopSTT()
}

// startSTT launches the microphone listener script
func startSTT(stream api.LivivaService_ChatSessionClient) {
	sttMu.Lock()
	defer sttMu.Unlock()

	if sttRunning {
		return
	}

	// Resolution of bins: Check .venv first, then PATH
	cwd, _ := os.Getwd()
	venvPython := filepath.Join(cwd, ".venv", "bin", "python3")
	pythonPath := "python3"
	if _, err := os.Stat(venvPython); err == nil {
		pythonPath = venvPython
	}

	scriptPath := "./scripts/listen.py"
	if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
		scriptPath = filepath.Join(cwd, "scripts", "listen.py")
	}

	cmd := exec.Command(pythonPath, scriptPath)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		msg := fmt.Sprintf("STT Error: failed to get stdout pipe: %v", err)
		log.Print(msg)
		if p != nil {
			p.Send(serverMsg{text: msg, isSystem: true})
		}
		return
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		msg := fmt.Sprintf("STT Error: failed to get stdin pipe: %v", err)
		log.Print(msg)
		return
	}
	sttStdin = stdin

	stderr, err := cmd.StderrPipe()
	if err != nil {
		log.Printf("STT Error: failed to get stderr pipe: %v", err)
		return
	}

	if err := cmd.Start(); err != nil {
		msg := fmt.Sprintf("STT Error: failed to start stt.py: %v", err)
		log.Print(msg)
		if p != nil {
			p.Send(serverMsg{text: msg, isSystem: true})
		}
		return
	}

	sttCmd = cmd
	sttRunning = true
	if p != nil {
		p.Send(recordingMsg(true))
	}

	// Goroutine to read stderr
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			log.Printf("STT Debug: %s", scanner.Text())
		}
	}()

	// Goroutine to read transcribed text
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			transcribed := scanner.Text()
			if transcribed != "" {
				if transcribed == "[SPEAKING] START" {
					if p != nil {
						p.Send(playingMsg(true))
					}
					continue
				}
				if transcribed == "[SPEAKING] END" {
					if p != nil {
						p.Send(playingMsg(false))
					}
					continue
				}

				if p != nil {
					p.Send(serverMsg{text: transcribed, isVoice: true})
				}

				sendMu.Lock()
				stream.Send(&api.ClientMessage{
					Payload: &api.ClientMessage_Text{
						Text: transcribed,
					},
				})
				sendMu.Unlock()
			}
		}
		sttMu.Lock()
		sttRunning = false
		sttStdin = nil
		if p != nil {
			p.Send(recordingMsg(false))
		}
		sttMu.Unlock()
	}()
}

// stopSTT kills the microphone listener script
func stopSTT() {
	sttMu.Lock()
	defer sttMu.Unlock()

	if !sttRunning || sttCmd == nil {
		return
	}

	if err := sttCmd.Process.Signal(os.Interrupt); err != nil {
		sttCmd.Process.Kill()
	}
	sttCmd.Wait()
	sttRunning = false
	if p != nil {
		p.Send(recordingMsg(false))
	}
}

// speak executes local TTS and plays audio
func speak(text string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}

	sttMu.Lock()
	defer sttMu.Unlock()

	if !sttRunning || sttStdin == nil {
		log.Printf("Voice Warning: Skip speaking, voice mode is off")
		return
	}

	// Just write to listen.py stdin
	fmt.Fprintln(sttStdin, text)

	// Note: We don't have a direct way to know when listen.py finishes speaking
	// from here without more complex IPC, but listen.py itself manages the
	// is_agent_speaking flag internally.
}

// handleToolRequest executes the requested tool and sends the response back
func handleToolRequest(stream api.LivivaService_ChatSessionClient, req *api.ToolRequest) {
	msg := fmt.Sprintf("Tool Request: %s (ID: %s)", req.ToolName, req.Id)
	log.Print(msg)
	if p != nil {
		p.Send(serverMsg{text: msg, isSystem: true})
	}

	var output string
	var errVal string

	switch req.ToolName {
	case "system.exec":
		var args struct {
			Command []string `json:"command"`
		}
		if err := json.Unmarshal([]byte(req.ArgumentsJson), &args); err != nil {
			errVal = fmt.Sprintf("failed to parse arguments: %v", err)
			break
		}
		if len(args.Command) == 0 {
			errVal = "no command specified"
			break
		}

		cmd := exec.Command(args.Command[0], args.Command[1:]...)
		out, err := cmd.CombinedOutput()
		output = string(out)
		if err != nil {
			errVal = fmt.Sprintf("execution failed: %v", err)
		}

	default:
		errVal = fmt.Sprintf("unknown tool: %s", req.ToolName)
	}

	// Send Response
	sendMu.Lock()
	defer sendMu.Unlock()
	stream.Send(&api.ClientMessage{
		Payload: &api.ClientMessage_ToolResponse{
			ToolResponse: &api.ToolResponse{
				Id:     req.Id,
				Output: output,
				Error:  errVal,
			},
		},
	})
}

// sendArtifact reads a local file and sends it to the server
func sendArtifact(stream api.LivivaService_ChatSessionClient, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	ext := filepath.Ext(path)
	mimeType := mime.TypeByExtension(ext)
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}

	sendMu.Lock()
	defer sendMu.Unlock()

	return stream.Send(&api.ClientMessage{
		Payload: &api.ClientMessage_SaveArtifactRequest{
			SaveArtifactRequest: &api.Artifact{
				Filename: filepath.Base(path),
				Data:     data,
				MimeType: mimeType,
			},
		},
	})
}
