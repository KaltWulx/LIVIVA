package callbacks

import (
	"fmt"
	"log"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/model"
	"google.golang.org/genai"
)

// AutoLoadScreenCapture checks if a screenshot was just taken and automatically loads it
// into the context for the model to see.
func AutoLoadScreenCapture(ctx agent.CallbackContext, req *model.LLMRequest) (*model.LLMResponse, error) {
	// 1. Check if 'latest_screenshot' is set in session state
	val, err := ctx.State().Get("latest_screenshot")
	if err != nil || val == nil {
		return nil, nil
	}

	filename, ok := val.(string)
	if !ok || filename == "" {
		return nil, nil
	}

	// 2. Clear the state immediately to prevent loops
	_ = ctx.State().Set("latest_screenshot", nil)

	log.Printf("[Vision] TURN START: Detected pending screenshot '%s'", filename)

	// 3. Load the artifact
	loadResp, err := ctx.Artifacts().Load(ctx, filename)
	if err != nil {
		log.Printf("[Vision] ERROR: Failed to load artifact '%s': %v", filename, err)
		return nil, nil
	}

	if loadResp == nil || loadResp.Part == nil {
		log.Printf("[Vision] ERROR: Loaded artifact '%s' is empty", filename)
		return nil, nil
	}

	// 4. Inject into request
	// We MUST inject into a "user" role content for OpenAI multimodal support.
	// If the last content in the history is not "user", we append a new turn.

	injectionText := fmt.Sprintf("\n[SYSTEM: Vision Context] Successfully captured screen. You are now seeing the file '%s'. Analyze this image to answer or act.\n", filename)
	imagePart := loadResp.Part
	textPart := genai.NewPartFromText(injectionText)

	var targetContent *genai.Content

	if len(req.Contents) > 0 {
		last := req.Contents[len(req.Contents)-1]
		if last.Role == "user" {
			targetContent = last
			log.Printf("[Vision] Injecting into existing LATEST user message (turn %d)", len(req.Contents)-1)
		}
	}

	if targetContent == nil {
		// Append a new User turn
		targetContent = &genai.Content{
			Role:  "user",
			Parts: []*genai.Part{},
		}
		req.Contents = append(req.Contents, targetContent)
		log.Printf("[Vision] Appending NEW user message for vision context (turn %d)", len(req.Contents)-1)
	}

	targetContent.Parts = append(targetContent.Parts, textPart, imagePart)

	log.Printf("[Vision] SUCCESS: Screen context '%s' injected into LLM request.", filename)
	return nil, nil
}
