package callbacks

import (
	"log"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/model"
	"google.golang.org/genai"
)

const (
	MaxImagesInContext = 1 // Limit to the most recent screenshot
	MaxHistoryTurns    = 40
)

// isToolResponse returns true if the content contains any FunctionResponse part.
func isToolResponse(c *genai.Content) bool {
	for _, p := range c.Parts {
		if p.FunctionResponse != nil {
			return true
		}
	}
	return false
}

// hasToolCalls returns true if the content contains any FunctionCall part
// (i.e., this is an assistant message that invoked tool(s)).
func hasToolCalls(c *genai.Content) bool {
	for _, p := range c.Parts {
		if p.FunctionCall != nil {
			return true
		}
	}
	return false
}

// PruneHistory ensures the LLMRequest stays within token limits by removing old images and turns.
// It preserves the integrity of assistant(tool_calls) → tool(response) pairs
// to avoid OpenAI "messages with role 'tool' must be a response to a preceeding message with 'tool_calls'" errors.
func PruneHistory(ctx agent.CallbackContext, req *model.LLMRequest) (*model.LLMResponse, error) {
	if len(req.Contents) == 0 {
		return nil, nil
	}

	// 1. Prune Images (The biggest token consumers)
	imageCount := 0
	// Iterate backwards to keep the most recent images
	for i := len(req.Contents) - 1; i >= 0; i-- {
		content := req.Contents[i]
		for j, part := range content.Parts {
			if part.InlineData != nil {
				imageCount++
				if imageCount > MaxImagesInContext {
					// Replace old image with a placeholder part
					content.Parts[j] = &genai.Part{
						Text: "[SYSTEM: Previous Screenshot removed from context to save tokens]",
					}
					log.Printf("[Prune] REPLACED old image in history turn index %d (image count: %d)", i, imageCount)
				}
			}
		}
	}

	// 2. Prune History Turns (Tool-Call-Aware Sliding Window)
	if len(req.Contents) > MaxHistoryTurns {
		removeCount := len(req.Contents) - MaxHistoryTurns

		// Phase A: Find a safe cut point.
		// We must land on a content that is a plain user message (no FunctionResponse).
		// We must also ensure we're not cutting between a model(FunctionCall) and its
		// corresponding tool response content.
		for removeCount < len(req.Contents) {
			c := req.Contents[removeCount]

			// A safe starting point is a user/empty-role content with no FunctionResponse parts
			if !isToolResponse(c) && !hasToolCalls(c) &&
				(c.Role == genai.RoleUser || c.Role == "") {
				break
			}
			removeCount++
		}

		// Fallback: keep at least 1 message
		if removeCount >= len(req.Contents) {
			removeCount = len(req.Contents) - 1
		}

		if removeCount > 0 {
			log.Printf("[Prune] Truncating history: removing %d turns. New size: %d", removeCount, len(req.Contents)-removeCount)
			req.Contents = req.Contents[removeCount:]
		}

		// Phase B: Defensive validation — strip any orphaned tool responses
		// that ended up in the retained history without a matching model(tool_calls).
		req.Contents = stripOrphanToolMessages(req.Contents)
	}

	return nil, nil
}

// stripOrphanToolMessages removes any tool response messages that don't have
// a preceding model message with matching tool_calls.
// This is the defensive fallback that guarantees API compatibility.
func stripOrphanToolMessages(contents []*genai.Content) []*genai.Content {
	result := make([]*genai.Content, 0, len(contents))

	for i, c := range contents {
		if isToolResponse(c) {
			// Look backwards for the nearest model message with tool_calls.
			// Skip over other tool responses (which may also have role "user" in ADK/Gemini format).
			found := false
			for j := i - 1; j >= 0; j-- {
				prev := contents[j]

				// Found the parent: model message with FunctionCall parts
				if prev.Role == genai.RoleModel && hasToolCalls(prev) {
					found = true
					break
				}

				// Skip over other tool responses (they might have role "user" or "" in ADK)
				if isToolResponse(prev) {
					continue
				}

				// Hit a model message WITHOUT tool_calls (plain text response) → orphan
				if prev.Role == genai.RoleModel {
					break
				}

				// Hit a plain user message (no FunctionResponse) → orphan
				if (prev.Role == genai.RoleUser || prev.Role == "") && !isToolResponse(prev) {
					break
				}
			}
			if !found {
				log.Printf("[Prune] Stripping orphan tool response at index %d (no preceding tool_calls)", i)
				continue // skip this orphan
			}
		}
		result = append(result, c)
	}

	return result
}
