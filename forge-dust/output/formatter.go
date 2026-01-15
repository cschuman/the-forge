package output

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"forge-dust/analyzer"
)

// ANSI color codes
const (
	Reset   = "\033[0m"
	Bold    = "\033[1m"
	Dim     = "\033[2m"
	Cyan    = "\033[36m"
	Green   = "\033[32m"
	Yellow  = "\033[33m"
	Red     = "\033[31m"
	Magenta = "\033[35m"
	Blue    = "\033[34m"
)

func FormatSize(bytes int64) string {
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

func FormatAge(d time.Duration) string {
	days := int(d.Hours() / 24)
	if days > 365 {
		years := days / 365
		return fmt.Sprintf("%dy ago", years)
	}
	if days > 30 {
		months := days / 30
		return fmt.Sprintf("%dmo ago", months)
	}
	if days > 0 {
		return fmt.Sprintf("%dd ago", days)
	}
	return "recent"
}

func PrintAnalysis(analysis *analyzer.Analysis) {
	printHeader("FORGE-DUST", "Disk Space Analysis")

	// Stats summary
	fmt.Printf("\n%sScanned:%s %s across %s%d%s files in %s%d%s directories\n",
		Dim, Reset,
		FormatSize(analysis.ScanStats.TotalSize),
		Bold, analysis.ScanStats.TotalFiles, Reset,
		Bold, analysis.ScanStats.TotalDirs, Reset)
	fmt.Printf("%sScan time:%s %v\n", Dim, Reset, analysis.ScanStats.ScanTime.Round(time.Millisecond))

	if analysis.TotalReclaimable > 0 {
		fmt.Printf("\n%s%s⚡ Potential space to reclaim: %s%s\n",
			Bold, Green, FormatSize(analysis.TotalReclaimable), Reset)
	}

	// Cache directories
	if len(analysis.CacheDirs) > 0 {
		printSection("CACHE DIRECTORIES")
		fmt.Printf("  %sThese can usually be safely deleted (will rebuild as needed):%s\n\n", Dim, Reset)

		var totalCache int64
		for _, cache := range analysis.CacheDirs {
			totalCache += cache.Size
			sizeStr := FormatSize(cache.Size)
			path := shortenPath(cache.Path, 50)
			fmt.Printf("  %s%8s%s  %s%-12s%s  %s%s%s\n",
				Yellow, sizeStr, Reset,
				Cyan, cache.Type, Reset,
				Dim, path, Reset)
		}
		fmt.Printf("\n  %sTotal cache: %s%s%s\n", Dim, Green, FormatSize(totalCache), Reset)
	}

	// Large files
	if len(analysis.LargeFiles) > 0 {
		printSection("LARGE FILES")
		fmt.Printf("  %sFiles over 100MB:%s\n\n", Dim, Reset)

		for i, f := range analysis.LargeFiles {
			if i >= 15 {
				fmt.Printf("  %s... and %d more%s\n", Dim, len(analysis.LargeFiles)-15, Reset)
				break
			}
			sizeStr := FormatSize(f.Size)
			path := shortenPath(f.Path, 55)
			age := FormatAge(f.Age)
			fmt.Printf("  %s%8s%s  %s%6s%s  %s%s%s\n",
				Red, sizeStr, Reset,
				Dim, age, Reset,
				Reset, path, Reset)
		}
	}

	// Downloads
	if len(analysis.Downloads) > 0 {
		printSection("DOWNLOADS FOLDER")
		fmt.Printf("  %sLarge files in ~/Downloads:%s\n\n", Dim, Reset)

		for _, f := range analysis.Downloads {
			sizeStr := FormatSize(f.Size)
			name := filepath.Base(f.Path)
			if len(name) > 50 {
				name = name[:47] + "..."
			}
			age := FormatAge(f.Age)
			fmt.Printf("  %s%8s%s  %s%6s%s  %s%s%s\n",
				Magenta, sizeStr, Reset,
				Dim, age, Reset,
				Reset, name, Reset)
		}
	}

	// Old files
	if len(analysis.OldFiles) > 0 {
		printSection("OLD FILES")
		fmt.Printf("  %sLarge files not modified in over a year:%s\n\n", Dim, Reset)

		for _, f := range analysis.OldFiles {
			sizeStr := FormatSize(f.Size)
			path := shortenPath(f.Path, 50)
			age := FormatAge(f.Age)
			fmt.Printf("  %s%8s%s  %s%6s%s  %s%s%s\n",
				Blue, sizeStr, Reset,
				Yellow, age, Reset,
				Dim, path, Reset)
		}
	}

	// Duplicates
	if len(analysis.DuplicateGroups) > 0 {
		printSection("DUPLICATE FILES")
		fmt.Printf("  %sFiles with identical content:%s\n\n", Dim, Reset)

		for _, group := range analysis.DuplicateGroups {
			fmt.Printf("  %s%s each × %d copies%s\n",
				Cyan, FormatSize(group.Size), len(group.Files), Reset)
			for _, path := range group.Files {
				fmt.Printf("    %s%s%s\n", Dim, shortenPath(path, 60), Reset)
			}
			fmt.Println()
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

func PrintDim(msg string) {
	fmt.Printf("%s%s%s\n", Dim, msg, Reset)
}

func printHeader(title, subtitle string) {
	width := 60
	fmt.Println()
	fmt.Printf("%s%s%s\n", Bold+Cyan, strings.Repeat("─", width), Reset)
	fmt.Printf("%s  🧹 %s%s\n", Bold+Cyan, title, Reset)
	fmt.Printf("%s  %s%s\n", Dim, subtitle, Reset)
	fmt.Printf("%s%s%s\n", Bold+Cyan, strings.Repeat("─", width), Reset)
}

func printSection(title string) {
	fmt.Printf("\n%s%s ─── %s %s%s\n\n", Bold, Cyan, title, strings.Repeat("─", 40-len(title)), Reset)
}

func shortenPath(path string, maxLen int) string {
	if len(path) <= maxLen {
		return path
	}

	// Try to keep the filename and shorten the middle
	dir := filepath.Dir(path)
	base := filepath.Base(path)

	if len(base) > maxLen-5 {
		return "..." + path[len(path)-maxLen+3:]
	}

	availableForDir := maxLen - len(base) - 4 // 4 for ".../""
	if availableForDir < 10 {
		return "..." + path[len(path)-maxLen+3:]
	}

	return dir[:availableForDir] + ".../" + base
}
