package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"forge-dust/analyzer"
	"forge-dust/llm"
	"forge-dust/output"
	"forge-dust/scanner"
)

var version = "0.1.0"

func main() {
	// CLI flags
	scanPath := flag.String("path", "", "Path to scan (default: home directory)")
	minSize := flag.Int64("min-size", 100, "Minimum file size in MB to report as 'large'")
	noLLM := flag.Bool("no-llm", false, "Skip LLM analysis")
	model := flag.String("model", "kimi-k2-thinking:cloud", "Ollama model for recommendations")
	checkDupes := flag.Bool("duplicates", false, "Check for duplicate files (slower)")
	showVersion := flag.Bool("version", false, "Show version")
	quick := flag.Bool("quick", false, "Quick scan (skip hidden directories, limit depth)")
	jsonOutput := flag.Bool("json", false, "Output results as JSON (for forge wrapper)")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `forge-dust - Find disk space optimization opportunities

Usage:
  forge-dust [flags]

Flags:
`)
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, `
Examples:
  forge-dust                      # Scan home directory
  forge-dust --path ~/Projects    # Scan specific directory
  forge-dust --quick              # Fast scan, less thorough
  forge-dust --duplicates         # Also find duplicate files
  forge-dust --no-llm             # Skip AI recommendations
`)
	}

	flag.Parse()

	if *showVersion {
		fmt.Printf("forge-dust v%s\n", version)
		os.Exit(0)
	}

	// Determine scan path
	path := *scanPath
	if path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting home directory: %v\n", err)
			os.Exit(1)
		}
		path = home
	}

	// Setup scanner
	s := scanner.New(path)
	if *quick {
		s.SkipHidden = true
		s.MaxDepth = 5
	}

	if !*jsonOutput {
		// Pre-scan messaging
		fmt.Println()
		output.PrintInfo(fmt.Sprintf("Scanning %s", path))
		if *quick {
			output.PrintInfo("Quick mode: skipping hidden dirs, max depth 5")
		}
		fmt.Println()
		output.PrintDim("Note: macOS may prompt for folder access permissions.")
		output.PrintDim("Grant access to allow scanning those directories.\n")

		// Setup progress callback for interactive mode
		s.OnProgress = func(p scanner.Progress) {
			// Shorten the path for display
			dir := p.CurrentDir
			if len(dir) > 50 {
				dir = "..." + dir[len(dir)-47:]
			}
			fmt.Printf("\r\033[K  %s%d files%s | %s%s%s | %s",
				output.Cyan, p.FilesScanned, output.Reset,
				output.Cyan, formatBytes(p.BytesScanned), output.Reset,
				dir)
		}
	}

	// Scan
	result, err := s.Scan()

	// Clear progress line
	if !*jsonOutput {
		fmt.Print("\r\033[K")
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Scan error: %v\n", err)
		os.Exit(1)
	}

	// Analyze
	a := analyzer.New()
	a.MinLargeFile = *minSize * 1024 * 1024
	a.CheckDuplicates = *checkDupes

	analysis := a.Analyze(result)

	// JSON output for forge wrapper
	if *jsonOutput {
		outputJSON(analysis, result)
		return
	}

	// Output
	output.PrintAnalysis(analysis)

	// LLM recommendations
	if !*noLLM {
		output.PrintInfo("Getting AI recommendations...")
		client := llm.NewClient(*model)
		recommendations, err := client.GetRecommendations(analysis)
		if err != nil {
			output.PrintError(fmt.Sprintf("Could not get AI recommendations: %v", err))
			output.PrintInfo("Run with --no-llm to skip AI analysis")
		} else {
			output.PrintLLMRecommendations(recommendations)
		}
	}

	// Print errors if any
	if len(result.Errors) > 0 {
		output.PrintInfo(fmt.Sprintf("\n%d files/directories could not be accessed", len(result.Errors)))
	}
}

// JSONOutput is the structure for forge wrapper integration
type JSONOutput struct {
	Tool        string        `json:"tool"`
	Version     string        `json:"version"`
	ScanSummary ScanSummary   `json:"scan_summary"`
	Categories  []JSONCategory `json:"categories"`
}

type ScanSummary struct {
	TotalScanned string `json:"total_scanned"`
	TotalFiles   int    `json:"total_files"`
	ScanTimeMs   int64  `json:"scan_time_ms"`
}

type JSONCategory struct {
	ID        string       `json:"id"`
	Name      string       `json:"name"`
	TotalSize int64        `json:"total_size"`
	ItemCount int          `json:"item_count"`
	Metadata  JSONMetadata `json:"metadata"`
	Items     []JSONItem   `json:"items"`
}

type JSONMetadata struct {
	TypicalRisk  string `json:"typical_risk"`
	Reversible   bool   `json:"reversible"`
	Description  string `json:"description"`
	SafeAction   string `json:"safe_action"`
}

type JSONItem struct {
	Path    string `json:"path"`
	Size    int64  `json:"size"`
	Type    string `json:"type"`
	AgeDays int    `json:"age_days,omitempty"`
}

func outputJSON(analysis *analyzer.Analysis, result *scanner.ScanResult) {
	out := JSONOutput{
		Tool:    "forge-dust",
		Version: version,
		ScanSummary: ScanSummary{
			TotalScanned: formatBytes(analysis.ScanStats.TotalSize),
			TotalFiles:   analysis.ScanStats.TotalFiles,
			ScanTimeMs:   analysis.ScanStats.ScanTime.Milliseconds(),
		},
	}

	// Cache directories
	if len(analysis.CacheDirs) > 0 {
		cat := JSONCategory{
			ID:        "cache_directories",
			Name:      "Cache Directories",
			ItemCount: len(analysis.CacheDirs),
			Metadata: JSONMetadata{
				TypicalRisk:  "low",
				Reversible:   true,
				Description:  "Build caches and package managers - all rebuildable",
				SafeAction:   "delete",
			},
		}
		for _, c := range analysis.CacheDirs {
			cat.TotalSize += c.Size
			cat.Items = append(cat.Items, JSONItem{
				Path: c.Path,
				Size: c.Size,
				Type: c.Type,
			})
		}
		out.Categories = append(out.Categories, cat)
	}

	// Large files
	if len(analysis.LargeFiles) > 0 {
		cat := JSONCategory{
			ID:        "large_files",
			Name:      "Large Files",
			ItemCount: len(analysis.LargeFiles),
			Metadata: JSONMetadata{
				TypicalRisk:  "medium",
				Reversible:   false,
				Description:  "Files over 100MB",
				SafeAction:   "review",
			},
		}
		for _, f := range analysis.LargeFiles {
			cat.TotalSize += f.Size
			cat.Items = append(cat.Items, JSONItem{
				Path:    f.Path,
				Size:    f.Size,
				Type:    "large_file",
				AgeDays: int(f.Age.Hours() / 24),
			})
		}
		out.Categories = append(out.Categories, cat)
	}

	// Downloads
	if len(analysis.Downloads) > 0 {
		cat := JSONCategory{
			ID:        "downloads",
			Name:      "Downloads",
			ItemCount: len(analysis.Downloads),
			Metadata: JSONMetadata{
				TypicalRisk:  "low",
				Reversible:   false,
				Description:  "Large files in Downloads folder",
				SafeAction:   "suggest_delete",
			},
		}
		for _, f := range analysis.Downloads {
			cat.TotalSize += f.Size
			cat.Items = append(cat.Items, JSONItem{
				Path:    f.Path,
				Size:    f.Size,
				Type:    "download",
				AgeDays: int(f.Age.Hours() / 24),
			})
		}
		out.Categories = append(out.Categories, cat)
	}

	// Old files
	if len(analysis.OldFiles) > 0 {
		cat := JSONCategory{
			ID:        "old_files",
			Name:      "Old Files",
			ItemCount: len(analysis.OldFiles),
			Metadata: JSONMetadata{
				TypicalRisk:  "medium",
				Reversible:   false,
				Description:  "Large files not modified in over a year",
				SafeAction:   "review",
			},
		}
		for _, f := range analysis.OldFiles {
			cat.TotalSize += f.Size
			cat.Items = append(cat.Items, JSONItem{
				Path:    f.Path,
				Size:    f.Size,
				Type:    "old_file",
				AgeDays: int(f.Age.Hours() / 24),
			})
		}
		out.Categories = append(out.Categories, cat)
	}

	// Output JSON
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.Encode(out)
}

func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}
