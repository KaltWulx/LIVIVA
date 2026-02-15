package server

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"os"

	"encoding/json"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"github.com/kalt/liviva/internal/agents"
	"github.com/kalt/liviva/internal/llm"
	"github.com/kalt/liviva/pkg/api"
	"google.golang.org/adk/agent"
	"google.golang.org/adk/artifact"
	"google.golang.org/adk/model"
	"google.golang.org/adk/runner"
	"google.golang.org/adk/session"
	"google.golang.org/genai"
	"google.golang.org/grpc"
)

type livivaServer struct {
	api.UnimplementedLivivaServiceServer
	llmModel        model.LLM
	sessionService  session.Service
	artifactService artifact.Service
}

type safeStream struct {
	stream api.LivivaService_ChatSessionServer
	mu     sync.Mutex
}

func (s *safeStream) Send(msg *api.ServerMessage) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.stream.Send(msg)
}

// toolDispatcher implements tools.RemoteDispatcher
type toolDispatcher struct {
	safeStream *safeStream
	responses  map[string]chan string
	mu         sync.Mutex
}

func (d *toolDispatcher) SendToolRequest(toolName string, args any) (string, error) {
	id := uuid.New().String()
	argsBytes, err := json.Marshal(args)
	if err != nil {
		return "", fmt.Errorf("failed to marshal args: %w", err)
	}

	respChan := make(chan string, 1)
	d.mu.Lock()
	if d.responses == nil {
		d.responses = make(map[string]chan string)
	}
	d.responses[id] = respChan
	d.mu.Unlock()

	defer func() {
		d.mu.Lock()
		delete(d.responses, id)
		d.mu.Unlock()
	}()

	err = d.safeStream.Send(&api.ServerMessage{
		Payload: &api.ServerMessage_ToolRequest{
			ToolRequest: &api.ToolRequest{
				Id:            id,
				ToolName:      toolName,
				ArgumentsJson: string(argsBytes),
			},
		},
	})
	if err != nil {
		return "", fmt.Errorf("failed to send tool request: %w", err)
	}

	// Wait for response with timeout
	select {
	case output := <-respChan:
		return output, nil
	case <-time.After(30 * time.Second):
		return "", fmt.Errorf("tool execution timed out")
	}
}

// voiceWriter sends text to be spoken by the client
type voiceWriter struct {
	safeStream *safeStream
}

func (w *voiceWriter) Write(p []byte) (n int, err error) {
	text := string(p)
	if text == "" {
		return 0, nil
	}
	err = w.safeStream.Send(&api.ServerMessage{
		Payload: &api.ServerMessage_SpeakText{
			SpeakText: text,
		},
	})
	if err != nil {
		return 0, err
	}
	return len(p), nil
}

func (s *livivaServer) ChatSession(stream api.LivivaService_ChatSessionServer) error {
	log.Println("New Client Connected")

	userId := "local-user"
	sessionId := fmt.Sprintf("session-%d", os.Getpid())

	// Ensure session exists
	if _, err := s.sessionService.Create(stream.Context(), &session.CreateRequest{
		AppName:   "LIVIVA",
		UserID:    userId,
		SessionID: sessionId,
	}); err != nil {
		log.Printf("Warning: Session creation failed (it might already exist): %v", err)
	}

	// Wrapper for thread-safe sending
	safe := &safeStream{stream: stream}

	// dedicated writer for voice tool
	vWriter := &voiceWriter{safeStream: safe}

	// dedicated dispatcher for tools
	tDispatcher := &toolDispatcher{safeStream: safe, responses: make(map[string]chan string)}

	// Create Coordinator for this session
	coord, err := agents.NewCoordinator(s.llmModel, vWriter, tDispatcher)
	if err != nil {
		return fmt.Errorf("failed to create coordinator: %w", err)
	}

	// Create Runner
	r, err := runner.New(runner.Config{
		AppName:         "LIVIVA",
		Agent:           coord,
		SessionService:  s.sessionService,
		ArtifactService: s.artifactService,
	})
	if err != nil {
		return fmt.Errorf("failed to create runner: %w", err)
	}

	// Send welcome message
	if err := safe.Send(&api.ServerMessage{
		Payload: &api.ServerMessage_Text{
			Text: "LIVIVA Session Started. How can I help you?",
		},
	}); err != nil {
		return err
	}

	for {
		in, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}

		switch p := in.Payload.(type) {
		case *api.ClientMessage_ToolResponse:
			// Route response to the waiting dispatcher
			resp := p.ToolResponse
			log.Printf("Received Tool Response for ID: %s", resp.Id)
			tDispatcher.mu.Lock()
			if ch, ok := tDispatcher.responses[resp.Id]; ok {
				if resp.Error != "" {
					ch <- fmt.Sprintf("ERROR: %s", resp.Error)
				} else {
					ch <- resp.Output
				}
			}
			tDispatcher.mu.Unlock()

		case *api.ClientMessage_SaveArtifactRequest:
			art := p.SaveArtifactRequest
			log.Printf("Received Artifact Upload: %s", art.Filename)
			part := &genai.Part{
				InlineData: &genai.Blob{
					MIMEType: art.MimeType,
					Data:     art.Data,
				},
			}
			resp, err := s.artifactService.Save(stream.Context(), &artifact.SaveRequest{
				AppName:   "LIVIVA",
				UserID:    userId,
				SessionID: sessionId,
				FileName:  art.Filename,
				Part:      part,
			})
			if err != nil {
				log.Printf("Error saving artifact: %v", err)
				safe.Send(&api.ServerMessage{
					Payload: &api.ServerMessage_SystemLog{
						SystemLog: fmt.Sprintf("Error saving artifact %s: %v", art.Filename, err),
					},
				})
			} else {
				log.Printf("Saved artifact %s (Version: %d)", art.Filename, resp.Version)
				safe.Send(&api.ServerMessage{
					Payload: &api.ServerMessage_SystemLog{
						SystemLog: fmt.Sprintf("File uploaded: %s (Version: %d)", art.Filename, resp.Version),
					},
				})
			}

		case *api.ClientMessage_Text:
			log.Printf("Processing Request: %s", p.Text)

			// Handle Agent execution in a separate goroutine to avoid blocking Recv (DEADLOCK FIX)
			go func(userText string) {
				msg := &genai.Content{
					Role:  genai.RoleUser,
					Parts: []*genai.Part{{Text: userText}},
				}

				// Run the agent turn and stream events back
				for event, err := range r.Run(context.Background(), userId, sessionId, msg, agent.RunConfig{}) {
					if err != nil {
						log.Printf("Agent Error: %v", err)
						continue
					}

					if event.LLMResponse.Content != nil {
						for _, part := range event.LLMResponse.Content.Parts {
							if part.Text != "" {
								safe.Send(&api.ServerMessage{
									Payload: &api.ServerMessage_Text{
										Text: part.Text,
									},
								})
							}
						}
						if event.LLMResponse.UsageMetadata != nil {
							usage := event.LLMResponse.UsageMetadata
							// Simple context percentage calculation (assuming 128k limit for now)
							// TODO: Make this dynamic based on model name
							percentage := int32((usage.TotalTokenCount * 100) / 128000)
							if percentage > 100 {
								percentage = 100
							}

							safe.Send(&api.ServerMessage{
								Payload: &api.ServerMessage_Metrics{
									Metrics: &api.Metrics{
										PromptTokens:      usage.PromptTokenCount,
										CompletionTokens:  usage.CandidatesTokenCount,
										TotalTokens:       usage.TotalTokenCount,
										ContextPercentage: percentage,
									},
								},
							})
						}
					}
				}
			}(p.Text)
		}
	}
}

// Run starts the gRPC server
func Run() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env found")
	}

	apiKey := os.Getenv("OPENAI_API_KEY")
	copilotKey := os.Getenv("COPILOT_API_KEY")
	modelName := os.Getenv("LIVIVA_MODEL")
	if modelName == "" {
		modelName = "gpt-4o"
	}

	var llmModel *llm.OpenAIModel
	if copilotKey != "" {
		log.Println("Using GitHub Copilot LLM Configuration")
		headers := map[string]string{
			"Editor-Version":        "vscode/1.85.1",
			"Editor-Plugin-Version": "copilot/1.143.0",
		}
		llmModel = llm.NewOpenAIModel(copilotKey, modelName, "https://api.githubcopilot.com", headers)
	} else {
		llmModel = llm.NewOpenAIModel(apiKey, modelName, "", nil)
	}

	sessionService := session.InMemoryService()
	artifactService := artifact.InMemoryService()

	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	s := grpc.NewServer()
	api.RegisterLivivaServiceServer(s, &livivaServer{
		llmModel:        llmModel,
		sessionService:  sessionService,
		artifactService: artifactService,
	})

	log.Printf("LIVIVA gRPC Server listening on %v", lis.Addr())
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
