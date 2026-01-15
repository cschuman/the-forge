package main

import (
	"flag"
	"fmt"
	"os"

	"forge-habits/analyzer"
	"forge-habits/llm"
	"forge-habits/output"
	"forge-habits/parser"
)

var (
	version = "0.1.0"
)

func main() {
	// CLI flags
	historyFile := flag.String("file", "", "Path to history file (auto-detected if not specified)")
	shellType := flag.String("shell", "", "Shell type: zsh or bash (auto-detected if not specified)")
	noLLM := flag.Bool("no-llm", false, "Skip LLM analysis (faster, works offline)")
	model := flag.String("model", "kimi-k2-thinking:cloud", "Ollama model to use for recommendations")
	showVersion := flag.Bool("version", false, "Show version")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `forge-habits - Analyze your shell history for efficiency improvements

Usage:
  forge-habits [flags]

Flags:
`)
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, `
Examples:
  forge-habits                    # Analyze with auto-detected history
  forge-habits --no-llm           # Quick analysis without AI
  forge-habits --file ~/.zsh_history --shell zsh
`)
	}

	flag.Parse()

	if *showVersion {
		fmt.Printf("forge-habits v%s\n", version)
		os.Exit(0)
	}

	// Parse history
	output.PrintInfo("Parsing shell history...")
	historyData, err := parser.Parse(*historyFile, *shellType)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing history: %v\n", err)
		os.Exit(1)
	}

	output.PrintInfo(fmt.Sprintf("Found %d commands in %s (%s)",
		len(historyData.Commands),
		historyData.FilePath,
		historyData.ShellType))

	// Analyze
	analysis := analyzer.Analyze(historyData)

	// Print analysis
	output.PrintAnalysis(analysis)

	// LLM recommendations
	if !*noLLM {
		output.PrintInfo("Getting AI recommendations (this may take a moment)...")

		client := llm.NewClient(*model)
		recommendations, err := client.GetRecommendations(analysis)
		if err != nil {
			output.PrintError(fmt.Sprintf("Could not get AI recommendations: %v", err))
			output.PrintInfo("Run with --no-llm to skip AI analysis")
		} else {
			output.PrintLLMRecommendations(recommendations)
		}
	}
}
