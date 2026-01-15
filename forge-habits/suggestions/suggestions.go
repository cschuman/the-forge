package suggestions

import (
	"fmt"
	"path/filepath"
	"strings"

	"forge-habits/analyzer"
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
	Command     string // original command
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

// Generate creates actionable suggestions from analysis
func Generate(analysis *analyzer.Analysis) *SuggestionSet {
	set := &SuggestionSet{}
	seen := make(map[string]bool) // Track by name to avoid duplicates

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

	// Process pipeline commands first (these often become functions)
	// Aggregate counts for similar patterns
	killportCount := 0
	for _, pc := range analysis.PipelineCommands {
		if strings.Contains(pc.Command, "lsof -ti:") && strings.Contains(pc.Command, "xargs kill") {
			killportCount += pc.Count
		}
	}
	if killportCount >= 5 {
		addSuggestion(createKillportFunction(killportCount))
	}

	// Process other pipelines
	for _, pc := range analysis.PipelineCommands {
		if pc.Count < 5 {
			continue
		}
		// Skip killport patterns - already handled
		if strings.Contains(pc.Command, "lsof -ti:") && strings.Contains(pc.Command, "xargs kill") {
			continue
		}

		addSuggestion(createFunctionSuggestion(pc.Command, pc.Count))
	}

	// Process alias candidates
	for _, ac := range analysis.AliasCandidates {
		if ac.Count < 3 {
			continue
		}

		addSuggestion(createAliasSuggestion(ac.Command, ac.Count))
	}

	// Process command sequences
	for _, seq := range analysis.CommandSequences {
		if seq.Count < 50 {
			continue
		}

		addSuggestion(createSequenceSuggestion(seq))
	}

	// Add tips
	set.Tips = generateTips(analysis)

	return set
}

func createKillportFunction(totalCount int) *Suggestion {
	return &Suggestion{
		Type:    TypeFunction,
		Name:    "killport",
		Command: "lsof -ti:<port> | xargs kill -9",
		Code: `killport() {
    if [ -z "$1" ]; then
        echo "Usage: killport <port>"
        return 1
    fi
    lsof -ti:"$1" | xargs kill -9 2>/dev/null && echo "Killed process on port $1" || echo "No process on port $1"
}`,
		Description: fmt.Sprintf("Kill process on any port - pattern used %d times", totalCount),
		Impact:      totalCount,
		Confidence:  ConfHigh,
	}
}

func createAliasSuggestion(cmd string, count int) *Suggestion {
	name := suggestAliasName(cmd)
	if name == "" {
		return nil
	}

	// Escape single quotes in command
	escaped := strings.ReplaceAll(cmd, "'", "'\\''")

	conf := ConfMedium
	if count >= 20 {
		conf = ConfHigh
	} else if count < 5 {
		conf = ConfLow
	}

	return &Suggestion{
		Type:        TypeAlias,
		Name:        name,
		Command:     cmd,
		Code:        fmt.Sprintf("alias %s='%s'", name, escaped),
		Description: fmt.Sprintf("Typed %d times", count),
		Impact:      count,
		Confidence:  conf,
	}
}

func createFunctionSuggestion(cmd string, count int) *Suggestion {
	// Detect pwd | pbcopy pattern
	if strings.TrimSpace(cmd) == "pwd | pbcopy" {
		return &Suggestion{
			Type:        TypeAlias,
			Name:        "cpwd",
			Command:     cmd,
			Code:        "alias cpwd='pwd | pbcopy'",
			Description: fmt.Sprintf("Copy current path to clipboard - used %d times", count),
			Impact:      count,
			Confidence:  ConfHigh,
		}
	}

	// Generic pipeline - lower confidence
	name := suggestFunctionName(cmd)
	if name == "" {
		return nil
	}

	conf := ConfLow
	if count >= 10 {
		conf = ConfMedium
	}

	escaped := strings.ReplaceAll(cmd, "'", "'\\''")
	return &Suggestion{
		Type:        TypeAlias,
		Name:        name,
		Command:     cmd,
		Code:        fmt.Sprintf("alias %s='%s'", name, escaped),
		Description: fmt.Sprintf("Pipeline used %d times", count),
		Impact:      count,
		Confidence:  conf,
	}
}

func createSequenceSuggestion(seq analyzer.SequenceCount) *Suggestion {
	// cd -> ls/l pattern
	if seq.From == "cd" && (seq.To == "l" || seq.To == "ls") {
		return &Suggestion{
			Type:    TypeFunction,
			Name:    "cl",
			Command: fmt.Sprintf("%s → %s", seq.From, seq.To),
			Code: `cl() {
    cd "$@" && l
}`,
			Description: fmt.Sprintf("cd then list - done %d times", seq.Count),
			Impact:      seq.Count,
			Confidence:  ConfHigh,
		}
	}

	return nil
}

func suggestAliasName(cmd string) string {
	cmd = strings.TrimSpace(cmd)

	// go build patterns
	if strings.HasPrefix(cmd, "go build") && strings.Contains(cmd, "./") {
		return "gobuild"
	}

	// App launchers
	if strings.Contains(cmd, ".app/Contents/MacOS/") {
		// Extract app name
		parts := strings.Split(cmd, "/")
		for _, p := range parts {
			if strings.HasSuffix(p, ".app") {
				name := strings.TrimSuffix(p, ".app")
				return strings.ToLower(name[:min(6, len(name))])
			}
		}
	}

	// pip install -r requirements.txt
	if strings.Contains(cmd, "pip install -r requirements") {
		return "pipreq"
	}

	// open ./build/*.app
	if strings.HasPrefix(cmd, "open ") && strings.Contains(cmd, ".app") {
		parts := strings.Split(cmd, "/")
		for _, p := range parts {
			if strings.HasSuffix(p, ".app") {
				name := strings.TrimSuffix(p, ".app")
				return "open" + strings.ToLower(name[:min(4, len(name))])
			}
		}
	}

	// ssh commands
	if strings.HasPrefix(cmd, "ssh ") {
		parts := strings.Fields(cmd)
		if len(parts) >= 2 {
			// Extract hostname
			host := parts[1]
			if strings.Contains(host, "@") {
				host = strings.Split(host, "@")[1]
			}
			host = strings.Split(host, ".")[0]
			if len(host) > 8 {
				host = host[:8]
			}
			return "ssh" + host
		}
	}

	// Generic: use first few chars of significant words
	words := strings.Fields(cmd)
	if len(words) >= 2 {
		name := ""
		for _, w := range words[:min(3, len(words))] {
			w = strings.TrimPrefix(w, "./")
			w = strings.TrimPrefix(w, "-")
			w = filepath.Base(w)
			if len(w) > 0 && w[0] != '-' {
				name += string(w[0])
			}
		}
		if len(name) >= 2 {
			return strings.ToLower(name)
		}
	}

	return ""
}

func suggestFunctionName(cmd string) string {
	// Very simple - just return empty for now, let user name it
	return ""
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
