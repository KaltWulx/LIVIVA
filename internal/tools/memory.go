package tools

import (
	"fmt"
	"iter"
	"time"

	"google.golang.org/adk/memory"
	"google.golang.org/adk/session"
	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"
	"google.golang.org/genai"
)

// NewRememberTool creates a tool for storing information in long-term memory.
func NewRememberTool(svc memory.Service) tool.Tool {
	t, err := functiontool.New(
		functiontool.Config{
			Name:        "remember",
			Description: "CRITICAL: Stores important facts or user preferences in long-term memory. Use this to save information that MUST be remembered across different chat sessions. You MUST provide the 'content' parameter.",
		},
		func(tctx tool.Context, args struct {
			Content string `json:"content" jsonschema:"The information to remember."`
		}) (map[string]any, error) {
			if args.Content == "" {
				return nil, fmt.Errorf("the 'content' parameter is required and cannot be empty")
			}

			// In ADK, we usually add sessions. Here we create a "synthetic" session or event.
			sess := &mockSession{
				id:      "manual-memory",
				appName: "LIVIVA",
				userID:  "local-user",
				events: []*session.Event{
					{
						Timestamp: time.Now(),
						Author:    "user",
					},
				},
			}
			// Set the content on the event
			sess.events[0].Content = &genai.Content{
				Parts: []*genai.Part{{Text: args.Content}},
				Role:  "user",
			}

			if err := svc.AddSession(tctx, sess); err != nil {
				return nil, fmt.Errorf("failed to remember: %w", err)
			}
			return map[string]any{"status": "success", "message": fmt.Sprintf("Fact remembered: %s", args.Content)}, nil
		},
	)
	if err != nil {
		panic(err)
	}
	return t
}

// NewRecallTool creates a tool for searching long-term memory.
func NewRecallTool(svc memory.Service) tool.Tool {
	t, err := functiontool.New(
		functiontool.Config{
			Name:        "recall",
			Description: "Searches long-term memory for relevant information about past conversations or stored facts.",
		},
		func(tctx tool.Context, args struct {
			Query string `json:"query" jsonschema:"The search term or question to look up in memory."`
		}) (map[string]any, error) {
			if args.Query == "" {
				return nil, fmt.Errorf("query is required")
			}

			// Use ADK Search
			resp, err := svc.Search(tctx, &memory.SearchRequest{
				Query:   args.Query,
				UserID:  "local-user", // Should be dynamic in a multi-user app
				AppName: "LIVIVA",
			})
			if err != nil {
				return nil, fmt.Errorf("failed to recall: %w", err)
			}

			var results []map[string]interface{}
			for _, m := range resp.Memories {
				content := ""
				if m.Content != nil {
					for _, p := range m.Content.Parts {
						content += p.Text
					}
				}
				results = append(results, map[string]interface{}{
					"content":   content,
					"author":    m.Author,
					"timestamp": m.Timestamp.Format(time.RFC3339),
				})
			}

			if len(results) == 0 {
				return map[string]any{"memories": []interface{}{}, "message": "No relevant memories found."}, nil
			}

			return map[string]any{"memories": results}, nil
		},
	)
	if err != nil {
		panic(err)
	}
	return t
}

// mockSession is a helper for manual memory ingestion
type mockSession struct {
	id      string
	appName string
	userID  string
	events  []*session.Event
}

func (m *mockSession) ID() string                { return m.id }
func (m *mockSession) AppName() string           { return m.appName }
func (m *mockSession) UserID() string            { return m.userID }
func (m *mockSession) State() session.State      { return nil }
func (m *mockSession) Events() session.Events    { return &mockEvents{m.events} }
func (m *mockSession) LastUpdateTime() time.Time { return time.Now() }

type mockEvents struct {
	events []*session.Event
}

func (e *mockEvents) Len() int { return len(e.events) }
func (e *mockEvents) At(i int) *session.Event {
	if i < 0 || i >= len(e.events) {
		return nil
	}
	return e.events[i]
}
func (e *mockEvents) All() iter.Seq[*session.Event] {
	return func(yield func(*session.Event) bool) {
		for _, evt := range e.events {
			if !yield(evt) {
				return
			}
		}
	}
}
