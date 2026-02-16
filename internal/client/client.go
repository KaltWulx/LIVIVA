package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"sync"

	tea "github.com/charmbracelet/bubbletea"
	executor "github.com/kalt/liviva/internal/client/exec"
	"github.com/kalt/liviva/pkg/api"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	p      *tea.Program
	sendMu sync.Mutex // Mutex for gRPC stream
	voice  *VoiceService
)

// Run starts the client TUI
func Run(addr string) {
	// Redirect logs to file to avoid TUI corruption
	f, err := os.OpenFile("liviva-client.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("error opening log file: %v", err)
	}
	defer f.Close()
	log.SetOutput(f)

	log.Printf("Connecting to LIVIVA Server at %s...", addr)

	// Initialize Voice Service
	voice = NewVoiceService()

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
				voice.Start(stream, &sendMu)
			} else if strings.HasPrefix(text, "/voice off") {
				voice.Stop()
			} else if strings.HasPrefix(text, "/upload ") {
				path := strings.TrimSpace(strings.TrimPrefix(text, "/upload "))
				if err := sendArtifact(stream, path); err != nil {
					return errMsg(err)
				}
				// The server will send a confirmation back
				return nil
			}

			// Smart File Detection (with or without @)
			words := strings.Fields(text)
			for _, word := range words {
				candidate := word
				// 1. Explicit @mention
				if strings.HasPrefix(word, "@") {
					candidate = strings.TrimPrefix(word, "@")
				} else {
					// 2. Implicit detection by extension (for voice/ease)
					// Only checks likely filenames to avoid spam
					if !strings.Contains(candidate, ".") {
						continue
					}
					// Filter out common punctuation if attached
					candidate = strings.TrimRight(candidate, ".,;?!")
				}

				// Check if file exists locally
				if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
					// Prevent redundant uploads logic could go here, but for now we trust the existence check
					p.Send(serverMsg{text: fmt.Sprintf("Auto-detect: Uploading %s...", candidate), isSystem: true})
					if err := sendArtifact(stream, candidate); err != nil {
						p.Send(errMsg(fmt.Errorf("failed to upload %s: %v", candidate, err)))
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

	var (
		execMu         sync.Mutex
		activeExecutor *executor.Executor
		toolInputCh    = make(chan string, 10)
		toolRequestCh  = make(chan *api.ToolRequest, 100) // Buffer for incoming tool requests
	)

	// Input Router
	go func() {
		for input := range toolInputCh {
			execMu.Lock()
			if activeExecutor != nil {
				_ = activeExecutor.WriteInput([]byte(input))
			}
			execMu.Unlock()
		}
	}()

	// Tool Execution Worker (Serialized)
	// This ensures that only ONE tool executes at a time, preventing race conditions
	// on the PTY and Input Router.
	go func() {
		for req := range toolRequestCh {
			// Pass shared state pointers
			// Executed synchronously by this worker
			handleToolRequest(stream, req, &execMu, &activeExecutor)
		}
	}()

	// Initialize TUI
	m := NewAppModel(sender, toolInputCh)
	p = tea.NewProgram(m, tea.WithAltScreen())
	voice.SetProgram(p)

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
				go voice.Speak(pld.SpeakText)
			case *api.ServerMessage_ToolRequest:
				// Push to queue for serialized execution
				select {
				case toolRequestCh <- pld.ToolRequest:
				default:
					log.Printf("Tool request queue full, dropping request: %s", pld.ToolRequest.Id)
					p.Send(errMsg(fmt.Errorf("tool queue full, dropped request %s", pld.ToolRequest.Id)))
				}
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
	voice.Stop()
}

// Deprecated functions removed (moved to VoiceService)

// handleToolRequest executes the requested tool and sends the response back
func handleToolRequest(stream api.LivivaService_ChatSessionClient, req *api.ToolRequest, mu *sync.Mutex, activeEx **executor.Executor) {
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

		// Initialize PTY Executor
		exe := executor.NewExecutor()
		mu.Lock()
		*activeEx = exe
		mu.Unlock()

		// Notify TUI Execution Mode
		p.Send(executingMsg(true))

		ptmx, err := exe.Start(args.Command)
		if err != nil {
			errVal = fmt.Sprintf("execution failed: %v", err)
			p.Send(executingMsg(false))
			mu.Lock()
			*activeEx = nil
			mu.Unlock()
			break
		}

		// Stream Output
		var outBuf strings.Builder
		buf := make([]byte, 1024)
		for {
			n, err := ptmx.Read(buf)
			if n > 0 {
				chunk := string(buf[:n])
				outBuf.WriteString(chunk) // Buffer for final response
				// Stream clean lines to TUI for visibility
				// Note: Raw PTY output might have control chars, for now we just dump it
				p.Send(serverMsg{text: chunk, isSystem: true})
			}
			if err != nil {
				break
			}
		}

		exe.Wait()
		exe.Close()
		output = outBuf.String()

		// Cleanup
		mu.Lock()
		*activeEx = nil
		mu.Unlock()
		p.Send(executingMsg(false))

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
