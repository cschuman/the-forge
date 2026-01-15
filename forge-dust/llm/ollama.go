package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"forge-dust/analyzer"
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

func formatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func buildPrompt(analysis *analyzer.Analysis) string {
	var sb strings.Builder

	sb.WriteString(`You are a disk cleanup expert. Analyze this user's disk usage and provide safe, actionable cleanup recommendations.

## Disk Analysis Results

`)

	sb.WriteString(fmt.Sprintf("**Total scanned:** %s across %d files\n",
		formatSize(analysis.ScanStats.TotalSize),
		analysis.ScanStats.TotalFiles))
	sb.WriteString(fmt.Sprintf("**Potential space to reclaim:** %s\n\n",
		formatSize(analysis.TotalReclaimable)))

	// Cache directories
	if len(analysis.CacheDirs) > 0 {
		sb.WriteString("### Cache Directories Found\n")
		for _, cache := range analysis.CacheDirs {
			sb.WriteString(fmt.Sprintf("- `%s` (%s) - %s\n", cache.Path, formatSize(cache.Size), cache.Description))
		}
		sb.WriteString("\n")
	}

	// Large files
	if len(analysis.LargeFiles) > 0 {
		sb.WriteString("### Large Files (>100MB)\n")
		for i, f := range analysis.LargeFiles {
			if i >= 10 {
				break
			}
			sb.WriteString(fmt.Sprintf("- `%s` (%s)\n", f.Path, formatSize(f.Size)))
		}
		sb.WriteString("\n")
	}

	// Old files
	if len(analysis.OldFiles) > 0 {
		sb.WriteString("### Old Files (>1 year, >10MB)\n")
		for i, f := range analysis.OldFiles {
			if i >= 8 {
				break
			}
			sb.WriteString(fmt.Sprintf("- `%s` (%s, %.0f days old)\n",
				f.Path, formatSize(f.Size), f.Age.Hours()/24))
		}
		sb.WriteString("\n")
	}

	// Downloads
	if len(analysis.Downloads) > 0 {
		sb.WriteString("### Large Downloads\n")
		for i, f := range analysis.Downloads {
			if i >= 8 {
				break
			}
			sb.WriteString(fmt.Sprintf("- `%s` (%s)\n", f.Path, formatSize(f.Size)))
		}
		sb.WriteString("\n")
	}

	sb.WriteString(`
## Your Task

Based on this analysis, provide:

1. **Safe Cleanup Commands** - Shell commands to clean up the safest items (caches that can be rebuilt). Include commands the user can copy-paste.

2. **Review Suggestions** - Files the user should manually review before deleting. Be specific about why each might be safe to delete.

3. **Maintenance Tips** - 2-3 tips to prevent disk clutter in the future.

Be conservative - only recommend deleting things that are clearly safe. When in doubt, suggest reviewing rather than deleting.
`)

	return sb.String()
}
