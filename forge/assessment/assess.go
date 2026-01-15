package assessment

import (
	"encoding/json"
	"fmt"
	"strings"

	"forge/llm"
	"forge/rules"
)

// Mode represents the interaction mode
type Mode string

const (
	ModeAuto          Mode = "auto"          // Execute immediately
	ModeSuggest       Mode = "suggest"       // Present action, ask Y/n
	ModeGuided        Mode = "guided"        // Walk through categories
	ModeCollaborative Mode = "collaborative" // Ask questions, learn
	ModeInformative   Mode = "informative"   // Present info, no suggestions
	ModeNull          Mode = "null"          // Nothing to do
)

// Finding represents a single item found by a tool
type Finding struct {
	Category    string            `json:"category"`
	Path        string            `json:"path"`
	Size        int64             `json:"size"`
	Type        string            `json:"type"`
	AgeDays     int               `json:"age_days,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	RuleApplied *rules.MergedRule `json:"-"`
}

// CategoryAssessment is the assessment for a category of findings
type CategoryAssessment struct {
	Category    string    `json:"category"`
	Findings    []Finding `json:"findings"`
	TotalSize   int64     `json:"total_size"`
	Confidence  string    `json:"confidence"`
	Risk        string    `json:"risk"`
	Reversible  bool      `json:"reversible"`
	Mode        Mode      `json:"mode"`
	Explanation string    `json:"explanation"`
	Action      string    `json:"suggested_action"`
}

// SessionAssessment is the overall assessment for a session
type SessionAssessment struct {
	OverallMode      Mode                 `json:"overall_mode"`
	OpeningMessage   string               `json:"opening_message"`
	Categories       []CategoryAssessment `json:"categories"`
	TotalReclaimable int64                `json:"total_reclaimable"`
	Flags            []string             `json:"flags_detected"`
}

// ToolOutput is the expected JSON structure from forge tools
type ToolOutput struct {
	Tool        string `json:"tool"`
	Version     string `json:"version"`
	ScanSummary struct {
		TotalScanned string `json:"total_scanned"`
		TotalFiles   int    `json:"total_files"`
		ScanTimeMs   int    `json:"scan_time_ms"`
	} `json:"scan_summary"`
	Categories []struct {
		ID        string `json:"id"`
		Name      string `json:"name"`
		TotalSize int64  `json:"total_size"`
		ItemCount int    `json:"item_count"`
		Metadata  struct {
			TypicalRisk  string `json:"typical_risk"`
			Reversible   bool   `json:"reversible"`
			Description  string `json:"description"`
			SafeAction   string `json:"safe_action"`
		} `json:"metadata"`
		Items []struct {
			Path    string            `json:"path"`
			Size    int64             `json:"size"`
			Type    string            `json:"type"`
			AgeDays int               `json:"age_days,omitempty"`
			Context map[string]string `json:"context,omitempty"`
		} `json:"items"`
	} `json:"categories"`
}

// Assessor determines the interaction mode based on findings
type Assessor struct {
	Rules  *rules.RuleSet
	Client *llm.OllamaClient
}

// NewAssessor creates a new assessor
func NewAssessor(rs *rules.RuleSet, client *llm.OllamaClient) *Assessor {
	return &Assessor{
		Rules:  rs,
		Client: client,
	}
}

// Assess analyzes tool output and determines interaction mode
func (a *Assessor) Assess(output *ToolOutput, flags []string) (*SessionAssessment, error) {
	assessment := &SessionAssessment{
		Flags: flags,
	}

	hasQuickFlag := contains(flags, "--quick")
	hasCarefulFlag := contains(flags, "--careful")

	// Assess each category
	for _, cat := range output.Categories {
		catAssess := CategoryAssessment{
			Category:   cat.Name,
			TotalSize:  cat.TotalSize,
			Confidence: "medium",
			Risk:       cat.Metadata.TypicalRisk,
			Reversible: cat.Metadata.Reversible,
		}

		// Apply rules to determine confidence
		for _, item := range cat.Items {
			finding := Finding{
				Category: cat.Name,
				Path:     item.Path,
				Size:     item.Size,
				Type:     item.Type,
				AgeDays:  item.AgeDays,
			}

			// Check if we have a rule for this
			rule := a.Rules.GetRuleFor(item.Path)
			if rule != nil {
				finding.RuleApplied = rule
				catAssess.Confidence = rule.EffectiveConf
			}

			catAssess.Findings = append(catAssess.Findings, finding)
		}

		// Determine mode for this category
		catAssess.Mode = determineMode(catAssess.Confidence, catAssess.Risk, catAssess.Reversible)

		// Override with flags
		if hasQuickFlag {
			catAssess.Mode = biasTowandAuto(catAssess.Mode)
		}
		if hasCarefulFlag {
			catAssess.Mode = biasTowardCareful(catAssess.Mode)
		}

		catAssess.Explanation = generateExplanation(catAssess)
		catAssess.Action = suggestAction(catAssess)

		assessment.Categories = append(assessment.Categories, catAssess)
		assessment.TotalReclaimable += cat.TotalSize
	}

	// Determine overall session mode
	assessment.OverallMode = aggregateMode(assessment.Categories)
	assessment.OpeningMessage = generateOpeningMessage(assessment)

	return assessment, nil
}

// AssessWithLLM uses the LLM for more nuanced assessment
func (a *Assessor) AssessWithLLM(output *ToolOutput, flags []string) (*SessionAssessment, error) {
	// First do rule-based assessment
	assessment, err := a.Assess(output, flags)
	if err != nil {
		return nil, err
	}

	// If we have mixed or complex findings, consult LLM
	if assessment.OverallMode == ModeGuided || assessment.OverallMode == ModeCollaborative {
		llmAssessment, err := a.getLLMAssessment(output, assessment)
		if err == nil && llmAssessment != "" {
			// Parse LLM response and enhance assessment
			assessment.OpeningMessage = llmAssessment
		}
	}

	return assessment, nil
}

func (a *Assessor) getLLMAssessment(output *ToolOutput, initial *SessionAssessment) (string, error) {
	prompt := buildAssessmentPrompt(output, initial)
	return a.Client.Generate(prompt)
}

func buildAssessmentPrompt(output *ToolOutput, initial *SessionAssessment) string {
	var sb strings.Builder

	sb.WriteString(`You are the Forge assistant, helping a user clean up their disk.

FINDINGS:
`)

	for _, cat := range output.Categories {
		sb.WriteString(fmt.Sprintf("\n%s (%d items, %d bytes):\n",
			cat.Name, cat.ItemCount, cat.TotalSize))
		sb.WriteString(fmt.Sprintf("  Risk: %s, Reversible: %v\n",
			cat.Metadata.TypicalRisk, cat.Metadata.Reversible))
	}

	sb.WriteString(`
INITIAL ASSESSMENT:
`)
	sb.WriteString(fmt.Sprintf("Overall mode: %s\n", initial.OverallMode))

	sb.WriteString(`
TASK:
Write a brief, friendly opening message (2-3 sentences) that:
1. Summarizes what was found
2. Sets expectations for the interaction style
3. Makes the user feel in control

Be concise. No markdown formatting.
`)

	return sb.String()
}

func determineMode(confidence, risk string, reversible bool) Mode {
	// High confidence + low risk = more automatic
	// Low confidence + high risk = more careful

	confScore := confidenceScore(confidence)
	riskScore := riskScore(risk)

	if riskScore == 3 { // high risk
		if confScore >= 2 {
			return ModeGuided
		}
		return ModeInformative
	}

	if riskScore == 2 { // medium risk
		if confScore == 3 {
			return ModeSuggest
		}
		if confScore == 2 {
			return ModeGuided
		}
		return ModeCollaborative
	}

	// low risk
	if confScore >= 2 && reversible {
		return ModeSuggest
	}
	if confScore == 3 {
		return ModeAuto
	}

	return ModeGuided
}

func confidenceScore(conf string) int {
	switch conf {
	case "very_high":
		return 4
	case "high":
		return 3
	case "medium":
		return 2
	case "low":
		return 1
	default:
		return 2
	}
}

func riskScore(risk string) int {
	switch risk {
	case "high":
		return 3
	case "medium":
		return 2
	case "low":
		return 1
	default:
		return 2
	}
}

func aggregateMode(categories []CategoryAssessment) Mode {
	// Rule 1: Any high-risk pulls toward careful
	for _, cat := range categories {
		if cat.Risk == "high" {
			return ModeGuided
		}
	}

	// Rule 2: All same mode? Use that mode
	if len(categories) > 0 {
		firstMode := categories[0].Mode
		allSame := true
		for _, cat := range categories {
			if cat.Mode != firstMode {
				allSame = false
				break
			}
		}
		if allSame {
			return firstMode
		}
	}

	// Rule 3: Mixed = guided
	return ModeGuided
}

func biasTowandAuto(m Mode) Mode {
	switch m {
	case ModeCollaborative:
		return ModeGuided
	case ModeGuided:
		return ModeSuggest
	case ModeSuggest:
		return ModeAuto
	default:
		return m
	}
}

func biasTowardCareful(m Mode) Mode {
	switch m {
	case ModeAuto:
		return ModeSuggest
	case ModeSuggest:
		return ModeGuided
	case ModeGuided:
		return ModeCollaborative
	default:
		return m
	}
}

func generateExplanation(cat CategoryAssessment) string {
	if cat.Reversible {
		return fmt.Sprintf("%s - these can be rebuilt if needed", cat.Category)
	}
	return cat.Category
}

func suggestAction(cat CategoryAssessment) string {
	switch cat.Mode {
	case ModeAuto:
		return "auto_delete"
	case ModeSuggest:
		return "suggest_delete"
	case ModeGuided:
		return "walk_through"
	case ModeCollaborative:
		return "discuss"
	default:
		return "inform_only"
	}
}

func generateOpeningMessage(a *SessionAssessment) string {
	switch a.OverallMode {
	case ModeAuto:
		return "Found pure slag ready to burn off."
	case ModeSuggest:
		return fmt.Sprintf("Found %s of raw material that could be smelted down.",
			formatBytes(a.TotalReclaimable))
	case ModeGuided:
		return fmt.Sprintf("Found %d ore deposits to inspect. Let's examine each one.",
			len(a.Categories))
	case ModeCollaborative:
		return "Found some unusual materials. Best we look at these together before firing up the furnace."
	case ModeInformative:
		return "Laid out the findings on the anvil. The hammer's yours."
	default:
		return "Forge inspection complete. The workshop is clean."
	}
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

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// ParseToolOutput parses JSON from a tool
func ParseToolOutput(data []byte) (*ToolOutput, error) {
	var output ToolOutput
	if err := json.Unmarshal(data, &output); err != nil {
		return nil, err
	}
	return &output, nil
}
