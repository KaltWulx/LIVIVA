package llm

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	// GitHub Copilot CLI Client ID (VSCode) - Corrected based on Python script
	GitHubClientID = "Iv1.b507a08c87ecfe98"
	GitHubScope    = "read:user"
)

type DeviceCodeResponse struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

type TokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	Scope       string `json:"scope"`
	Error       string `json:"error"`
}

type CopilotTokenResponse struct {
	Token     string `json:"token"`
	ExpiresAt int64  `json:"expires_at"`
}

type CachedToken struct {
	GitHubToken  string `json:"github_token"`
	CopilotToken string `json:"copilot_token"`
	ExpiresAt    int64  `json:"expires_at"`
}

// GetCopilotToken orchestrates the token obtaining process.
// It checks cache first, then refreshes if expired, or triggers Device Flow.
func GetCopilotToken() (string, error) {
	home, _ := os.UserHomeDir()
	configDir := filepath.Join(home, ".liviva")
	_ = os.MkdirAll(configDir, 0755)
	cachePath := filepath.Join(configDir, "token.json")

	var cached CachedToken
	if data, err := os.ReadFile(cachePath); err == nil {
		if err := json.Unmarshal(data, &cached); err == nil {
			// Check if Copilot token is still valid (with 5 min buffer)
			if cached.CopilotToken != "" && cached.ExpiresAt > time.Now().Unix()+300 {
				return cached.CopilotToken, nil
			}

			// If we have a GitHub token but Copilot token is expired, refresh it
			if cached.GitHubToken != "" {
				log.Println("[Auth] Refreshing Copilot session token...")
				copilot, err := ExchangeGitHubTokenForCopilot(cached.GitHubToken)
				if err == nil {
					cached.CopilotToken = copilot.Token
					cached.ExpiresAt = copilot.ExpiresAt
					saveToken(cachePath, cached)
					return cached.CopilotToken, nil
				}
				log.Printf("[Auth] Failed to refresh: %v. Re-authorizing...\n", err)
			}
		}
	}

	// If no token or refresh failed, do Device Flow
	ghToken, err := DeviceFlowLogin()
	if err != nil {
		return "", err
	}

	copilot, err := ExchangeGitHubTokenForCopilot(ghToken)
	if err != nil {
		return "", err
	}

	cached = CachedToken{
		GitHubToken:  ghToken,
		CopilotToken: copilot.Token,
		ExpiresAt:    copilot.ExpiresAt,
	}
	saveToken(cachePath, cached)

	return cached.CopilotToken, nil
}

// DeviceFlowLogin starts the GitHub Device Flow.
func DeviceFlowLogin() (string, error) {
	log.Println("[Auth] Initiating GitHub Device Flow...")

	data := url.Values{}
	data.Set("client_id", GitHubClientID)
	data.Set("scope", GitHubScope)

	req, _ := http.NewRequest("POST", "https://github.com/login/device/code", strings.NewReader(data.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("failed to get device code (status %d): %s", resp.StatusCode, string(body))
	}

	var dcr DeviceCodeResponse
	if err := json.NewDecoder(resp.Body).Decode(&dcr); err != nil {
		return "", fmt.Errorf("failed to decode device code response: %w", err)
	}

	if dcr.UserCode == "" {
		return "", fmt.Errorf("github returned an empty user code")
	}

	log.Printf("\n! ACTION REQUIRED !\n")
	log.Printf("Visit: %s\n", dcr.VerificationURI)
	log.Printf("Enter code: %s\n\n", dcr.UserCode)

	// Polling
	interval := time.Duration(dcr.Interval) * time.Second
	if interval == 0 {
		interval = 6 * time.Second // Python uses 5.5s
	}

	for {
		time.Sleep(interval)

		pollData := url.Values{}
		pollData.Set("client_id", GitHubClientID)
		pollData.Set("device_code", dcr.DeviceCode)
		pollData.Set("grant_type", "urn:ietf:params:oauth:grant-type:device_code")

		pollReq, _ := http.NewRequest("POST", "https://github.com/login/oauth/access_token", strings.NewReader(pollData.Encode()))
		pollReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		pollReq.Header.Set("Accept", "application/json")

		pollResp, err := http.DefaultClient.Do(pollReq)
		if err != nil {
			return "", err
		}

		var tr TokenResponse
		json.NewDecoder(pollResp.Body).Decode(&tr)
		pollResp.Body.Close()

		if tr.AccessToken != "" {
			log.Println("[Auth] Successfully authorized!")
			return tr.AccessToken, nil
		}

		switch tr.Error {
		case "authorization_pending":
			continue
		case "slow_down":
			interval += 5 * time.Second
		case "expired_token":
			return "", fmt.Errorf("session expired, please try again")
		case "access_denied":
			return "", fmt.Errorf("access denied by user")
		default:
			if tr.Error != "" {
				return "", fmt.Errorf("oauth error: %s", tr.Error)
			}
		}
	}
}

// ExchangeGitHubTokenForCopilot exchanges a standard GH token for a Copilot session token.
func ExchangeGitHubTokenForCopilot(ghToken string) (*CopilotTokenResponse, error) {
	req, err := http.NewRequest("GET", "https://api.github.com/copilot_internal/v2/token", nil)
	if err != nil {
		return nil, err
	}
	// Python uses "token {access_token}" format, let's match it
	req.Header.Set("Authorization", "token "+ghToken)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Editor-Version", "vscode/1.85.1")
	req.Header.Set("Editor-Plugin-Version", "copilot/1.143.0")
	req.Header.Set("User-Agent", "GithubCopilot/1.143.0")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to exchange token (status %d): %s", resp.StatusCode, string(body))
	}

	var ctr CopilotTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&ctr); err != nil {
		return nil, err
	}

	return &ctr, nil
}

func saveToken(path string, token CachedToken) {
	data, _ := json.MarshalIndent(token, "", "  ")
	_ = os.WriteFile(path, data, 0600)
}
