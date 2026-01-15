package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"forge-habits/analyzer"
	"forge-habits/parser"
	"forge-habits/shell"
	"forge-habits/suggestions"
)

// ANSI colors
const (
	Reset   = "\033[0m"
	Bold    = "\033[1m"
	Dim     = "\033[2m"
	Cyan    = "\033[36m"
	Green   = "\033[32m"
	Yellow  = "\033[33m"
	Red     = "\033[31m"
)

var (
	version = "0.1.0"
	reader  *bufio.Reader
)

func main() {
	reader = bufio.NewReader(os.Stdin)

	// CLI flags
	historyFile := flag.String("file", "", "Path to history file (auto-detected if not specified)")
	shellType := flag.String("shell", "", "Shell type: zsh or bash (auto-detected if not specified)")
	showVersion := flag.Bool("version", false, "Show version")
	reportOnly := flag.Bool("report", false, "Just show report, no interactive prompts")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `forge-habits - Analyze shell history and forge better workflows

Usage:
  forge-habits [flags]

Flags:
`)
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, `
Examples:
  forge-habits                    # Interactive analysis
  forge-habits --report           # Just show the report
`)
	}

	flag.Parse()

	if *showVersion {
		fmt.Printf("forge-habits v%s\n", version)
		os.Exit(0)
	}

	// Parse history
	printInfo("Examining your command history...")
	historyData, err := parser.Parse(*historyFile, *shellType)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing history: %v\n", err)
		os.Exit(1)
	}

	printInfo(fmt.Sprintf("Found %d commands in %s",
		len(historyData.Commands),
		historyData.FilePath))

	// Analyze
	analysis := analyzer.Analyze(historyData)

	// Generate actionable suggestions
	suggestionSet := suggestions.Generate(analysis)

	// Show header
	printHeader()

	if *reportOnly {
		showReport(analysis, suggestionSet)
		return
	}

	// Interactive flow
	runInteractive(analysis, suggestionSet)
}

func printHeader() {
	fmt.Println()
	fmt.Printf("%s%s────────────────────────────────────────────────────────────%s\n", Bold, Cyan, Reset)
	fmt.Printf("%s  ⚒  FORGE-HABITS%s\n", Bold+Cyan, Reset)
	fmt.Printf("%s────────────────────────────────────────────────────────────%s\n", Bold+Cyan, Reset)
}

func printInfo(msg string) {
	fmt.Printf("%s%s%s\n", Dim, msg, Reset)
}

func runInteractive(analysis *analyzer.Analysis, set *suggestions.SuggestionSet) {
	// Get RC file path
	rcPath, err := shell.GetRCFile()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not determine shell config file: %v\n", err)
		return
	}

	// Filter out suggestions that already exist
	var highImpact []suggestions.Suggestion
	for _, s := range set.HighImpact {
		exists, _ := shell.HasAlias(rcPath, s.Name)
		if !exists {
			highImpact = append(highImpact, s)
		}
	}

	var review []suggestions.Suggestion
	for _, s := range set.Review {
		exists, _ := shell.HasAlias(rcPath, s.Name)
		if !exists {
			review = append(review, s)
		}
	}

	if len(highImpact) == 0 && len(review) == 0 {
		fmt.Printf("\n%sNo new suggestions found. Your workflow is already well-forged!%s\n", Dim, Reset)
		showTips(set.Tips)
		return
	}

	// High impact suggestions
	if len(highImpact) > 0 {
		fmt.Printf("\n%sFound %d high-impact improvements ready to forge:%s\n\n", Bold, len(highImpact), Reset)

		for i, s := range highImpact {
			fmt.Printf("  %s[%d]%s %s%s%s\n", Cyan, i+1, Reset, Bold, s.Name, Reset)
			fmt.Printf("      %s%s%s\n", Dim, s.Description, Reset)
			fmt.Printf("      %s%s%s\n\n", Dim, truncate(s.Command, 60), Reset)
		}

		fmt.Printf("Add these to %s%s%s? %s[Y/n]%s ", Cyan, rcPath, Reset, Dim, Reset)
		response := readLine()

		if response == "" || strings.ToLower(response) == "y" || strings.ToLower(response) == "yes" {
			var toAdd []string
			for _, s := range highImpact {
				toAdd = append(toAdd, s.Code)
			}

			// Backup first
			backupPath, _ := shell.Backup(rcPath)
			if backupPath != "" {
				printInfo(fmt.Sprintf("Backed up to %s", backupPath))
			}

			if err := shell.AddToRC(rcPath, toAdd); err != nil {
				fmt.Printf("%sError writing to %s: %v%s\n", Red, rcPath, err, Reset)
			} else {
				fmt.Printf("\n%s✓ Forged %d improvements into %s%s\n", Green, len(toAdd), rcPath, Reset)
				fmt.Printf("%sRun 'source %s' or open a new terminal to use them.%s\n", Dim, rcPath, Reset)
			}
		} else {
			fmt.Printf("%sSkipped.%s\n", Dim, Reset)
		}
	}

	// Review suggestions
	if len(review) > 0 {
		fmt.Printf("\n%s───%s\n", Cyan, Reset)
		fmt.Printf("\n%sFound %d more patterns worth reviewing:%s\n\n", Bold, len(review), Reset)

		for i, s := range review {
			fmt.Printf("  %s[%d]%s %s%s%s - %s\n",
				Cyan, i+1, Reset,
				Bold, s.Name, Reset,
				s.Description)
		}

		fmt.Printf("\n  %s[1-%d]%s Inspect  %s[a]%s Add all  %s[s]%s Skip\n",
			Cyan, len(review), Reset,
			Green, Reset,
			Dim, Reset)
		fmt.Printf("\n%s→%s ", Cyan, Reset)

		input := readLine()

		// Check if number
		if num, err := strconv.Atoi(input); err == nil && num >= 1 && num <= len(review) {
			inspectSuggestion(review[num-1], rcPath)
		} else if strings.ToLower(input) == "a" {
			var toAdd []string
			for _, s := range review {
				toAdd = append(toAdd, s.Code)
			}
			if err := shell.AddToRC(rcPath, toAdd); err != nil {
				fmt.Printf("%sError: %v%s\n", Red, err, Reset)
			} else {
				fmt.Printf("\n%s✓ Forged %d more improvements.%s\n", Green, len(toAdd), Reset)
			}
		}
	}

	// Show tips
	showTips(set.Tips)

	fmt.Printf("\n%sForged and finished.%s\n\n", Green, Reset)
}

func inspectSuggestion(s suggestions.Suggestion, rcPath string) {
	fmt.Printf("\n%s────────────────────────────────────────────────%s\n", Cyan, Reset)
	fmt.Printf("  %sName:%s %s\n", Bold, Reset, s.Name)
	fmt.Printf("  %sOriginal:%s %s\n", Bold, Reset, s.Command)
	fmt.Printf("  %sImpact:%s Used %d times\n", Bold, Reset, s.Impact)
	fmt.Printf("\n  %sWould add:%s\n", Bold, Reset)
	fmt.Printf("  %s%s%s\n", Dim, s.Code, Reset)
	fmt.Printf("%s────────────────────────────────────────────────%s\n", Cyan, Reset)

	fmt.Printf("\n  %s[a]%s Add  %s[s]%s Skip  %s[b]%s Back\n",
		Green, Reset, Yellow, Reset, Dim, Reset)
	fmt.Printf("\n%s→%s ", Cyan, Reset)

	input := readLine()

	switch strings.ToLower(input) {
	case "a", "add":
		if err := shell.AddToRC(rcPath, []string{s.Code}); err != nil {
			fmt.Printf("%sError: %v%s\n", Red, err, Reset)
		} else {
			fmt.Printf("%s✓ Added %s%s\n", Green, s.Name, Reset)
		}
	default:
		fmt.Printf("%sSkipped.%s\n", Dim, Reset)
	}
}

func showTips(tips []suggestions.Suggestion) {
	if len(tips) == 0 {
		return
	}

	fmt.Printf("\n%s───%s\n", Cyan, Reset)
	fmt.Printf("\n%s💡 Tips:%s\n\n", Bold, Reset)

	for _, tip := range tips {
		fmt.Printf("  %s•%s %s\n", Yellow, Reset, tip.Description)
	}
}

func showReport(analysis *analyzer.Analysis, set *suggestions.SuggestionSet) {
	fmt.Printf("\n%sTotal commands analyzed: %d%s\n", Dim, analysis.TotalCommands, Reset)

	// Top commands
	fmt.Printf("\n%s── Top Commands ──%s\n\n", Bold+Cyan, Reset)
	for i, tc := range analysis.TopCommands {
		if i >= 10 {
			break
		}
		bar := strings.Repeat("█", min(30, tc.Count/20+1))
		fmt.Printf("  %-12s %4d %s%s%s\n", tc.Command, tc.Count, Cyan, bar, Reset)
	}

	// High impact suggestions
	if len(set.HighImpact) > 0 {
		fmt.Printf("\n%s── High-Impact Suggestions ──%s\n\n", Bold+Cyan, Reset)
		for _, s := range set.HighImpact {
			fmt.Printf("  %s%s%s - %s\n", Bold, s.Name, Reset, s.Description)
			fmt.Printf("    %s%s%s\n\n", Dim, s.Code, Reset)
		}
	}

	// Review suggestions
	if len(set.Review) > 0 {
		fmt.Printf("\n%s── Worth Reviewing ──%s\n\n", Bold+Cyan, Reset)
		for _, s := range set.Review {
			fmt.Printf("  %s%s%s - %s\n", Bold, s.Name, Reset, s.Description)
		}
	}

	// Tips
	showTips(set.Tips)
}

func readLine() string {
	line, _ := reader.ReadString('\n')
	return strings.TrimSpace(line)
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}
