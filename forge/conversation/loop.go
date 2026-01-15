package conversation

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"forge/assessment"
	"forge/llm"
	"forge/session"
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
	Magenta = "\033[35m"
)

// Loop handles the interactive conversation with the user
type Loop struct {
	Assessment *assessment.SessionAssessment
	Session    *session.Session
	Client     *llm.OllamaClient
	reader     *bufio.Reader
}

// NewLoop creates a new conversation loop
func NewLoop(assess *assessment.SessionAssessment, sess *session.Session, client *llm.OllamaClient) *Loop {
	return &Loop{
		Assessment: assess,
		Session:    sess,
		Client:     client,
		reader:     bufio.NewReader(os.Stdin),
	}
}

// Run executes the conversation loop
func (l *Loop) Run() error {
	// Display opening
	l.printHeader()
	fmt.Printf("\n%s%s%s\n\n", Dim, l.Assessment.OpeningMessage, Reset)

	// Route based on mode
	switch l.Assessment.OverallMode {
	case assessment.ModeAuto:
		return l.runAutoMode()
	case assessment.ModeSuggest:
		return l.runSuggestMode()
	case assessment.ModeGuided:
		return l.runGuidedMode()
	case assessment.ModeCollaborative:
		return l.runCollaborativeMode()
	case assessment.ModeInformative:
		return l.runInformativeMode()
	default:
		fmt.Println("Nothing significant found.")
		return nil
	}
}

func (l *Loop) printHeader() {
	fmt.Println()
	fmt.Printf("%s%s────────────────────────────────────────────────────────────%s\n", Bold, Cyan, Reset)
	fmt.Printf("%s  ⚒  FORGE%s\n", Bold+Cyan, Reset)
	fmt.Printf("%s────────────────────────────────────────────────────────────%s\n", Bold+Cyan, Reset)
}

func (l *Loop) runAutoMode() error {
	fmt.Printf("%s⚡ Burning off the slag...%s\n\n", Green, Reset)

	for _, cat := range l.Assessment.Categories {
		if cat.Mode == assessment.ModeAuto {
			fmt.Printf("  %s✓%s %s (%s)\n", Green, Reset, cat.Category, formatBytes(cat.TotalSize))

			// Record interaction
			l.Session.AddInteraction(session.Interaction{
				Category:     cat.Category,
				TotalSize:    cat.TotalSize,
				Suggestion:   "auto_delete",
				Confidence:   cat.Confidence,
				UserResponse: "auto_accepted",
				BytesFreed:   cat.TotalSize,
			})
		}
	}

	fmt.Printf("\n%sForged and finished.%s\n", Green, Reset)
	return nil
}

func (l *Loop) runSuggestMode() error {
	totalSize := int64(0)
	for _, cat := range l.Assessment.Categories {
		totalSize += cat.TotalSize
	}

	fmt.Printf("Found %s%s%s of raw material to reclaim:\n\n", Bold, formatBytes(totalSize), Reset)

	for _, cat := range l.Assessment.Categories {
		icon := "🟢"
		if cat.Risk == "medium" {
			icon = "🟡"
		}
		fmt.Printf("  %s %s (%s)\n", icon, cat.Category, formatBytes(cat.TotalSize))
	}

	fmt.Printf("\nClean all? %s[Y/n]%s ", Dim, Reset)

	response := l.readLine()
	accepted := response == "" || strings.ToLower(response) == "y" || strings.ToLower(response) == "yes"

	userResp := "accept"
	if !accepted {
		userResp = "reject"
	}

	for _, cat := range l.Assessment.Categories {
		l.Session.AddInteraction(session.Interaction{
			Category:     cat.Category,
			TotalSize:    cat.TotalSize,
			Suggestion:   "suggest_delete",
			Confidence:   cat.Confidence,
			UserResponse: userResp,
		})
	}

	if accepted {
		fmt.Printf("\n%s✓ Firing up the crucible...%s\n", Green, Reset)
		// TODO: Actually execute cleanup
		fmt.Printf("%sForged and finished.%s\n", Green, Reset)
	} else {
		fmt.Println("\nThe metal cools. Nothing changed.")
	}

	return nil
}

func (l *Loop) runGuidedMode() error {
	fmt.Printf("Found %s%d ore deposits%s to inspect:\n\n", Bold, len(l.Assessment.Categories), Reset)

	for i, cat := range l.Assessment.Categories {
		icon := "🟢"
		if cat.Risk == "medium" {
			icon = "🟡"
		} else if cat.Risk == "high" {
			icon = "🔴"
		}
		fmt.Printf("  %s[%d]%s %s %s (%s)\n", Cyan, i+1, Reset, icon, cat.Category, formatBytes(cat.TotalSize))
	}

	fmt.Printf("\n  %s[a]%s Clean all safe items\n", Cyan, Reset)
	fmt.Printf("  %s[q]%s Quit\n", Cyan, Reset)

	for {
		fmt.Printf("\n%s→%s Pick a category (1-%d), or action: ", Cyan, Reset, len(l.Assessment.Categories))
		input := l.readLine()

		if input == "q" || input == "quit" {
			fmt.Println("Banking the fire. Until next time.")
			return nil
		}

		if input == "a" || input == "all" {
			return l.cleanAllSafe()
		}

		// Try to parse as category number
		num, err := strconv.Atoi(input)
		if err == nil && num >= 1 && num <= len(l.Assessment.Categories) {
			if err := l.exploreCat(num - 1); err != nil {
				return err
			}
			continue
		}

		fmt.Printf("%sInvalid choice. Try again.%s\n", Yellow, Reset)
	}
}

func (l *Loop) exploreCat(idx int) error {
	cat := l.Assessment.Categories[idx]

	fmt.Printf("\n%s── %s (%s) ──%s\n\n", Bold+Cyan, cat.Category, formatBytes(cat.TotalSize), Reset)

	// Group files by type for better understanding
	groups := groupFilesByType(cat.Findings)

	// Display grouped files with context
	fileNum := 1
	fileMap := make(map[int]assessment.Finding)

	for groupName, files := range groups {
		if len(files) == 0 {
			continue
		}

		groupSize := int64(0)
		for _, f := range files {
			groupSize += f.Size
		}

		fmt.Printf("  %s%s%s %s(%s)%s\n", Bold, groupName, Reset, Dim, formatBytes(groupSize), Reset)

		for _, f := range files {
			fileMap[fileNum] = f
			// Show number, size, and readable filename
			filename := filepath.Base(f.Path)
			parentDir := filepath.Base(filepath.Dir(f.Path))

			// Truncate filename if too long, but keep it readable
			displayName := filename
			if len(filename) > 45 {
				displayName = filename[:42] + "..."
			}

			fmt.Printf("    %s[%2d]%s %s%8s%s  %s\n",
				Cyan, fileNum, Reset,
				Yellow, formatBytes(f.Size), Reset,
				displayName)
			fmt.Printf("         %sin %s%s\n", Dim, parentDir, Reset)

			fileNum++
			if fileNum > 20 {
				remaining := len(cat.Findings) - 20
				if remaining > 0 {
					fmt.Printf("\n    %s... and %d more files%s\n", Dim, remaining, Reset)
				}
				break
			}
		}
		fmt.Println()
	}

	// Interactive loop for this category
	for {
		fmt.Printf("  %s[1-%d]%s Inspect file  %s[d]%s Delete all  %s[s]%s Skip  %s[b]%s Back\n",
			Cyan, len(fileMap), Reset,
			Green, Reset,
			Yellow, Reset,
			Dim, Reset)
		fmt.Printf("\n%s→%s ", Cyan, Reset)

		input := l.readLine()

		// Check if it's a number (file selection)
		if num, err := strconv.Atoi(input); err == nil && num >= 1 && num <= len(fileMap) {
			l.inspectFile(fileMap[num])
			continue
		}

		var userResp string
		switch strings.ToLower(input) {
		case "d", "delete":
			userResp = "accept"
			fmt.Printf("\n%s✓ Into the furnace%s\n", Green, Reset)
		case "s", "skip":
			userResp = "reject"
			fmt.Println("\nSet aside for now.")
		case "b", "back", "q":
			return nil
		default:
			fmt.Printf("%sType a number to examine, or pick an action.%s\n", Dim, Reset)
			continue
		}

		l.Session.AddInteraction(session.Interaction{
			Category:     cat.Category,
			TotalSize:    cat.TotalSize,
			Suggestion:   cat.Action,
			Confidence:   cat.Confidence,
			UserResponse: userResp,
		})

		return nil
	}
}

// groupFilesByType organizes files into meaningful groups
func groupFilesByType(findings []assessment.Finding) map[string][]assessment.Finding {
	groups := map[string][]assessment.Finding{
		"🐳 Docker & Containers": {},
		"🤖 AI/ML Models":        {},
		"🎬 Videos":              {},
		"📦 Archives":            {},
		"💾 Disk Images":         {},
		"📁 Application Data":    {},
		"📄 Other":               {},
	}

	for _, f := range findings {
		path := strings.ToLower(f.Path)
		filename := strings.ToLower(filepath.Base(f.Path))
		ext := strings.ToLower(filepath.Ext(f.Path))

		switch {
		case strings.Contains(path, "docker") || strings.Contains(path, "container"):
			groups["🐳 Docker & Containers"] = append(groups["🐳 Docker & Containers"], f)
		case strings.Contains(path, "whisper") || strings.Contains(path, "llama") ||
			strings.Contains(path, "models") || strings.Contains(filename, "ggml") ||
			strings.Contains(path, "huggingface") || strings.Contains(path, "transformers"):
			groups["🤖 AI/ML Models"] = append(groups["🤖 AI/ML Models"], f)
		case ext == ".mp4" || ext == ".mov" || ext == ".avi" || ext == ".mkv" ||
			ext == ".wmv" || ext == ".m4v" || ext == ".webm":
			groups["🎬 Videos"] = append(groups["🎬 Videos"], f)
		case ext == ".zip" || ext == ".tar" || ext == ".gz" || ext == ".7z" ||
			ext == ".rar" || ext == ".tar.gz" || ext == ".tgz":
			groups["📦 Archives"] = append(groups["📦 Archives"], f)
		case ext == ".dmg" || ext == ".iso" || ext == ".img" || ext == ".raw":
			groups["💾 Disk Images"] = append(groups["💾 Disk Images"], f)
		case strings.Contains(path, "application support") || strings.Contains(path, "library"):
			groups["📁 Application Data"] = append(groups["📁 Application Data"], f)
		default:
			groups["📄 Other"] = append(groups["📄 Other"], f)
		}
	}

	return groups
}

// inspectFile shows detailed info about a specific file and asks LLM for context
func (l *Loop) inspectFile(f assessment.Finding) {
	fmt.Printf("\n%s────────────────────────────────────────────────%s\n", Cyan, Reset)
	fmt.Printf("  %sFile:%s %s\n", Bold, Reset, filepath.Base(f.Path))
	fmt.Printf("  %sSize:%s %s\n", Bold, Reset, formatBytes(f.Size))
	fmt.Printf("  %sPath:%s %s\n", Bold, Reset, f.Path)
	if f.AgeDays > 0 {
		fmt.Printf("  %sAge:%s %s\n", Bold, Reset, formatAgeDays(f.AgeDays))
	}
	fmt.Printf("%s────────────────────────────────────────────────%s\n", Cyan, Reset)

	// Ask LLM for context
	fmt.Printf("\n%sAnalyzing...%s", Dim, Reset)

	prompt := fmt.Sprintf(`What is this file and is it safe to delete? Be specific and concise (2-3 sentences).

File: %s
Size: %s
Full path: %s

Consider: Is this user data that can't be recovered? Is it a cache/temp file? Is it from a specific application?`,
		filepath.Base(f.Path), formatBytes(f.Size), f.Path)

	explanation, err := l.Client.Generate(prompt)
	if err != nil {
		fmt.Printf("\r%s                    %s\n", Reset, Reset)
		fmt.Printf("  %sCouldn't analyze - check if Ollama is running%s\n", Yellow, Reset)
	} else {
		fmt.Printf("\r%s                    %s\n", Reset, Reset)
		// Clean up and display the explanation
		explanation = strings.TrimSpace(explanation)
		fmt.Printf("  %s%s%s\n", Dim, explanation, Reset)
	}

	fmt.Printf("\n  %s[d]%s Delete  %s[k]%s Keep  %s[o]%s Open folder  %s[b]%s Back\n",
		Red, Reset, Green, Reset, Cyan, Reset, Dim, Reset)
	fmt.Printf("\n%s→%s ", Cyan, Reset)

	input := l.readLine()

	switch strings.ToLower(input) {
	case "d", "delete":
		fmt.Printf("%s✓ Marked for the crucible%s\n", Green, Reset)
		l.Session.AddInteraction(session.Interaction{
			Category:     "individual_file",
			Item:         f.Path,
			TotalSize:    f.Size,
			Suggestion:   "delete",
			UserResponse: "accept",
		})
	case "o", "open":
		// Open the folder in Finder
		dir := filepath.Dir(f.Path)
		exec.Command("open", dir).Run()
		fmt.Printf("%sOpened in Finder%s\n", Dim, Reset)
	case "k", "keep":
		fmt.Printf("%s✓ Preserved%s\n", Green, Reset)
		l.Session.AddInteraction(session.Interaction{
			Category:     "individual_file",
			Item:         f.Path,
			TotalSize:    f.Size,
			Suggestion:   "delete",
			UserResponse: "reject",
		})
	}
	fmt.Println()
}

func formatAgeDays(days int) string {
	if days > 365 {
		years := days / 365
		return fmt.Sprintf("%d years ago", years)
	}
	if days > 30 {
		months := days / 30
		return fmt.Sprintf("%d months ago", months)
	}
	if days > 0 {
		return fmt.Sprintf("%d days ago", days)
	}
	return "recently modified"
}

func (l *Loop) explainCategory(cat assessment.CategoryAssessment) {
	prompt := fmt.Sprintf(`Explain in 2-3 sentences what "%s" files are and whether they're safe to delete. Be concise and helpful. The user is looking at %d files totaling %s.`,
		cat.Category, len(cat.Findings), formatBytes(cat.TotalSize))

	fmt.Printf("\n%sThinking...%s\n", Dim, Reset)

	explanation, err := l.Client.Generate(prompt)
	if err != nil {
		fmt.Printf("\n%s%s%s\n", Dim, cat.Explanation, Reset)
		return
	}

	fmt.Printf("\n%s%s%s\n", Dim, explanation, Reset)
}

func (l *Loop) cleanAllSafe() error {
	fmt.Printf("\n%sSmelting the pure ore...%s\n\n", Green, Reset)

	for _, cat := range l.Assessment.Categories {
		if cat.Risk != "high" {
			fmt.Printf("  %s✓%s %s (%s)\n", Green, Reset, cat.Category, formatBytes(cat.TotalSize))

			l.Session.AddInteraction(session.Interaction{
				Category:     cat.Category,
				TotalSize:    cat.TotalSize,
				Suggestion:   "clean_all_safe",
				Confidence:   cat.Confidence,
				UserResponse: "accept",
				BytesFreed:   cat.TotalSize,
			})
		}
	}

	fmt.Printf("\n%sForged and finished.%s\n", Green, Reset)
	return nil
}

func (l *Loop) runCollaborativeMode() error {
	fmt.Printf("Found some unusual alloys that need your eye.\n\n")

	for _, cat := range l.Assessment.Categories {
		if cat.Risk == "high" || cat.Confidence == "low" {
			fmt.Printf("%s── %s ──%s\n\n", Bold+Cyan, cat.Category, Reset)

			for _, finding := range cat.Findings {
				fmt.Printf("  %s\n", shortenPath(finding.Path, 60))
				fmt.Printf("  Size: %s\n\n", formatBytes(finding.Size))

				fmt.Printf("  What would you like to do?\n")
				fmt.Printf("  %s[d]%s Delete  %s[k]%s Keep  %s[?]%s Tell me more\n\n",
					Red, Reset, Green, Reset, Cyan, Reset)
				fmt.Printf("%s→%s ", Cyan, Reset)

				input := l.readLine()

				var userResp string
				switch strings.ToLower(input) {
				case "d", "delete":
					userResp = "accept"
					fmt.Printf("%s✓ Into the crucible%s\n\n", Green, Reset)
				case "k", "keep":
					userResp = "reject"
					fmt.Printf("%s✓ Set aside%s\n\n", Green, Reset)
				case "?":
					userResp = "explain"
					l.explainFile(finding)
				default:
					userResp = "skip"
					fmt.Println("Passing over.\n")
				}

				l.Session.AddInteraction(session.Interaction{
					Category:     cat.Category,
					Item:         finding.Path,
					TotalSize:    finding.Size,
					Suggestion:   "discuss",
					Confidence:   cat.Confidence,
					UserResponse: userResp,
				})
			}
		}
	}

	return nil
}

func (l *Loop) explainFile(finding assessment.Finding) {
	prompt := fmt.Sprintf(`The user is looking at this file and wondering if they should delete it:

Path: %s
Size: %s
Type: %s

Give a brief (2-3 sentence) explanation of what this file likely is and whether it's safe to delete. Be helpful but cautious.`,
		finding.Path, formatBytes(finding.Size), finding.Type)

	fmt.Printf("\n%sThinking...%s\n", Dim, Reset)

	explanation, err := l.Client.Generate(prompt)
	if err != nil {
		fmt.Printf("\n%sI'm not sure about this file.%s\n\n", Dim, Reset)
		return
	}

	fmt.Printf("\n%s%s%s\n\n", Dim, explanation, Reset)
}

func (l *Loop) runInformativeMode() error {
	fmt.Printf("Laid out the materials for your inspection.\n\n")

	for _, cat := range l.Assessment.Categories {
		fmt.Printf("%s── %s (%s) ──%s\n\n", Bold+Cyan, cat.Category, formatBytes(cat.TotalSize), Reset)

		for i, finding := range cat.Findings {
			if i >= 10 {
				fmt.Printf("  %s... and %d more%s\n", Dim, len(cat.Findings)-10, Reset)
				break
			}
			fmt.Printf("  %s (%s)\n", shortenPath(finding.Path, 50), formatBytes(finding.Size))
		}
		fmt.Println()

		l.Session.AddInteraction(session.Interaction{
			Category:     cat.Category,
			TotalSize:    cat.TotalSize,
			Suggestion:   "inform_only",
			Confidence:   cat.Confidence,
			UserResponse: "viewed",
		})
	}

	fmt.Printf("%sThe forge stands ready. Return when you're prepared to work the metal.%s\n", Dim, Reset)
	return nil
}

func (l *Loop) readLine() string {
	line, _ := l.reader.ReadString('\n')
	return strings.TrimSpace(line)
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

func shortenPath(path string, maxLen int) string {
	if len(path) <= maxLen {
		return path
	}
	return "..." + path[len(path)-maxLen+3:]
}
