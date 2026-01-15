package output

import (
	"fmt"
	"strings"

	"forge-habits/analyzer"
)

// ANSI color codes
const (
	Reset   = "\033[0m"
	Bold    = "\033[1m"
	Dim     = "\033[2m"
	Cyan    = "\033[36m"
	Green   = "\033[32m"
	Yellow  = "\033[33m"
	Magenta = "\033[35m"
	Blue    = "\033[34m"
)

func PrintAnalysis(analysis *analyzer.Analysis) {
	printHeader("FORGE-HABITS", "Shell History Analysis")
	fmt.Printf("\n%sTotal commands analyzed:%s %d\n", Dim, Reset, analysis.TotalCommands)

	// Top Commands
	printSection("TOP COMMANDS")
	for i, cmd := range analysis.TopCommands {
		if i >= 15 {
			break
		}
		bar := strings.Repeat("█", min(cmd.Count/20+1, 30))
		fmt.Printf("  %s%-12s%s %s%4d%s %s%s%s\n",
			Cyan, cmd.Command, Reset,
			Bold, cmd.Count, Reset,
			Dim, bar, Reset)
	}

	// Alias Candidates
	if len(analysis.AliasCandidates) > 0 {
		printSection("ALIAS CANDIDATES")
		fmt.Printf("  %sLong commands you type repeatedly:%s\n\n", Dim, Reset)
		for i, cmd := range analysis.AliasCandidates {
			if i >= 10 {
				break
			}
			display := cmd.Command
			if len(display) > 65 {
				display = display[:65] + "..."
			}
			fmt.Printf("  %s%dx%s  %s%s%s\n", Yellow, cmd.Count, Reset, Dim, display, Reset)
		}
	}

	// Pipeline Commands
	if len(analysis.PipelineCommands) > 0 {
		printSection("SCRIPT CANDIDATES")
		fmt.Printf("  %sPipelines you run repeatedly (consider making these scripts):%s\n\n", Dim, Reset)
		for i, cmd := range analysis.PipelineCommands {
			if i >= 8 {
				break
			}
			display := cmd.Command
			if len(display) > 65 {
				display = display[:65] + "..."
			}
			fmt.Printf("  %s%dx%s  %s%s%s\n", Green, cmd.Count, Reset, Dim, display, Reset)
		}
	}

	// Directory Stats
	if len(analysis.DirectoryStats) > 0 {
		printSection("MOST VISITED DIRECTORIES")
		for i, dir := range analysis.DirectoryStats {
			if i >= 10 {
				break
			}
			fmt.Printf("  %s%4d%s  %s%s%s\n", Blue, dir.Count, Reset, Cyan, dir.Command, Reset)
		}
	}

	// Command Sequences
	if len(analysis.CommandSequences) > 0 {
		printSection("COMMAND SEQUENCES")
		fmt.Printf("  %sCommands you often run back-to-back:%s\n\n", Dim, Reset)
		for i, seq := range analysis.CommandSequences {
			if i >= 10 {
				break
			}
			fmt.Printf("  %s%3d%s  %s%s%s → %s%s%s\n",
				Magenta, seq.Count, Reset,
				Cyan, seq.From, Reset,
				Cyan, seq.To, Reset)
		}
	}

	// Typos
	if len(analysis.PossibleTypos) > 0 {
		printSection("POSSIBLE TYPOS")
		for _, typo := range analysis.PossibleTypos {
			fmt.Printf("  %s%dx%s  %s'%s'%s → probably meant %s'%s'%s\n",
				Yellow, typo.Count, Reset,
				Dim, typo.Typed, Reset,
				Green, typo.Intended, Reset)
		}
	}

	fmt.Println()
}

func PrintLLMRecommendations(recommendations string) {
	printSection("AI RECOMMENDATIONS")
	fmt.Println()
	fmt.Println(recommendations)
	fmt.Println()
}

func PrintError(msg string) {
	fmt.Printf("\n%s⚠ %s%s\n", Yellow, msg, Reset)
}

func PrintInfo(msg string) {
	fmt.Printf("%s%s%s\n", Dim, msg, Reset)
}

func printHeader(title, subtitle string) {
	width := 60
	fmt.Println()
	fmt.Printf("%s%s%s\n", Bold+Cyan, strings.Repeat("─", width), Reset)
	fmt.Printf("%s  ⚒  %s%s\n", Bold+Cyan, title, Reset)
	fmt.Printf("%s  %s%s\n", Dim, subtitle, Reset)
	fmt.Printf("%s%s%s\n", Bold+Cyan, strings.Repeat("─", width), Reset)
}

func printSection(title string) {
	fmt.Printf("\n%s%s ─── %s %s%s\n\n", Bold, Cyan, title, strings.Repeat("─", 40-len(title)), Reset)
}
