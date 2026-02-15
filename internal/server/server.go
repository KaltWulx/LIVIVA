package server

import (
	"fmt"
	"io"
	"log"
	"net"
	"os"

	"github.com/joho/godotenv"
	"github.com/kalt/liviva/internal/agents"
	"github.com/kalt/liviva/internal/llm"
	"github.com/kalt/liviva/pkg/api"
	"google.golang.org/adk/agent"
	"google.golang.org/adk/model"
	"google.golang.org/adk/runner"
	"google.golang.org/adk/session"
	"google.golang.org/genai"
	"google.golang.org/grpc"
)

type livivaServer struct {
	api.UnimplementedLivivaServiceServer
	llmModel       model.LLM
	sessionService session.Service
}

// grpcWriter sends data back to the client over the gRPC stream
type grpcWriter struct {
	stream api.LivivaService_ChatSessionServer
}

func (w *grpcWriter) Write(p []byte) (n int, err error) {
	err = w.stream.Send(&api.ServerMessage{
		Payload: &api.ServerMessage_SystemLog{
			SystemLog: string(p),
		},
	})
	if err != nil {
		return 0, err
	}
	return len(p), nil
}

// voiceWriter sends text to be spoken by the client
type voiceWriter struct {
	stream api.LivivaService_ChatSessionServer
}

func (w *voiceWriter) Write(p []byte) (n int, err error) {
	text := string(p)
	if text == "" {
		return 0, nil
	}
	err = w.stream.Send(&api.ServerMessage{
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

	// dedicated writer for voice tool
	vWriter := &voiceWriter{stream: stream}

	// Create Coordinator for this session
	coord, err := agents.NewCoordinator(s.llmModel, vWriter)
	if err != nil {
		return fmt.Errorf("failed to create coordinator: %w", err)
	}

	// Create Runner
	r, err := runner.New(runner.Config{
		AppName:        "LIVIVA",
		Agent:          coord,
		SessionService: s.sessionService,
	})
	if err != nil {
		return fmt.Errorf("failed to create runner: %w", err)
	}

	// Send welcome message
	if err := stream.Send(&api.ServerMessage{
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
		case *api.ClientMessage_Text:
			log.Printf("Processing Request: %s", p.Text)

			msg := &genai.Content{
				Role:  genai.RoleUser,
				Parts: []*genai.Part{{Text: p.Text}},
			}

			// Run the agent turn and stream events back
			for event, err := range r.Run(stream.Context(), userId, sessionId, msg, agent.RunConfig{}) {
				if err != nil {
					log.Printf("Agent Error: %v", err)
					continue
				}

				if event.LLMResponse.Content != nil {
					for _, part := range event.LLMResponse.Content.Parts {
						if part.Text != "" {
							stream.Send(&api.ServerMessage{
								Payload: &api.ServerMessage_Text{
									Text: part.Text,
								},
							})
						}
					}
				}
			}
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

	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	s := grpc.NewServer()
	api.RegisterLivivaServiceServer(s, &livivaServer{
		llmModel:       llmModel,
		sessionService: sessionService,
	})

	log.Printf("LIVIVA gRPC Server listening on %v", lis.Addr())
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
