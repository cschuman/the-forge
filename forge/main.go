package main

import (
	"fmt"
	"os"
	"os/exec"
	"time"

	"forge/assessment"
	"forge/conversation"
	"forge/learning"
	"forge/llm"
	"forge/rules"
	"forge/session"
)

var version = "0.1.0"

func main() {
	// Subcommands
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "dust":
			runTool("forge-dust", os.Args[2:])
			return
		case "habits":
			runTool("forge-habits", os.Args[2:])
			return
		case "review":
			runReview()
			return
		case "learn":
			runLearn()
			return
		case "always":
			if len(os.Args) > 2 {
				runAlways(os.Args[2])
			} else {
				fmt.Println("Usage: forge always <pattern>")
			}
			return
		case "never":
			if len(os.Args) > 2 {
				runNever(os.Args[2])
			} else {
				fmt.Println("Usage: forge never <pattern>")
			}
			return
		case "forget":
			if len(os.Args) > 2 {
				runForget(os.Args[2])
			} else {
				fmt.Println("Usage: forge forget <pattern>")
			}
			return
		case "reset":
			runReset(len(os.Args) > 2 && os.Args[2] == "--all")
			return
		case "rules":
			runShowRules()
			return
		case "sessions":
			runShowSessions()
			return
		case "version":
			fmt.Printf("forge v%s\n", version)
			return
		case "help", "--help", "-h":
			printHelp()
			return
		}
	}

	// No subcommand - show help
	printHelp()
}

// ANSI codes for output
const (
	Reset   = "\033[0m"
	Bold    = "\033[1m"
	Dim     = "\033[2m"
	Cyan    = "\033[36m"
	Green   = "\033[32m"
	Yellow  = "\033[33m"
)

func runTool(tool string, args []string) {
	// Load rules
	rs, err := rules.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not load rules: %v\n", err)
		rs = &rules.RuleSet{}
	}

	// Initialize LLM client
	client := llm.NewClient("kimi-k2-thinking:cloud")

	// Check for --no-llm flag
	noLLM := false
	var filteredArgs []string
	for _, arg := range args {
		if arg == "--no-llm" {
			noLLM = true
		} else {
			filteredArgs = append(filteredArgs, arg)
		}
	}

	// Show pre-run messaging
	toolDesc := getToolDescription(tool)
	fmt.Println()
	fmt.Printf("%s%s────────────────────────────────────────────────────────────%s\n", Bold, Cyan, Reset)
	fmt.Printf("%s  ⚒  FORGE%s\n", Bold+Cyan, Reset)
	fmt.Printf("%s────────────────────────────────────────────────────────────%s\n", Bold+Cyan, Reset)
	fmt.Println()
	fmt.Printf("%s%s%s\n", Dim, toolDesc, Reset)
	fmt.Println()
	fmt.Printf("%sNote: macOS may prompt for folder access.%s\n", Dim, Reset)
	fmt.Printf("%sGrant access to allow scanning protected directories.%s\n\n", Dim, Reset)

	// Show spinner while running
	done := make(chan bool)
	go showSpinner("Scanning", done)

	// Run the tool with --json flag
	toolArgs := append(filteredArgs, "--json")
	cmd := exec.Command(tool, toolArgs...)
	output, err := cmd.Output()

	// Stop spinner
	done <- true
	fmt.Print("\r\033[K") // Clear the spinner line

	if err != nil {
		// Tool might not support --json yet, fall back to normal execution
		fmt.Printf("%sRunning %s...%s\n", Dim, tool, Reset)
		cmd := exec.Command(tool, filteredArgs...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
		cmd.Run()
		return
	}

	// Parse tool output
	toolOutput, err := assessment.ParseToolOutput(output)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing tool output: %v\n", err)
		return
	}

	// Create session
	sess := session.NewSession(tool)

	// Create assessor
	assessor := assessment.NewAssessor(rs, client)

	// Assess findings
	var assess *assessment.SessionAssessment
	if noLLM {
		assess, err = assessor.Assess(toolOutput, args)
	} else {
		assess, err = assessor.AssessWithLLM(toolOutput, args)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error assessing: %v\n", err)
		return
	}

	// Run conversation loop
	loop := conversation.NewLoop(assess, sess, client)
	if err := loop.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	}

	// Save session
	sess.Finish()
	if err := sess.Save(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not save session: %v\n", err)
	}

	// Check if we should reflect
	learner := learning.NewLearner(rs, client)
	if learner.ShouldReflect() && !noLLM {
		fmt.Println("\n⚙ Running learning reflection...")
		result, err := learner.Reflect()
		if err == nil {
			applied, _ := learner.ApplyCalibrations(result)
			if len(applied) > 0 {
				fmt.Printf("Learned %d new patterns from your usage.\n", len(applied))
			}
		}
	}
}

func runReview() {
	rs, _ := rules.Load()
	client := llm.NewClient("kimi-k2-thinking:cloud")
	learner := learning.NewLearner(rs, client)

	fmt.Println(learner.GetLearningSummary())
}

func runLearn() {
	rs, err := rules.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading rules: %v\n", err)
		return
	}

	client := llm.NewClient("kimi-k2-thinking:cloud")
	learner := learning.NewLearner(rs, client)

	fmt.Println("Running learning reflection...")

	result, err := learner.Reflect()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return
	}

	fmt.Printf("\nAnalyzed %d sessions, %d total interactions\n",
		result.AnalysisSummary.SessionsAnalyzed,
		result.AnalysisSummary.TotalInteractions)
	fmt.Printf("Overall acceptance rate: %.0f%%\n\n",
		result.AnalysisSummary.OverallAcceptanceRate*100)

	if len(result.Calibrations) > 0 {
		fmt.Println("Proposed calibrations:")
		for _, cal := range result.Calibrations {
			fmt.Printf("  • %s: %s → %s (%.0f%% confidence)\n",
				cal.Pattern, cal.CurrentAction, cal.ProposedAction,
				cal.ConfidenceInProposal*100)
		}

		fmt.Print("\nApply these calibrations? [Y/n] ")
		var input string
		fmt.Scanln(&input)

		if input == "" || input == "y" || input == "Y" {
			applied, err := learner.ApplyCalibrations(result)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				return
			}
			fmt.Printf("Applied %d calibrations.\n", len(applied))
		}
	} else {
		fmt.Println("No calibrations needed at this time.")
	}

	if result.Insights != "" {
		fmt.Printf("\nInsights:\n%s\n", result.Insights)
	}
}

func runAlways(pattern string) {
	rs, _ := rules.Load()
	client := llm.NewClient("kimi-k2-thinking:cloud")
	learner := learning.NewLearner(rs, client)

	if err := learner.AddPreference("always_delete", pattern, "", "User specified"); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return
	}

	fmt.Printf("✓ Will always delete: %s\n", pattern)
}

func runNever(pattern string) {
	rs, _ := rules.Load()
	client := llm.NewClient("kimi-k2-thinking:cloud")
	learner := learning.NewLearner(rs, client)

	if err := learner.AddPreference("never_delete", pattern, "", "User specified"); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return
	}

	fmt.Printf("✓ Will never delete: %s\n", pattern)
}

func runForget(pattern string) {
	rs, _ := rules.Load()
	client := llm.NewClient("kimi-k2-thinking:cloud")
	learner := learning.NewLearner(rs, client)

	if learner.ForgetCalibration(pattern) {
		fmt.Printf("✓ Forgot learned behavior for: %s\n", pattern)
	} else {
		fmt.Printf("No learned behavior found for: %s\n", pattern)
	}
}

func runReset(includePrefs bool) {
	rs, _ := rules.Load()
	client := llm.NewClient("kimi-k2-thinking:cloud")
	learner := learning.NewLearner(rs, client)

	if err := learner.Reset(includePrefs); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return
	}

	if includePrefs {
		fmt.Println("✓ Reset all calibrations and preferences.")
	} else {
		fmt.Println("✓ Reset calibrations (preferences kept).")
	}
}

func runShowRules() {
	rs, err := rules.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return
	}

	fmt.Println("Base rules:")
	for name, rule := range rs.Base.Categories {
		fmt.Printf("  %s: confidence=%s, risk=%s, action=%s\n",
			name, rule.Confidence, rule.Risk, rule.DefaultAction)
	}

	if len(rs.Calibrations.Adjustments) > 0 {
		fmt.Println("\nCalibrations:")
		for _, cal := range rs.Calibrations.Adjustments {
			fmt.Printf("  %s: %s → %s (%s)\n",
				cal.Pattern, cal.Original.Action, cal.Calibrated.Action, cal.Reason)
		}
	}

	if len(rs.Preferences.AlwaysDelete) > 0 || len(rs.Preferences.NeverDelete) > 0 {
		fmt.Println("\nPreferences:")
		for _, p := range rs.Preferences.AlwaysDelete {
			fmt.Printf("  always delete: %s\n", p.Pattern)
		}
		for _, p := range rs.Preferences.NeverDelete {
			fmt.Printf("  never delete: %s\n", p.Pattern)
		}
	}
}

func runShowSessions() {
	sessions, err := session.ListSessions(10)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return
	}

	if len(sessions) == 0 {
		fmt.Println("No sessions yet.")
		return
	}

	fmt.Println("Recent sessions:")
	for _, id := range sessions {
		s, err := session.LoadSession(id)
		if err != nil {
			continue
		}
		fmt.Printf("  %s - %s (%d interactions)\n",
			s.ID, s.Tool, len(s.Interactions))
	}
}

func getToolDescription(tool string) string {
	switch tool {
	case "forge-dust":
		return "Firing up the furnace to smelt away disk clutter..."
	case "forge-habits":
		return "Examining your workflow to hammer out inefficiencies..."
	default:
		return fmt.Sprintf("Forging %s...", tool)
	}
}

func showSpinner(prefix string, done chan bool) {
	frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

	// Rotating status messages with forge personality
	statusMessages := []string{
		"Heating up the forge",
		"Stoking the flames",
		"Examining the ore",
		"Working the bellows",
		"Smelting the data",
		"Hammering out details",
		"Shaping raw findings",
		"Tempering the results",
		"Striking while hot",
		"Forging insights",
		"Refining the metal",
		"Checking the crucible",
		"Quenching the analysis",
		"Polishing the output",
		"Annealing the findings",
		"Nearly forged",
	}

	i := 0
	msgIndex := 0
	lastMsgChange := time.Now()
	msgInterval := 8 * time.Second // Change message every 8 seconds

	for {
		select {
		case <-done:
			return
		default:
			// Change status message periodically
			if time.Since(lastMsgChange) > msgInterval {
				msgIndex = (msgIndex + 1) % len(statusMessages)
				lastMsgChange = time.Now()
			}

			currentMsg := statusMessages[msgIndex]
			fmt.Printf("\r\033[K%s%s %s...%s", Cyan, frames[i%len(frames)], currentMsg, Reset)
			i++
			time.Sleep(80 * time.Millisecond)
		}
	}
}

func printHelp() {
	fmt.Printf(`forge v%s - Adaptive system optimization toolkit

Usage:
  forge <tool> [flags]     Run a tool with adaptive interaction
  forge <command>          Run a forge command

Tools:
  dust                     Disk space optimization
  habits                   Shell history analysis

Commands:
  review                   Show what forge has learned
  learn                    Force learning reflection
  always <pattern>         Always delete files matching pattern
  never <pattern>          Never delete files matching pattern
  forget <pattern>         Forget learned behavior for pattern
  reset [--all]            Reset calibrations (--all includes preferences)
  rules                    Show current ruleset
  sessions                 Show recent sessions
  help                     Show this help

Examples:
  forge dust               Run disk cleanup with adaptive guidance
  forge dust --quick       Quick mode, bias toward auto-cleanup
  forge habits             Analyze shell history
  forge review             See what behaviors have been learned
  forge always "*.dmg"     Always auto-delete .dmg files
  forge never "*.mov"      Never suggest deleting .mov files

The forge adapts to your preferences over time. Run 'forge review' to see
what it has learned, or 'forge reset' to start fresh.
`, version)
}
