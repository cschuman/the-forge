package rules

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Rule represents a single rule for handling findings
type Rule struct {
	ID             string   `yaml:"id,omitempty"`
	Type           string   `yaml:"type"`
	Patterns       []string `yaml:"patterns"`
	Locations      []string `yaml:"locations,omitempty"`
	Confidence     string   `yaml:"confidence"`     // very_high, high, medium, low
	Risk           string   `yaml:"risk"`           // high, medium, low
	Reversible     bool     `yaml:"reversible"`
	RebuildCommand string   `yaml:"rebuild_command,omitempty"`
	DefaultAction  string   `yaml:"default_action"` // auto_delete, suggest_delete, ask_first, inform_only
}

// Calibration represents a learned adjustment to a rule
type Calibration struct {
	ID       string `yaml:"id"`
	Pattern  string `yaml:"pattern"`
	Location string `yaml:"location,omitempty"`
	Original struct {
		Confidence string `yaml:"confidence"`
		Action     string `yaml:"action"`
	} `yaml:"original"`
	Calibrated struct {
		Confidence string `yaml:"confidence"`
		Action     string `yaml:"action"`
	} `yaml:"calibrated"`
	Evidence struct {
		Observations int     `yaml:"observations"`
		AcceptRate   float64 `yaml:"accept_rate"`
		Sessions     []int   `yaml:"sessions"`
	} `yaml:"evidence"`
	Reason    string `yaml:"reason"`
	LearnedAt string `yaml:"learned_at"`
}

// Preference represents an explicit user preference
type Preference struct {
	Pattern  string `yaml:"pattern"`
	Location string `yaml:"location,omitempty"`
	Added    string `yaml:"added"`
	Reason   string `yaml:"reason,omitempty"`
}

// BaseRules contains the shipped default rules
type BaseRules struct {
	Version    int             `yaml:"version"`
	Categories map[string]Rule `yaml:"categories"`
}

// Calibrations contains learned adjustments
type Calibrations struct {
	Version        int           `yaml:"version"`
	LastReflection string        `yaml:"last_reflection"`
	TotalSessions  int           `yaml:"total_sessions"`
	Adjustments    []Calibration `yaml:"adjustments"`
}

// Preferences contains user's explicit choices
type Preferences struct {
	Version          int          `yaml:"version"`
	AlwaysDelete     []Preference `yaml:"always_delete"`
	NeverDelete      []Preference `yaml:"never_delete"`
	AlwaysAsk        []Preference `yaml:"always_ask"`
	InteractionStyle string       `yaml:"interaction_style"` // efficient, thorough, minimal
}

// MergedRule is a rule with all calibrations and preferences applied
type MergedRule struct {
	Rule
	Source          string  // "base", "calibration", "preference"
	CalibratedConf  string  // adjusted confidence
	CalibratedAct   string  // adjusted action
	EffectiveConf   string  // final confidence after all adjustments
	EffectiveAction string  // final action after all adjustments
	IsOverridden    bool    // user has explicit preference
}

// RuleSet holds all rules from all sources
type RuleSet struct {
	Base         BaseRules
	Calibrations Calibrations
	Preferences  Preferences
	Merged       map[string]MergedRule
}

// ForgeDir returns the forge configuration directory
func ForgeDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".forge")
}

// Load reads all rule files and merges them
func Load() (*RuleSet, error) {
	rs := &RuleSet{
		Merged: make(map[string]MergedRule),
	}

	forgeDir := ForgeDir()

	// Load base rules (shipped with binary, or from file)
	baseFile := filepath.Join(forgeDir, "rules", "base.yaml")
	if data, err := os.ReadFile(baseFile); err == nil {
		yaml.Unmarshal(data, &rs.Base)
	} else {
		// Use embedded defaults
		rs.Base = defaultBaseRules()
	}

	// Load calibrations
	calFile := filepath.Join(forgeDir, "rules", "calibrations.yaml")
	if data, err := os.ReadFile(calFile); err == nil {
		yaml.Unmarshal(data, &rs.Calibrations)
	}

	// Load preferences
	prefFile := filepath.Join(forgeDir, "rules", "preferences.yaml")
	if data, err := os.ReadFile(prefFile); err == nil {
		yaml.Unmarshal(data, &rs.Preferences)
	}

	// Merge rules (preferences > calibrations > base)
	rs.merge()

	return rs, nil
}

// Save writes the calibrations and preferences to disk
func (rs *RuleSet) Save() error {
	forgeDir := ForgeDir()
	rulesDir := filepath.Join(forgeDir, "rules")

	if err := os.MkdirAll(rulesDir, 0755); err != nil {
		return err
	}

	// Save calibrations
	if len(rs.Calibrations.Adjustments) > 0 || rs.Calibrations.TotalSessions > 0 {
		calData, _ := yaml.Marshal(&rs.Calibrations)
		os.WriteFile(filepath.Join(rulesDir, "calibrations.yaml"), calData, 0644)
	}

	// Save preferences
	prefData, _ := yaml.Marshal(&rs.Preferences)
	os.WriteFile(filepath.Join(rulesDir, "preferences.yaml"), prefData, 0644)

	return nil
}

func (rs *RuleSet) merge() {
	// Start with base rules
	for name, rule := range rs.Base.Categories {
		merged := MergedRule{
			Rule:            rule,
			Source:          "base",
			EffectiveConf:   rule.Confidence,
			EffectiveAction: rule.DefaultAction,
		}
		rs.Merged[name] = merged
	}

	// Apply calibrations
	for _, cal := range rs.Calibrations.Adjustments {
		// Find matching rule and adjust
		for name, merged := range rs.Merged {
			if matchesPattern(merged.Patterns, cal.Pattern) {
				merged.CalibratedConf = cal.Calibrated.Confidence
				merged.CalibratedAct = cal.Calibrated.Action
				if cal.Calibrated.Confidence != "" {
					merged.EffectiveConf = cal.Calibrated.Confidence
				}
				if cal.Calibrated.Action != "" {
					merged.EffectiveAction = cal.Calibrated.Action
				}
				rs.Merged[name] = merged
			}
		}
	}

	// Apply preferences (override everything)
	// TODO: Apply always_delete, never_delete, always_ask preferences
}

func matchesPattern(patterns []string, pattern string) bool {
	for _, p := range patterns {
		if p == pattern {
			return true
		}
	}
	return false
}

// GetRuleFor returns the most applicable rule for a given path
func (rs *RuleSet) GetRuleFor(path string) *MergedRule {
	// Check preferences first
	for _, pref := range rs.Preferences.NeverDelete {
		if matchPath(path, pref.Pattern, pref.Location) {
			return &MergedRule{
				EffectiveAction: "never_delete",
				IsOverridden:    true,
				Source:          "preference",
			}
		}
	}

	for _, pref := range rs.Preferences.AlwaysDelete {
		if matchPath(path, pref.Pattern, pref.Location) {
			return &MergedRule{
				EffectiveAction: "auto_delete",
				IsOverridden:    true,
				Source:          "preference",
			}
		}
	}

	// Check merged rules
	for _, rule := range rs.Merged {
		for _, pattern := range rule.Patterns {
			if matchPath(path, pattern, "") {
				return &rule
			}
		}
	}

	return nil
}

func matchPath(path, pattern, location string) bool {
	// Simple pattern matching - could be enhanced
	matched, _ := filepath.Match(pattern, filepath.Base(path))
	return matched
}

func defaultBaseRules() BaseRules {
	return BaseRules{
		Version: 1,
		Categories: map[string]Rule{
			"node_modules": {
				Type:           "cache",
				Patterns:       []string{"node_modules"},
				Confidence:     "high",
				Risk:           "low",
				Reversible:     true,
				RebuildCommand: "npm install",
				DefaultAction:  "suggest_delete",
			},
			"rust_target": {
				Type:           "cache",
				Patterns:       []string{"target"},
				Confidence:     "high",
				Risk:           "low",
				Reversible:     true,
				RebuildCommand: "cargo build",
				DefaultAction:  "suggest_delete",
			},
			"xcode_derived": {
				Type:           "cache",
				Patterns:       []string{"DerivedData"},
				Confidence:     "high",
				Risk:           "low",
				Reversible:     true,
				DefaultAction:  "suggest_delete",
			},
			"homebrew_cache": {
				Type:           "cache",
				Patterns:       []string{"Homebrew/downloads"},
				Confidence:     "high",
				Risk:           "low",
				Reversible:     true,
				RebuildCommand: "brew fetch",
				DefaultAction:  "suggest_delete",
			},
			"python_cache": {
				Type:           "cache",
				Patterns:       []string{"__pycache__", ".pytest_cache", ".mypy_cache"},
				Confidence:     "high",
				Risk:           "low",
				Reversible:     true,
				DefaultAction:  "suggest_delete",
			},
			"installers": {
				Type:          "temporary",
				Patterns:      []string{"*.dmg", "*.pkg"},
				Locations:     []string{"~/Downloads"},
				Confidence:    "medium",
				Risk:          "low",
				Reversible:    false,
				DefaultAction: "suggest_delete",
			},
			"personal_media": {
				Type:          "personal",
				Patterns:      []string{"*.mov", "*.mp4", "*.wav", "*.jpg", "*.png"},
				Locations:     []string{"~/Documents", "~/Pictures", "~/Movies"},
				Confidence:    "low",
				Risk:          "high",
				Reversible:    false,
				DefaultAction: "inform_only",
			},
		},
	}
}
