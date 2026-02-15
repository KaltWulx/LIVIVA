package callbacks

import (
	"fmt"
	"regexp"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/model"
)

// Regex patterns for sensitive data
var (
	// Matches typical API key patterns (OpenAI sk-..., generic long base64 strings)
	apiKeyRegex = regexp.MustCompile(`(sk-[a-zA-Z0-9]{32,})|([a-zA-Z0-9]{40,})`)
	// Matches strict IPv4 (excluding localhost/127.0.0.1 and private ranges if desired, here matching general IPs)
	ipRegex = regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`)
	// Matches email addresses
	emailRegex = regexp.MustCompile(`[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`)
)

// RedactPII is a BeforeModel callback that scans user input for sensitive PII
// and replaces it with [REDACTED].
func RedactPII(ctx agent.CallbackContext, req *model.LLMRequest) (*model.LLMResponse, error) {
	redactedCount := 0

	// Iterate over all content parts in the request
	for _, content := range req.Contents {
		for _, part := range content.Parts {
			originalText := part.Text

			// Apply redactions
			newText := apiKeyRegex.ReplaceAllString(originalText, "[API_KEY_REDACTED]")
			newText = ipRegex.ReplaceAllString(newText, "[IP_REDACTED]")
			newText = emailRegex.ReplaceAllString(newText, "[EMAIL_REDACTED]")

			if newText != originalText {
				part.Text = newText
				redactedCount++
			}
		}
	}

	if redactedCount > 0 {
		fmt.Printf("[Callback-Privacy] Redacted %d sensitive items from request.\n", redactedCount)
	}

	return nil, nil // Continue with sanitized request
}
