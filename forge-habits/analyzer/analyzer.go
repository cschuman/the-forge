package analyzer

import (
	"sort"
	"strings"

	"forge-habits/parser"
)

type Analysis struct {
	TotalCommands    int
	TopCommands      []CommandCount
	AliasCandidates  []CommandCount
	DirectoryStats   []CommandCount
	PipelineCommands []CommandCount
	CommandSequences []SequenceCount
	PossibleTypos    []Typo
}

type CommandCount struct {
	Command string
	Count   int
}

type SequenceCount struct {
	From  string
	To    string
	Count int
}

type Typo struct {
	Typed    string
	Intended string
	Count    int
}

// Common commands for typo detection (as slice for Levenshtein comparison)
var commonCommands = []string{
	"git", "cd", "ls", "npm", "yarn", "pnpm", "clear", "cat", "grep", "find",
	"sudo", "brew", "open", "mkdir", "touch", "rm", "cp", "mv", "code",
	"docker", "kubectl", "go", "python", "python3", "pip", "node", "ssh",
	"curl", "wget", "vim", "nvim", "nano", "echo", "source", "export",
}

// Common commands as a map for O(1) lookup
var commonCommandsMap = make(map[string]bool)

func init() {
	for _, cmd := range commonCommands {
		commonCommandsMap[cmd] = true
	}
}

// Known valid commands that might look like typos
var knownValidCommands = map[string]bool{
	"npx": true, "pip3": true, "pipx": true, "pipenv": true,
	"bat": true, "uv": true, "ln": true, "gh": true, "fx": true,
	"codex": true, "import": true, "-d": true, "rg": true,
	"sh": true, "ps": true, "nvm": true, "env": true, "awk": true,
}

func Analyze(data *parser.HistoryData) *Analysis {
	analysis := &Analysis{
		TotalCommands: len(data.Commands),
	}

	// Count command frequencies
	cmdCounts := make(map[string]int)
	fullCmdCounts := make(map[string]int)
	dirCounts := make(map[string]int)
	pipelineCounts := make(map[string]int)

	for _, cmd := range data.Commands {
		// First word (command name)
		cmdCounts[cmd.Command]++

		// Full command for alias candidates
		if len(cmd.Raw) > 30 {
			fullCmdCounts[cmd.Raw]++
		}

		// Directory navigation
		if cmd.Command == "cd" && len(cmd.Args) > 0 {
			dirCounts[cmd.Args[0]]++
		}

		// Pipeline commands
		if strings.Contains(cmd.Raw, "|") {
			pipelineCounts[cmd.Raw]++
		}
	}

	// Top commands
	analysis.TopCommands = topN(cmdCounts, 20)

	// Alias candidates (long commands used 2+ times)
	aliasCandidates := make(map[string]int)
	for cmd, count := range fullCmdCounts {
		if count >= 2 {
			aliasCandidates[cmd] = count
		}
	}
	analysis.AliasCandidates = topN(aliasCandidates, 15)

	// Directory stats
	analysis.DirectoryStats = topN(dirCounts, 15)

	// Pipeline commands
	pipelines := make(map[string]int)
	for cmd, count := range pipelineCounts {
		if count >= 2 {
			pipelines[cmd] = count
		}
	}
	analysis.PipelineCommands = topN(pipelines, 10)

	// Command sequences
	analysis.CommandSequences = analyzeSequences(data.Commands)

	// Typo detection
	analysis.PossibleTypos = detectTypos(cmdCounts)

	return analysis
}

func topN(counts map[string]int, n int) []CommandCount {
	var result []CommandCount
	for cmd, count := range counts {
		result = append(result, CommandCount{Command: cmd, Count: count})
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Count > result[j].Count
	})

	if len(result) > n {
		result = result[:n]
	}

	return result
}

func analyzeSequences(commands []parser.Command) []SequenceCount {
	sequences := make(map[string]int)

	for i := 0; i < len(commands)-1; i++ {
		from := commands[i].Command
		to := commands[i+1].Command
		if from != "" && to != "" {
			key := from + " → " + to
			sequences[key]++
		}
	}

	var result []SequenceCount
	for seq, count := range sequences {
		if count >= 10 {
			parts := strings.Split(seq, " → ")
			result = append(result, SequenceCount{
				From:  parts[0],
				To:    parts[1],
				Count: count,
			})
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Count > result[j].Count
	})

	if len(result) > 15 {
		result = result[:15]
	}

	return result
}

func detectTypos(cmdCounts map[string]int) []Typo {
	var typos []Typo

	for typed, count := range cmdCounts {
		if count < 2 || len(typed) < 2 {
			continue
		}

		// Skip if it's a known valid command (O(1) lookup)
		if knownValidCommands[typed] {
			continue
		}

		// Skip if it's a valid common command (O(1) lookup instead of O(n))
		if commonCommandsMap[typed] {
			continue
		}

		// Check against common commands for typos
		for _, common := range commonCommands {
			dist := levenshtein(typed, common)
			maxDist := max(1, len(common)/3)

			if dist > 0 && dist <= maxDist && len(typed) >= len(common)-1 && len(typed) <= len(common)+1 {
				typos = append(typos, Typo{
					Typed:    typed,
					Intended: common,
					Count:    count,
				})
				break
			}
		}
	}

	sort.Slice(typos, func(i, j int) bool {
		return typos[i].Count > typos[j].Count
	})

	if len(typos) > 10 {
		typos = typos[:10]
	}

	return typos
}

// Optimized Levenshtein distance implementation
// Uses O(min(n,m)) space instead of O(n*m) by only keeping two rows
func levenshtein(a, b string) int {
	if len(a) == 0 {
		return len(b)
	}
	if len(b) == 0 {
		return len(a)
	}

	// Make 'a' the shorter string to minimize space usage
	if len(a) > len(b) {
		a, b = b, a
	}

	// Only need two rows: previous and current
	prev := make([]int, len(a)+1)
	curr := make([]int, len(a)+1)

	// Initialize first row
	for i := range prev {
		prev[i] = i
	}

	// Fill in the rest
	for j := 1; j <= len(b); j++ {
		curr[0] = j
		for i := 1; i <= len(a); i++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			curr[i] = min(
				prev[i]+1,          // deletion
				min(curr[i-1]+1,    // insertion
					prev[i-1]+cost), // substitution
			)
		}
		// Swap rows
		prev, curr = curr, prev
	}

	return prev[len(a)]
}
