package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"iter"
	"log"
	"net/http"
	"strings"

	"google.golang.org/adk/model"
	"google.golang.org/genai"
)

// OpenAIModel implements model.LLM interface for OpenAI-compatible APIs
type OpenAIModel struct {
	client       *http.Client
	apiKey       string
	model        string
	baseURL      string
	extraHeaders map[string]string
	Debug        bool
}

// NewOpenAIModel creates a new OpenAIModel adapter
func NewOpenAIModel(apiKey, modelName, baseURL string, extraHeaders map[string]string) *OpenAIModel {
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	return &OpenAIModel{
		client:       &http.Client{},
		apiKey:       apiKey,
		model:        modelName,
		baseURL:      baseURL,
		extraHeaders: extraHeaders,
	}
}

func (m *OpenAIModel) SetDebug(debug bool) {
	m.Debug = debug
}

func (m *OpenAIModel) Name() string {
	return m.model
}

// GenerateContent implements model.LLM.GenerateContent with iter.Seq2 for streaming
func (m *OpenAIModel) GenerateContent(ctx context.Context, req *model.LLMRequest, stream bool) iter.Seq2[*model.LLMResponse, error] {
	return func(yield func(*model.LLMResponse, error) bool) {
		// 1. Prepare OpenAI Request
		openAIReq, err := m.prepareRequest(req, stream)
		if err != nil {
			yield(nil, fmt.Errorf("failed to prepare openai request: %w", err))
			return
		}

		// 2. Execute HTTP Request
		// Log Request for Debugging
		if m.Debug {
			log.Printf("[OpenAI] Request Payload:\n%s", string(openAIReq))
		}

		httpReq, err := http.NewRequestWithContext(ctx, "POST", m.baseURL+"/chat/completions", bytes.NewBuffer(openAIReq))
		if err != nil {
			yield(nil, fmt.Errorf("failed to create http request: %w", err))
			return
		}

		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("Authorization", "Bearer "+m.apiKey)

		for k, v := range m.extraHeaders {
			httpReq.Header.Set(k, v)
		}

		resp, err := m.client.Do(httpReq)
		if err != nil {
			yield(nil, fmt.Errorf("failed to execute http request: %w", err))
			return
		}
		defer resp.Body.Close()

		// Read Body for Debugging
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			yield(nil, fmt.Errorf("failed to read response body: %w", err))
			return
		}
		if m.Debug {
			log.Printf("[OpenAI] Raw Response:\n%s", string(bodyBytes))
		}

		// Restore Body for Decoding
		resp.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

		if resp.StatusCode != http.StatusOK {
			// Read error body
			yield(nil, fmt.Errorf("openai api error (status %d): %s", resp.StatusCode, string(bodyBytes)))
			return
		}

		// 3. Handle Streaming Response
		if stream {
			scanner := bufio.NewScanner(resp.Body)
			// Track accumulated tool calls
			type toolCallAccumulator struct {
				id        string
				name      string
				arguments strings.Builder
			}
			accumulatedTools := make(map[int]*toolCallAccumulator)
			var accumulatedText strings.Builder
			var lastUsage *OpenAIUsage

			for scanner.Scan() {
				line := scanner.Text()
				if !strings.HasPrefix(line, "data: ") {
					continue
				}
				data := strings.TrimPrefix(line, "data: ")
				if data == "[DONE]" {
					break
				}

				var streamResp OpenAIStreamResponse
				if err := json.Unmarshal([]byte(data), &streamResp); err != nil {
					log.Printf("error unmarshalling stream data: %v", err)
					continue
				}

				if streamResp.Usage != nil {
					lastUsage = streamResp.Usage
				}

				if len(streamResp.Choices) > 0 {
					choice := streamResp.Choices[0]

					// 1. Handle Content Delta
					content := choice.Delta.Content
					if content != "" {
						accumulatedText.WriteString(content)
						// Yield partial text for immediate UI feedback
						modelResp := &model.LLMResponse{
							Content: &genai.Content{
								Role:  genai.RoleModel,
								Parts: []*genai.Part{{Text: content}},
							},
							Partial: true,
						}
						if !yield(modelResp, nil) {
							return
						}
					}

					// 2. Handle Tool Call Deltas
					for _, tcDelta := range choice.Delta.ToolCalls {
						idx := tcDelta.Index
						acc, ok := accumulatedTools[idx]
						if !ok {
							acc = &toolCallAccumulator{
								id:   tcDelta.ID,
								name: tcDelta.Function.Name,
							}
							accumulatedTools[idx] = acc
						}
						if tcDelta.ID != "" {
							acc.id = tcDelta.ID
						}
						if tcDelta.Function.Name != "" {
							acc.name = tcDelta.Function.Name
						}
						acc.arguments.WriteString(tcDelta.Function.Arguments)
					}
				}
			}

			// End of stream: Finalize and yield non-partial events for history

			// Yield Full Accumulated Text and Tool Calls as a SINGLE non-partial event
			if accumulatedText.Len() > 0 || len(accumulatedTools) > 0 {
				parts := []*genai.Part{}
				if accumulatedText.Len() > 0 {
					parts = append(parts, &genai.Part{Text: accumulatedText.String()})
				}

				if len(accumulatedTools) > 0 {
					for i := 0; i < len(accumulatedTools); i++ {
						acc, ok := accumulatedTools[i]
						if !ok {
							continue
						}
						var args map[string]any
						if err := json.Unmarshal([]byte(acc.arguments.String()), &args); err != nil {
							log.Printf("[OpenAI] failed to unmarshal streamed function args: %v", err)
							args = map[string]any{}
						}
						parts = append(parts, &genai.Part{
							FunctionCall: &genai.FunctionCall{
								ID:   acc.id,
								Name: acc.name,
								Args: args,
							},
						})
					}
				}

				if m.Debug {
					log.Printf("[OpenAI] Yielding Consolidated Streamed Event: %d parts", len(parts))
				}
				modelResp := &model.LLMResponse{
					Content: &genai.Content{
						Role:  genai.RoleModel,
						Parts: parts,
					},
					Partial:      false,
					TurnComplete: true,
				}
				if lastUsage != nil {
					modelResp.UsageMetadata = &genai.GenerateContentResponseUsageMetadata{
						PromptTokenCount:     int32(lastUsage.PromptTokens),
						CandidatesTokenCount: int32(lastUsage.CompletionTokens),
						TotalTokenCount:      int32(lastUsage.TotalTokens),
					}
				}
				if !yield(modelResp, nil) {
					return
				}
			}

			if err := scanner.Err(); err != nil {
				yield(nil, fmt.Errorf("error reading stream: %w", err))
			}
			return
		}

		// 4. Handle Non-Streaming Response
		var fullResp OpenAIResponse
		if err := json.NewDecoder(resp.Body).Decode(&fullResp); err != nil {
			yield(nil, fmt.Errorf("failed to decode response: %w", err))
			return
		}

		if len(fullResp.Choices) > 0 {
			msg := fullResp.Choices[0].Message
			parts := []*genai.Part{}

			if msg.Content != nil {
				if text, ok := msg.Content.(string); ok && text != "" {
					parts = append(parts, &genai.Part{Text: text})
				}
			}

			// Handle Tool Calls
			for _, tc := range msg.ToolCalls {
				if m.Debug {
					log.Printf("[OpenAI] Received Tool Call: %s(%s)", tc.Function.Name, tc.Function.Arguments)
				}

				// Parse arguments JSON
				var args map[string]any
				if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
					log.Printf("[OpenAI] failed to unmarshal function args: %v", err)
					args = map[string]any{} // fallback
				}

				parts = append(parts, &genai.Part{
					FunctionCall: &genai.FunctionCall{
						ID:   tc.ID,
						Name: tc.Function.Name,
						Args: args,
					},
				})
			}

			modelResp := &model.LLMResponse{
				Content: &genai.Content{
					Role:  genai.RoleModel,
					Parts: parts,
				},
				TurnComplete: true,
			}
			if fullResp.Usage != nil {
				modelResp.UsageMetadata = &genai.GenerateContentResponseUsageMetadata{
					PromptTokenCount:     int32(fullResp.Usage.PromptTokens),
					CandidatesTokenCount: int32(fullResp.Usage.CompletionTokens),
					TotalTokenCount:      int32(fullResp.Usage.TotalTokens),
				}
			}
			yield(modelResp, nil)
		}
	}
}

// Helper structs for OpenAI API
type OpenAIRequest struct {
	Model         string               `json:"model"`
	Messages      []OpenAIMessage      `json:"messages"`
	Stream        bool                 `json:"stream"`
	StreamOptions *OpenAIStreamOptions `json:"stream_options,omitempty"`
	Tools         []OpenAITool         `json:"tools,omitempty"`
	ToolChoice    any                  `json:"tool_choice,omitempty"`
}

type OpenAIStreamOptions struct {
	IncludeUsage bool `json:"include_usage"`
}

type OpenAIMessage struct {
	Role       string           `json:"role"`
	Content    any              `json:"content,omitempty"` // string or []OpenAIContentPart
	ToolCalls  []OpenAIToolCall `json:"tool_calls,omitempty"`
	ToolCallID string           `json:"tool_call_id,omitempty"`
}

type OpenAIContentPart struct {
	Type     string          `json:"type"`
	Text     string          `json:"text,omitempty"`
	ImageURL *OpenAIImageURL `json:"image_url,omitempty"`
}

type OpenAIImageURL struct {
	URL string `json:"url"`
}

type OpenAIResponse struct {
	Choices []struct {
		Message OpenAIMessage `json:"message"`
	} `json:"choices"`
	Usage *OpenAIUsage `json:"usage,omitempty"`
}

type OpenAIUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type OpenAIStreamResponse struct {
	Choices []struct {
		Delta struct {
			Content   string                 `json:"content"`
			ToolCalls []OpenAIStreamToolCall `json:"tool_calls,omitempty"`
		} `json:"delta"`
		FinishReason string `json:"finish_reason,omitempty"`
	} `json:"choices"`
	Usage *OpenAIUsage `json:"usage,omitempty"`
}

type OpenAIStreamToolCall struct {
	Index    int                `json:"index"`
	ID       string             `json:"id,omitempty"`
	Type     string             `json:"type,omitempty"`
	Function OpenAIFunctionCall `json:"function"`
}

// prepareRequest converts ADK LLMRequest to OpenAI API JSON
func (m *OpenAIModel) prepareRequest(req *model.LLMRequest, stream bool) ([]byte, error) {
	messages := []OpenAIMessage{}
	// Handle System Instructions
	if req.Config != nil && req.Config.SystemInstruction != nil {
		var sb strings.Builder
		for _, part := range req.Config.SystemInstruction.Parts {
			if part.Text != "" {
				sb.WriteString(part.Text)
			}
		}
		if sb.Len() > 0 {
			messages = append(messages, OpenAIMessage{
				Role:    "system",
				Content: sb.String(),
			})
		}
	}

	// Convert previous history
	for _, content := range req.Contents {
		if content.Role == genai.RoleModel {
			// Assistant message (often includes tool calls)
			var textBuilder strings.Builder
			toolCalls := []OpenAIToolCall{}

			for _, part := range content.Parts {
				if part.Text != "" {
					textBuilder.WriteString(part.Text)
				}
				if part.FunctionCall != nil {
					argsBytes, _ := json.Marshal(part.FunctionCall.Args)
					toolCalls = append(toolCalls, OpenAIToolCall{
						ID:   part.FunctionCall.ID,
						Type: "function",
						Function: OpenAIFunctionCall{
							Name:      part.FunctionCall.Name,
							Arguments: string(argsBytes),
						},
					})
				}
			}

			messages = append(messages, OpenAIMessage{
				Role:      "assistant",
				Content:   textBuilder.String(),
				ToolCalls: toolCalls,
			})
		} else {
			// User or Tool response
			hasToolResponse := false
			for _, part := range content.Parts {
				if part.FunctionResponse != nil {
					hasToolResponse = true
					respBytes, _ := json.Marshal(part.FunctionResponse.Response)
					messages = append(messages, OpenAIMessage{
						Role:       "tool",
						Content:    string(respBytes),
						ToolCallID: part.FunctionResponse.ID,
					})
				}
			}

			if !hasToolResponse {
				// Regular user message - now supports multimodal (Text + InlineData)
				contentParts := []OpenAIContentPart{}
				for _, part := range content.Parts {
					if part.Text != "" {
						contentParts = append(contentParts, OpenAIContentPart{
							Type: "text",
							Text: part.Text,
						})
					}
					if part.InlineData != nil {
						// Convert binary data to base64 data URL
						data64 := base64.StdEncoding.EncodeToString(part.InlineData.Data)
						dataURL := fmt.Sprintf("data:%s;base64,%s", part.InlineData.MIMEType, data64)
						contentParts = append(contentParts, OpenAIContentPart{
							Type: "image_url",
							ImageURL: &OpenAIImageURL{
								URL: dataURL,
							},
						})
					}
				}

				// If only 1 text part, we can keep it simple as a string (optional)
				// But using []OpenAIContentPart is robust.
				messages = append(messages, OpenAIMessage{
					Role:    "user",
					Content: contentParts,
				})
			}
		}
	}

	// ToolDeclarer is an interface for tools that can provide their own GenAI declaration
	type ToolDeclarer interface {
		Declaration() *genai.FunctionDeclaration
	}

	// Map ADK Tools to OpenAI Tools
	var tools []OpenAITool
	if len(req.Tools) > 0 {
		for _, rawTool := range req.Tools {
			// Check if tool implements Declarer
			declarer, ok := rawTool.(ToolDeclarer)
			if !ok {
				log.Printf("warning: skipping tool %T (does not implement Declaration())", rawTool)
				continue
			}

			fn := declarer.Declaration()
			if fn == nil {
				continue
			}

			tools = append(tools, OpenAITool{
				Type: "function",
				Function: OpenAIFunction{
					Name:        fn.Name,
					Description: fn.Description,
					Parameters:  fn.Parameters,
				},
			})
		}
	}

	openAIReq := OpenAIRequest{
		Model:    m.model,
		Messages: messages,
		Stream:   stream,
	}

	if stream {
		openAIReq.StreamOptions = &OpenAIStreamOptions{IncludeUsage: true}
	}

	if len(tools) > 0 {
		openAIReq.Tools = tools
		// simple policy: auto
		openAIReq.ToolChoice = "auto"
	}

	return json.Marshal(openAIReq)
}

// Structs for OpenAI Tools
type OpenAITool struct {
	Type     string         `json:"type"`
	Function OpenAIFunction `json:"function"`
}

type OpenAIFunction struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Parameters  any    `json:"parameters"` // JSON Schema
}

type OpenAIToolCall struct {
	ID       string             `json:"id"`
	Type     string             `json:"type"`
	Function OpenAIFunctionCall `json:"function"`
}

type OpenAIFunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}
