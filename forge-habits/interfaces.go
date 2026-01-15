package main

import (
	"forge-habits/analyzer"
	"forge-habits/parser"
	"forge-habits/suggestions"
)

// LLMClient defines the interface for LLM operations
type LLMClient interface {
	// Generate sends a prompt to the LLM and returns the response
	Generate(prompt string) (string, error)
	// IsAvailable checks if the LLM service is accessible
	IsAvailable() bool
}

// HistoryParser defines the interface for parsing shell history
type HistoryParser interface {
	// Parse reads and parses a shell history file
	Parse(filePath string, shellType string) (*parser.HistoryData, error)
}

// ShellConfig defines the interface for shell configuration operations
type ShellConfig interface {
	// GetRCFile returns the path to the shell RC file
	GetRCFile() (string, error)
	// HasAlias checks if an alias/function already exists
	HasAlias(rcPath, name string) (bool, error)
	// AddToRC adds code entries to the RC file
	AddToRC(rcPath string, entries []string) error
	// Backup creates a backup of the RC file
	Backup(rcPath string) (string, error)
}

// SuggestionGenerator defines the interface for generating suggestions
type SuggestionGenerator interface {
	// Generate creates suggestions using LLM
	Generate(analysis *analyzer.Analysis, client LLMClient) *suggestions.SuggestionSet
	// GenerateWithoutLLM creates suggestions using heuristics
	GenerateWithoutLLM(analysis *analyzer.Analysis) *suggestions.SuggestionSet
}

// Analyzer defines the interface for analyzing shell history
type Analyzer interface {
	// Analyze performs statistical analysis on history data
	Analyze(data *parser.HistoryData) *analyzer.Analysis
}
