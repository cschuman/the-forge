package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"forge-habits/analyzer"
)

type OllamaClient struct {
	BaseURL string
	Model   string
	Timeout time.Duration
}

type generateRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
}

type generateResponse struct {
	Response string `json:"response"`
	Done     bool   `json:"done"`
}

func NewClient(model string) *OllamaClient {
	return &OllamaClient{
		BaseURL: "http://localhost:11434",
		Model:   model,
		Timeout: 120 * time.Second,
	}
}

func (c *OllamaClient) GetRecommendations(analysis *analyzer.Analysis) (string, error) {
	prompt := buildPrompt(analysis)

	reqBody := generateRequest{
		Model:  c.Model,
		Prompt: prompt,
		Stream: false,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	client := &http.Client{Timeout: c.Timeout}
	resp, err := client.Post(c.BaseURL+"/api/generate", "application/json", bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", fmt.Errorf("failed to call Ollama: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("Ollama returned status %d: %s", resp.StatusCode, string(body))
	}

	var result generateResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	return result.Response, nil
}

func buildPrompt(analysis *analyzer.Analysis) string {
	var sb strings.Builder

	sb.WriteString(`You are a shell efficiency expert. Analyze this user's shell history and provide actionable recommendations.

## User's Shell History Analysis

`)

	sb.WriteString(fmt.Sprintf("**Total commands:** %d\n\n", analysis.TotalCommands))

	// Top commands
	sb.WriteString("### Most Used Commands\n")
	for i, cmd := range analysis.TopCommands {
		if i >= 10 {
			break
		}
		sb.WriteString(fmt.Sprintf("- `%s`: %d times\n", cmd.Command, cmd.Count))
	}

	// Alias candidates
	if len(analysis.AliasCandidates) > 0 {
		sb.WriteString("\n### Long Commands (Alias Candidates)\n")
		for i, cmd := range analysis.AliasCandidates {
			if i >= 8 {
				break
			}
			display := cmd.Command
			if len(display) > 60 {
				display = display[:60] + "..."
			}
			sb.WriteString(fmt.Sprintf("- `%s`: %d times\n", display, cmd.Count))
		}
	}

	// Pipeline commands
	if len(analysis.PipelineCommands) > 0 {
		sb.WriteString("\n### Repeated Pipelines (Script Candidates)\n")
		for i, cmd := range analysis.PipelineCommands {
			if i >= 5 {
				break
			}
			display := cmd.Command
			if len(display) > 60 {
				display = display[:60] + "..."
			}
			sb.WriteString(fmt.Sprintf("- `%s`: %d times\n", display, cmd.Count))
		}
	}

	// Command sequences
	if len(analysis.CommandSequences) > 0 {
		sb.WriteString("\n### Common Command Sequences\n")
		for i, seq := range analysis.CommandSequences {
			if i >= 8 {
				break
			}
			sb.WriteString(fmt.Sprintf("- `%s` → `%s`: %d times\n", seq.From, seq.To, seq.Count))
		}
	}

	// Typos
	if len(analysis.PossibleTypos) > 0 {
		sb.WriteString("\n### Possible Typos\n")
		for _, typo := range analysis.PossibleTypos {
			sb.WriteString(fmt.Sprintf("- `%s` (probably meant `%s`): %d times\n", typo.Typed, typo.Intended, typo.Count))
		}
	}

	sb.WriteString(`

## Your Task

Based on this analysis, provide:

1. **Top 5 Alias Recommendations** - Suggest short, memorable alias names for the most impactful commands. Format as ready-to-use shell aliases.

2. **Shell Functions** - For complex pipelines or sequences, provide ready-to-use shell functions.

3. **Workflow Tips** - 2-3 specific tips based on their command patterns.

Keep recommendations practical and immediately usable. Output should be copy-paste ready for a .zshrc file.
`)

	return sb.String()
}
