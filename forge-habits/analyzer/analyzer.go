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

// Common commands for typo detection
var commonCommands = []string{
	"git", "cd", "ls", "npm", "yarn", "pnpm", "clear", "cat", "grep", "find",
	"sudo", "brew", "open", "mkdir", "touch", "rm", "cp", "mv", "code",
	"docker", "kubectl", "go", "python", "python3", "pip", "node", "ssh",
	"curl", "wget", "vim", "nvim", "nano", "echo", "source", "export",
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

		// Skip if it's a known valid command
		if knownValidCommands[typed] {
			continue
		}

		// Skip if it's a valid common command
		isCommon := false
		for _, common := range commonCommands {
			if typed == common {
				isCommon = true
				break
			}
		}
		if isCommon {
			continue
		}

		// Check against common commands
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

// Simple Levenshtein distance implementation
func levenshtein(a, b string) int {
	if len(a) == 0 {
		return len(b)
	}
	if len(b) == 0 {
		return len(a)
	}

	matrix := make([][]int, len(a)+1)
	for i := range matrix {
		matrix[i] = make([]int, len(b)+1)
		matrix[i][0] = i
	}
	for j := range matrix[0] {
		matrix[0][j] = j
	}

	for i := 1; i <= len(a); i++ {
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			matrix[i][j] = min(
				matrix[i-1][j]+1,
				min(matrix[i][j-1]+1, matrix[i-1][j-1]+cost),
			)
		}
	}

	return matrix[len(a)][len(b)]
}
