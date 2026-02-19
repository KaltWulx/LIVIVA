package callbacks

import (
	"testing"

	"google.golang.org/adk/model"
	"google.golang.org/genai"
)

func TestPruneHistory_Images(t *testing.T) {
	req := &model.LLMRequest{
		Contents: []*genai.Content{
			{
				Role: "user",
				Parts: []*genai.Part{
					{InlineData: &genai.Blob{Data: []byte("img1")}},
				},
			},
			{
				Role: "user",
				Parts: []*genai.Part{
					{InlineData: &genai.Blob{Data: []byte("img2")}},
				},
			},
			{
				Role: "user",
				Parts: []*genai.Part{
					{InlineData: &genai.Blob{Data: []byte("img3")}},
				},
			},
		},
	}

	_, err := PruneHistory(nil, req)
	if err != nil {
		t.Fatalf("PruneHistory failed: %v", err)
	}

	// Should keep only last 1 image
	imageCount := 0
	for _, c := range req.Contents {
		for _, p := range c.Parts {
			if p.InlineData != nil {
				imageCount++
			}
		}
	}

	if imageCount != 1 {
		t.Errorf("Expected 1 image, got %d", imageCount)
	}
}

func TestPruneHistory_SlidingWindow(t *testing.T) {
	req := &model.LLMRequest{
		Contents: make([]*genai.Content, 50),
	}
	for i := 0; i < 50; i++ {
		req.Contents[i] = &genai.Content{
			Role:  "user",
			Parts: []*genai.Part{{Text: "msg"}},
		}
	}

	_, err := PruneHistory(nil, req)
	if err != nil {
		t.Fatalf("PruneHistory failed: %v", err)
	}

	if len(req.Contents) > MaxHistoryTurns {
		t.Errorf("Expected at most %d turns, got %d", MaxHistoryTurns, len(req.Contents))
	}
}

func TestPruneHistory_SequenceSafety(t *testing.T) {
	// History with 51 turns
	// Naive truncation (remove 11) lands at index 11.
	// We ensure it starts with 'user'.
	req := &model.LLMRequest{
		Contents: make([]*genai.Content, 51),
	}
	for i := 0; i < 51; i++ {
		role := "user"
		if i == 11 {
			role = "tool" // Land here
		}
		if i == 12 {
			role = "user" // Skip to here
		}
		req.Contents[i] = &genai.Content{
			Role:  role,
			Parts: []*genai.Part{{Text: "msg"}},
		}
	}

	// The tool message at index 11 has a FunctionResponse to make it realistic
	req.Contents[11].Parts = []*genai.Part{{FunctionResponse: &genai.FunctionResponse{ID: "c1"}}}

	_, err := PruneHistory(nil, req)
	if err != nil {
		t.Fatalf("PruneHistory failed: %v", err)
	}

	if req.Contents[0].Role != "user" {
		t.Errorf("Pruned history MUST start with 'user' role, got %s", req.Contents[0].Role)
	}
}

func TestPruneHistory_EmptyRole(t *testing.T) {
	req := &model.LLMRequest{
		Contents: make([]*genai.Content, 50),
	}
	for i := 0; i < 50; i++ {
		req.Contents[i] = &genai.Content{
			Role:  "", // Empty role
			Parts: []*genai.Part{{Text: "msg"}},
		}
	}

	_, err := PruneHistory(nil, req)
	if err != nil {
		t.Fatalf("PruneHistory failed: %v", err)
	}

	if len(req.Contents) > MaxHistoryTurns {
		t.Errorf("Expected at most %d turns, got %d", MaxHistoryTurns, len(req.Contents))
	}
}

func TestPruneHistory_OrphanToolSplit(t *testing.T) {
	// History size: MaxHistoryTurns + 1 (41)
	// Naive truncation removes index 0, starts at index 1.
	// We place a 'tool' response at index 1.
	// The logic should skip it and find the next 'user' message.

	total := MaxHistoryTurns + 1
	contents := make([]*genai.Content, total)

	// Index 0: User (removed)
	contents[0] = &genai.Content{Role: "user", Parts: []*genai.Part{{Text: "U0"}}}

	// Index 1: Tool Response (naive start - SHOULD BE SKIPPED)
	contents[1] = &genai.Content{Role: "tool", Parts: []*genai.Part{{FunctionResponse: &genai.FunctionResponse{ID: "c1"}}}}

	// Index 2: Assistant Response
	contents[2] = &genai.Content{Role: "model", Parts: []*genai.Part{{Text: "A1"}}}

	// Index 3: Next User Message (TARGET START)
	contents[3] = &genai.Content{Role: "user", Parts: []*genai.Part{{Text: "U1"}}}

	// Rest are just users
	for i := 4; i < total; i++ {
		contents[i] = &genai.Content{Role: "user", Parts: []*genai.Part{{Text: "msg"}}}
	}

	req := &model.LLMRequest{Contents: contents}

	_, err := PruneHistory(nil, req)
	if err != nil {
		t.Fatalf("PruneHistory failed: %v", err)
	}

	if req.Contents[0].Role != "user" && req.Contents[0].Role != "" {
		t.Errorf("Pruned history MUST start with user/empty role, got %s", req.Contents[0].Role)
	}

	if req.Contents[0].Parts[0].Text != "U1" {
		t.Errorf("Expected first message to be 'U1', got %v", req.Contents[0].Parts[0].Text)
	}
}

// --- NEW: Tool-Call-Pair Preservation Tests ---

func TestPruneHistory_ToolCallPairPreservation(t *testing.T) {
	// Scenario: History has an assistant(tool_calls) → tool pair straddling
	// the naive cut point. Both must be either kept together or removed together.
	//
	// Layout (total: 45):
	//   [0..4]  = user messages
	//   [5]     = assistant with FunctionCall (tool_calls)
	//   [6]     = tool with FunctionResponse
	//   [7..44] = user messages (38 messages)
	//
	// Naive removeCount = 45 - 40 = 5 → lands at index 5 (assistant with tool_calls)
	// The algorithm should advance past the tool_call group to index 7 (next user).

	total := 45
	contents := make([]*genai.Content, total)

	for i := 0; i < 5; i++ {
		contents[i] = &genai.Content{Role: "user", Parts: []*genai.Part{{Text: "early"}}}
	}

	// Assistant with tool_calls at index 5
	contents[5] = &genai.Content{
		Role: genai.RoleModel,
		Parts: []*genai.Part{
			{FunctionCall: &genai.FunctionCall{ID: "tc1", Name: "screen_capture", Args: map[string]any{}}},
		},
	}

	// Tool response at index 6
	contents[6] = &genai.Content{
		Role: "tool",
		Parts: []*genai.Part{
			{FunctionResponse: &genai.FunctionResponse{ID: "tc1", Response: map[string]any{"result": "ok"}}},
		},
	}

	for i := 7; i < total; i++ {
		contents[i] = &genai.Content{Role: "user", Parts: []*genai.Part{{Text: "msg"}}}
	}

	req := &model.LLMRequest{Contents: contents}

	_, err := PruneHistory(nil, req)
	if err != nil {
		t.Fatalf("PruneHistory failed: %v", err)
	}

	// Verify: no orphan tool responses in the result
	for i, c := range req.Contents {
		if isToolResponse(c) {
			// Must have a preceding model with tool_calls
			found := false
			for j := i - 1; j >= 0; j-- {
				if req.Contents[j].Role == genai.RoleModel && hasToolCalls(req.Contents[j]) {
					found = true
					break
				}
				if req.Contents[j].Role == "user" || req.Contents[j].Role == "" {
					break
				}
			}
			if !found {
				t.Errorf("Orphan tool response found at index %d after pruning", i)
			}
		}
	}

	// Must start with user
	if req.Contents[0].Role != "user" && req.Contents[0].Role != "" {
		t.Errorf("History must start with user, got %s", req.Contents[0].Role)
	}
}

func TestPruneHistory_MultipleToolCallCycles(t *testing.T) {
	// Simulates a screenshot-heavy session with multiple tool_call → tool cycles.
	// Layout (total: 55):
	//   For each cycle i (0..4):
	//     [i*5+0] user
	//     [i*5+1] assistant(tool_calls)
	//     [i*5+2] tool(response)
	//     [i*5+3] assistant(text reply)
	//     [i*5+4] user (follow-up)
	//   [25..54] = 30 more user messages
	//
	// Total = 55. Prune should cut ~15 turns, must not orphan any tool messages.

	total := 55
	contents := make([]*genai.Content, total)

	for cycle := 0; cycle < 5; cycle++ {
		base := cycle * 5
		contents[base] = &genai.Content{Role: "user", Parts: []*genai.Part{{Text: "show screen"}}}
		contents[base+1] = &genai.Content{
			Role: genai.RoleModel,
			Parts: []*genai.Part{
				{FunctionCall: &genai.FunctionCall{ID: "tc" + string(rune('a'+cycle)), Name: "screen_capture"}},
			},
		}
		contents[base+2] = &genai.Content{
			Role: "tool",
			Parts: []*genai.Part{
				{FunctionResponse: &genai.FunctionResponse{ID: "tc" + string(rune('a'+cycle))}},
			},
		}
		contents[base+3] = &genai.Content{
			Role:  genai.RoleModel,
			Parts: []*genai.Part{{Text: "I see your screen"}},
		}
		contents[base+4] = &genai.Content{Role: "user", Parts: []*genai.Part{{Text: "ok thanks"}}}
	}

	for i := 25; i < total; i++ {
		contents[i] = &genai.Content{Role: "user", Parts: []*genai.Part{{Text: "more chat"}}}
	}

	req := &model.LLMRequest{Contents: contents}

	_, err := PruneHistory(nil, req)
	if err != nil {
		t.Fatalf("PruneHistory failed: %v", err)
	}

	// Verify: no orphan tool responses
	for i, c := range req.Contents {
		if isToolResponse(c) {
			found := false
			for j := i - 1; j >= 0; j-- {
				if req.Contents[j].Role == genai.RoleModel && hasToolCalls(req.Contents[j]) {
					found = true
					break
				}
				if req.Contents[j].Role == "user" || req.Contents[j].Role == "" {
					break
				}
			}
			if !found {
				t.Errorf("Orphan tool response at index %d after pruning", i)
			}
		}
	}

	// Must start with user
	if req.Contents[0].Role != "user" && req.Contents[0].Role != "" {
		t.Errorf("History must start with user, got %s", req.Contents[0].Role)
	}

	// Must not exceed max
	if len(req.Contents) > MaxHistoryTurns+5 {
		// Allow slight overshoot from group preservation
		t.Errorf("History too large: %d", len(req.Contents))
	}
}

func TestPruneHistory_RepeatedPruning(t *testing.T) {
	// Simulates the ADK runner calling PruneHistory multiple times per turn.
	// Each call adds a new tool_call → tool pair and prunes again.

	// Start with a history at the limit
	contents := make([]*genai.Content, MaxHistoryTurns)
	for i := 0; i < MaxHistoryTurns; i++ {
		contents[i] = &genai.Content{Role: "user", Parts: []*genai.Part{{Text: "msg"}}}
	}

	req := &model.LLMRequest{Contents: contents}

	// Simulate 5 successive tool executions within one turn
	for round := 0; round < 5; round++ {
		// ADK adds: assistant(tool_call) + tool(response)
		req.Contents = append(req.Contents,
			&genai.Content{
				Role: genai.RoleModel,
				Parts: []*genai.Part{
					{FunctionCall: &genai.FunctionCall{ID: "r" + string(rune('0'+round)), Name: "speak"}},
				},
			},
			&genai.Content{
				Role: "tool",
				Parts: []*genai.Part{
					{FunctionResponse: &genai.FunctionResponse{ID: "r" + string(rune('0'+round))}},
				},
			},
		)

		_, err := PruneHistory(nil, req)
		if err != nil {
			t.Fatalf("PruneHistory round %d failed: %v", round, err)
		}

		// Validate: no orphan tool responses
		for i, c := range req.Contents {
			if isToolResponse(c) {
				found := false
				for j := i - 1; j >= 0; j-- {
					if req.Contents[j].Role == genai.RoleModel && hasToolCalls(req.Contents[j]) {
						found = true
						break
					}
					if req.Contents[j].Role == "user" || req.Contents[j].Role == "" {
						break
					}
				}
				if !found {
					t.Fatalf("Round %d: orphan tool response at index %d", round, i)
				}
			}
		}

		// Must start with user
		if req.Contents[0].Role != "user" && req.Contents[0].Role != "" {
			t.Fatalf("Round %d: history must start with user, got %s", round, req.Contents[0].Role)
		}
	}
}
