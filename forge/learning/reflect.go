package learning

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"forge/llm"
	"forge/rules"
	"forge/session"
)

// ReflectionResult contains the output of a learning reflection
type ReflectionResult struct {
	AnalysisSummary struct {
		SessionsAnalyzed      int     `json:"sessions_analyzed"`
		TotalInteractions     int     `json:"total_interactions"`
		OverallAcceptanceRate float64 `json:"overall_acceptance_rate"`
	} `json:"analysis_summary"`
	Calibrations []ProposedCalibration `json:"calibrations"`
	NewRules     []ProposedRule        `json:"new_rules"`
	Insights     string                `json:"insights"`
}

// ProposedCalibration is a suggested adjustment to a rule
type ProposedCalibration struct {
	RuleID             string  `json:"rule_id"`
	Pattern            string  `json:"pattern"`
	Location           string  `json:"location,omitempty"`
	CurrentConfidence  string  `json:"current_confidence"`
	ProposedConfidence string  `json:"proposed_confidence"`
	CurrentAction      string  `json:"current_action"`
	ProposedAction     string  `json:"proposed_action"`
	Evidence           struct {
		Observations int     `json:"observations"`
		AcceptRate   float64 `json:"accept_rate"`
		RejectRate   float64 `json:"reject_rate"`
	} `json:"evidence"`
	Rationale            string  `json:"rationale"`
	ConfidenceInProposal float64 `json:"confidence_in_proposal"`
}

// ProposedRule is a suggested new rule
type ProposedRule struct {
	Pattern          string            `json:"pattern"`
	ProposedSettings map[string]string `json:"proposed_settings"`
	Evidence         struct {
		Observations int     `json:"observations"`
		AcceptRate   float64 `json:"accept_rate"`
	} `json:"evidence"`
	Rationale string `json:"rationale"`
}

// Learner handles the reflection and learning process
type Learner struct {
	Rules  *rules.RuleSet
	Client *llm.OllamaClient
}

// NewLearner creates a new learner
func NewLearner(rs *rules.RuleSet, client *llm.OllamaClient) *Learner {
	return &Learner{
		Rules:  rs,
		Client: client,
	}
}

// ShouldReflect checks if it's time for reflection
func (l *Learner) ShouldReflect() bool {
	sessionCount := session.CountSessions()
	lastReflection := l.Rules.Calibrations.TotalSessions

	// Reflect every 10 new sessions
	return sessionCount-lastReflection >= 10
}

// Reflect analyzes recent sessions and proposes calibrations
func (l *Learner) Reflect() (*ReflectionResult, error) {
	// Load recent sessions
	sessions, err := session.LoadRecentSessions(20)
	if err != nil {
		return nil, err
	}

	if len(sessions) < 5 {
		return nil, fmt.Errorf("not enough sessions for reflection (need 5, have %d)", len(sessions))
	}

	// Build prompt
	prompt := l.buildReflectionPrompt(sessions)

	// Get LLM analysis
	response, err := l.Client.Generate(prompt)
	if err != nil {
		return nil, err
	}

	// Parse response
	result, err := parseReflectionResponse(response)
	if err != nil {
		// If parsing fails, return raw insights
		return &ReflectionResult{
			Insights: response,
		}, nil
	}

	return result, nil
}

// ApplyCalibrations applies proposed calibrations that meet the threshold
func (l *Learner) ApplyCalibrations(result *ReflectionResult) ([]string, error) {
	var applied []string

	for _, cal := range result.Calibrations {
		// Only apply if confidence is high enough
		if cal.ConfidenceInProposal < 0.7 {
			continue
		}

		// Only apply if enough observations
		if cal.Evidence.Observations < 5 {
			continue
		}

		// Create calibration entry
		newCal := rules.Calibration{
			ID:       fmt.Sprintf("cal_%d", time.Now().Unix()),
			Pattern:  cal.Pattern,
			Location: cal.Location,
			Reason:   cal.Rationale,
			LearnedAt: time.Now().Format(time.RFC3339),
		}
		newCal.Original.Confidence = cal.CurrentConfidence
		newCal.Original.Action = cal.CurrentAction
		newCal.Calibrated.Confidence = cal.ProposedConfidence
		newCal.Calibrated.Action = cal.ProposedAction
		newCal.Evidence.Observations = cal.Evidence.Observations
		newCal.Evidence.AcceptRate = cal.Evidence.AcceptRate

		l.Rules.Calibrations.Adjustments = append(l.Rules.Calibrations.Adjustments, newCal)
		applied = append(applied, cal.Pattern)
	}

	// Update metadata
	l.Rules.Calibrations.TotalSessions = session.CountSessions()
	l.Rules.Calibrations.LastReflection = time.Now().Format(time.RFC3339)
	l.Rules.Calibrations.Version = 1

	// Save
	if err := l.Rules.Save(); err != nil {
		return applied, err
	}

	// Log what was learned
	logLearning(applied, result)

	return applied, nil
}

func (l *Learner) buildReflectionPrompt(sessions []*session.Session) string {
	var sb strings.Builder

	sb.WriteString(`You are analyzing usage patterns for the Forge toolkit to improve its suggestions.

CURRENT RULES:
`)

	// Add current rules
	for name, rule := range l.Rules.Base.Categories {
		sb.WriteString(fmt.Sprintf("- %s: confidence=%s, risk=%s, action=%s\n",
			name, rule.Confidence, rule.Risk, rule.DefaultAction))
	}

	sb.WriteString("\nRECENT SESSIONS:\n")

	// Add session summaries
	for _, s := range sessions {
		sb.WriteString(fmt.Sprintf("\nSession %s (%s):\n", s.ID, s.Tool))
		for _, i := range s.Interactions {
			sb.WriteString(fmt.Sprintf("  - %s: suggested=%s, response=%s\n",
				i.Category, i.Suggestion, i.UserResponse))
		}
	}

	sb.WriteString(`

ANALYSIS TASKS:

1. For each rule category, calculate acceptance rate
2. Identify rules where user behavior diverges from expectations (>20% difference)
3. Look for contextual patterns (same type, different behavior by location)
4. Propose specific calibration adjustments

OUTPUT as JSON:
{
  "analysis_summary": {
    "sessions_analyzed": N,
    "total_interactions": N,
    "overall_acceptance_rate": 0.XX
  },
  "calibrations": [
    {
      "pattern": "*.ext",
      "current_confidence": "high",
      "proposed_confidence": "medium",
      "current_action": "suggest_delete",
      "proposed_action": "ask_first",
      "evidence": {
        "observations": N,
        "accept_rate": 0.XX,
        "reject_rate": 0.XX
      },
      "rationale": "explanation",
      "confidence_in_proposal": 0.XX
    }
  ],
  "insights": "Free-form observations"
}

CONSTRAINTS:
- Only propose calibrations with >= 5 observations
- Be conservative - when uncertain, don't change
`)

	return sb.String()
}

func parseReflectionResponse(response string) (*ReflectionResult, error) {
	// Try to extract JSON from response
	start := strings.Index(response, "{")
	end := strings.LastIndex(response, "}")

	if start == -1 || end == -1 || end <= start {
		return nil, fmt.Errorf("no JSON found in response")
	}

	jsonStr := response[start : end+1]

	var result ReflectionResult
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return nil, err
	}

	return &result, nil
}

func logLearning(applied []string, result *ReflectionResult) {
	logFile := filepath.Join(rules.ForgeDir(), "learning.log")

	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()

	entry := fmt.Sprintf("\n=== %s ===\n", time.Now().Format(time.RFC3339))
	entry += fmt.Sprintf("Sessions analyzed: %d\n", result.AnalysisSummary.SessionsAnalyzed)
	entry += fmt.Sprintf("Calibrations applied: %v\n", applied)
	entry += fmt.Sprintf("Insights: %s\n", result.Insights)

	f.WriteString(entry)
}

// AddPreference adds an explicit user preference
func (l *Learner) AddPreference(prefType, pattern, location, reason string) error {
	pref := rules.Preference{
		Pattern:  pattern,
		Location: location,
		Added:    time.Now().Format("2006-01-02"),
		Reason:   reason,
	}

	switch prefType {
	case "always_delete":
		l.Rules.Preferences.AlwaysDelete = append(l.Rules.Preferences.AlwaysDelete, pref)
	case "never_delete":
		l.Rules.Preferences.NeverDelete = append(l.Rules.Preferences.NeverDelete, pref)
	case "always_ask":
		l.Rules.Preferences.AlwaysAsk = append(l.Rules.Preferences.AlwaysAsk, pref)
	default:
		return fmt.Errorf("unknown preference type: %s", prefType)
	}

	return l.Rules.Save()
}

// ForgetCalibration removes a learned calibration
func (l *Learner) ForgetCalibration(pattern string) bool {
	var remaining []rules.Calibration
	found := false

	for _, cal := range l.Rules.Calibrations.Adjustments {
		if cal.Pattern != pattern {
			remaining = append(remaining, cal)
		} else {
			found = true
		}
	}

	if found {
		l.Rules.Calibrations.Adjustments = remaining
		l.Rules.Save()
	}

	return found
}

// Reset clears all calibrations (keeps preferences)
func (l *Learner) Reset(includePreferences bool) error {
	l.Rules.Calibrations = rules.Calibrations{Version: 1}

	if includePreferences {
		l.Rules.Preferences = rules.Preferences{Version: 1}
	}

	return l.Rules.Save()
}

// GetLearningSummary returns a human-readable summary of what's been learned
func (l *Learner) GetLearningSummary() string {
	var sb strings.Builder

	sb.WriteString("⚙ FORGE LEARNING SUMMARY\n\n")

	if len(l.Rules.Calibrations.Adjustments) == 0 && len(l.Rules.Preferences.AlwaysDelete) == 0 {
		sb.WriteString("No learned behaviors yet.\n")
		sb.WriteString("Use the tools and I'll learn your preferences over time.\n")
		return sb.String()
	}

	if len(l.Rules.Calibrations.Adjustments) > 0 {
		sb.WriteString("What I've learned from your usage:\n\n")
		for _, cal := range l.Rules.Calibrations.Adjustments {
			sb.WriteString(fmt.Sprintf("✓ %s\n", cal.Pattern))
			sb.WriteString(fmt.Sprintf("  %s → %s\n", cal.Original.Action, cal.Calibrated.Action))
			sb.WriteString(fmt.Sprintf("  Reason: %s\n\n", cal.Reason))
		}
	}

	if len(l.Rules.Preferences.AlwaysDelete) > 0 {
		sb.WriteString("Your explicit preferences (always delete):\n")
		for _, pref := range l.Rules.Preferences.AlwaysDelete {
			sb.WriteString(fmt.Sprintf("  • %s", pref.Pattern))
			if pref.Location != "" {
				sb.WriteString(fmt.Sprintf(" in %s", pref.Location))
			}
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	if len(l.Rules.Preferences.NeverDelete) > 0 {
		sb.WriteString("Your explicit preferences (never delete):\n")
		for _, pref := range l.Rules.Preferences.NeverDelete {
			sb.WriteString(fmt.Sprintf("  • %s", pref.Pattern))
			if pref.Location != "" {
				sb.WriteString(fmt.Sprintf(" in %s", pref.Location))
			}
			sb.WriteString("\n")
		}
	}

	return sb.String()
}
