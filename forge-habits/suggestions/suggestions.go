package suggestions

import (
	"encoding/json"
	"fmt"
	"strings"

	"forge-habits/analyzer"
	"forge-habits/llm"
)

// Confidence levels
type Confidence string

const (
	ConfHigh   Confidence = "high"   // Clear win, should auto-add
	ConfMedium Confidence = "medium" // Good suggestion, ask user
	ConfLow    Confidence = "low"    // Show for reference
)

// SuggestionType categorizes what kind of suggestion this is
type SuggestionType string

const (
	TypeAlias    SuggestionType = "alias"
	TypeFunction SuggestionType = "function"
	TypeTip      SuggestionType = "tip"
)

// Suggestion represents an actionable improvement
type Suggestion struct {
	Type        SuggestionType
	Name        string // alias/function name
	Usage       string // how to use it (e.g., "killport 8080")
	Command     string // original command pattern
	Code        string // the alias/function code to add
	Description string // human-readable explanation
	Impact      int    // usage count - how many times this was typed
	Confidence  Confidence
}

// SuggestionSet groups suggestions by confidence
type SuggestionSet struct {
	HighImpact []Suggestion // Auto-add candidates
	Review     []Suggestion // Need user review
	Tips       []Suggestion // Just informational
}

// Generate creates actionable suggestions from analysis using LLM
func Generate(analysis *analyzer.Analysis, client *llm.Client) *SuggestionSet {
	set := &SuggestionSet{}

	// Collect patterns worth analyzing
	var patterns []PatternInput

	// Long commands used repeatedly
	for _, ac := range analysis.AliasCandidates {
		if ac.Count >= 5 {
			patterns = append(patterns, PatternInput{
				Command: ac.Command,
				Count:   ac.Count,
				Type:    "repeated_command",
			})
		}
	}

	// Pipeline commands
	for _, pc := range analysis.PipelineCommands {
		if pc.Count >= 3 {
			patterns = append(patterns, PatternInput{
				Command: pc.Command,
				Count:   pc.Count,
				Type:    "pipeline",
			})
		}
	}

	// Command sequences
	for _, seq := range analysis.CommandSequences {
		if seq.Count >= 30 {
			patterns = append(patterns, PatternInput{
				Command: fmt.Sprintf("%s → %s", seq.From, seq.To),
				Count:   seq.Count,
				Type:    "sequence",
			})
		}
	}

	if len(patterns) == 0 {
		return set
	}

	// Ask LLM to analyze patterns
	suggestions := analyzePatternsWithLLM(patterns, client)

	// Categorize by confidence
	seen := make(map[string]bool)
	for _, s := range suggestions {
		if seen[s.Name] {
			continue
		}
		seen[s.Name] = true

		if s.Confidence == ConfHigh {
			set.HighImpact = append(set.HighImpact, s)
		} else if s.Confidence == ConfMedium {
			set.Review = append(set.Review, s)
		}
	}

	// Add tips
	set.Tips = generateTips(analysis)

	return set
}

// GenerateWithoutLLM creates suggestions using heuristics only
func GenerateWithoutLLM(analysis *analyzer.Analysis) *SuggestionSet {
	set := &SuggestionSet{}
	seen := make(map[string]bool)

	addSuggestion := func(s *Suggestion) {
		if s == nil || seen[s.Name] {
			return
		}
		seen[s.Name] = true

		if s.Confidence == ConfHigh {
			set.HighImpact = append(set.HighImpact, *s)
		} else {
			set.Review = append(set.Review, *s)
		}
	}

	// Simple heuristics for common patterns
	for _, pc := range analysis.PipelineCommands {
		if pc.Count < 5 {
			continue
		}
		s := createSimpleSuggestion(pc.Command, pc.Count)
		addSuggestion(s)
	}

	for _, ac := range analysis.AliasCandidates {
		if ac.Count < 5 {
			continue
		}
		s := createSimpleSuggestion(ac.Command, ac.Count)
		addSuggestion(s)
	}

	set.Tips = generateTips(analysis)
	return set
}

type PatternInput struct {
	Command string `json:"command"`
	Count   int    `json:"count"`
	Type    string `json:"type"`
}

type LLMSuggestion struct {
	Name        string `json:"name"`
	Type        string `json:"type"` // "alias" or "function"
	Usage       string `json:"usage"`
	Code        string `json:"code"`
	Description string `json:"description"`
	Confidence  string `json:"confidence"` // "high", "medium", "low"
	Pattern     string `json:"pattern"`    // which input pattern this addresses
}

func analyzePatternsWithLLM(patterns []PatternInput, client *llm.Client) []Suggestion {
	prompt := buildAnalysisPrompt(patterns)

	response, err := client.Generate(prompt)
	if err != nil {
		// Fall back to simple heuristics
		return nil
	}

	return parseLLMResponse(response, patterns)
}

func buildAnalysisPrompt(patterns []PatternInput) string {
	var sb strings.Builder

	sb.WriteString(`You are analyzing shell command patterns to suggest aliases and functions.

PATTERNS FOUND:
`)

	for _, p := range patterns {
		sb.WriteString(fmt.Sprintf("- %q (used %d times, type: %s)\n", p.Command, p.Count, p.Type))
	}

	sb.WriteString(`
YOUR TASK:
For each pattern that would benefit from an alias or function, provide a suggestion.

RULES:
1. If a command has variable parts (like port numbers, file paths), make it a FUNCTION with parameters
2. If a command is always the same, make it an ALIAS
3. Choose short, memorable names (2-8 chars)
4. For functions, show usage with example arguments
5. Confidence: "high" if used 20+ times, "medium" if 10+, "low" otherwise
6. Consolidate similar patterns (e.g., all "lsof -ti:XXXX | xargs kill" become one function)
7. Skip patterns that are already short or wouldn't benefit much

OUTPUT FORMAT (JSON array):
[
  {
    "name": "kp",
    "type": "function",
    "usage": "kp 8080",
    "code": "kp() {\n  lsof -ti:\"$1\" | xargs kill -9\n}",
    "description": "Kill process on port",
    "confidence": "high",
    "pattern": "lsof -ti:8080 | xargs kill -9"
  },
  {
    "name": "gob",
    "type": "alias",
    "usage": "gob",
    "code": "alias gob='go build -o api . && ./api --server'",
    "description": "Build and run Go API",
    "confidence": "high",
    "pattern": "go build -o api . && ./api --server"
  }
]

Only output the JSON array, nothing else.
`)

	return sb.String()
}

func parseLLMResponse(response string, patterns []PatternInput) []Suggestion {
	// Find JSON array in response
	start := strings.Index(response, "[")
	end := strings.LastIndex(response, "]")

	if start == -1 || end == -1 || end <= start {
		return nil
	}

	jsonStr := response[start : end+1]

	var llmSuggestions []LLMSuggestion
	if err := json.Unmarshal([]byte(jsonStr), &llmSuggestions); err != nil {
		return nil
	}

	// Convert to our Suggestion type
	var suggestions []Suggestion
	patternCounts := make(map[string]int)
	for _, p := range patterns {
		patternCounts[p.Command] = p.Count
	}

	for _, ls := range llmSuggestions {
		sugType := TypeAlias
		if ls.Type == "function" {
			sugType = TypeFunction
		}

		conf := ConfMedium
		switch ls.Confidence {
		case "high":
			conf = ConfHigh
		case "low":
			conf = ConfLow
		}

		impact := patternCounts[ls.Pattern]
		if impact == 0 {
			// Try to find a matching pattern
			for cmd, count := range patternCounts {
				if strings.Contains(cmd, ls.Pattern) || strings.Contains(ls.Pattern, cmd) {
					impact = count
					break
				}
			}
		}

		suggestions = append(suggestions, Suggestion{
			Type:        sugType,
			Name:        ls.Name,
			Usage:       ls.Usage,
			Command:     ls.Pattern,
			Code:        ls.Code,
			Description: ls.Description,
			Impact:      impact,
			Confidence:  conf,
		})
	}

	return suggestions
}

func createSimpleSuggestion(cmd string, count int) *Suggestion {
	// Very basic heuristic fallback
	name := generateSimpleName(cmd)
	if name == "" {
		return nil
	}

	escaped := strings.ReplaceAll(cmd, "'", "'\\''")

	conf := ConfLow
	if count >= 20 {
		conf = ConfHigh
	} else if count >= 10 {
		conf = ConfMedium
	}

	return &Suggestion{
		Type:        TypeAlias,
		Name:        name,
		Usage:       name,
		Command:     cmd,
		Code:        fmt.Sprintf("alias %s='%s'", name, escaped),
		Description: fmt.Sprintf("Used %d times", count),
		Impact:      count,
		Confidence:  conf,
	}
}

func generateSimpleName(cmd string) string {
	// Remove pipe and redirect operators for cleaner parsing
	clean := cmd
	clean = strings.ReplaceAll(clean, "|", " ")
	clean = strings.ReplaceAll(clean, ">", " ")
	clean = strings.ReplaceAll(clean, "<", " ")
	clean = strings.ReplaceAll(clean, "&", " ")
	clean = strings.ReplaceAll(clean, ";", " ")

	words := strings.Fields(clean)
	if len(words) == 0 {
		return ""
	}

	// Take first letter of first 2-3 significant words (commands, not flags/paths)
	name := ""
	for _, w := range words {
		if len(w) == 0 {
			continue
		}
		// Skip flags, paths, numbers, and special chars
		if w[0] == '-' || w[0] == '.' || w[0] == '/' || w[0] == '$' ||
			(w[0] >= '0' && w[0] <= '9') {
			continue
		}
		// Skip common shell words
		if w == "xargs" || w == "grep" || w == "awk" || w == "sed" {
			continue
		}
		name += string(w[0])
		if len(name) >= 3 {
			break
		}
	}

	if len(name) < 2 {
		return ""
	}

	return strings.ToLower(name)
}

func generateTips(analysis *analyzer.Analysis) []Suggestion {
	var tips []Suggestion

	// Check for clear command overuse
	for _, tc := range analysis.TopCommands {
		if tc.Command == "clear" && tc.Count > 100 {
			tips = append(tips, Suggestion{
				Type:        TypeTip,
				Name:        "Ctrl+L",
				Description: fmt.Sprintf("You typed 'clear' %d times. Use Ctrl+L instead - it's built into your terminal.", tc.Count),
				Confidence:  ConfHigh,
			})
			break
		}
	}

	return tips
}
