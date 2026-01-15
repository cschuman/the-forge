package parser

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type Command struct {
	Raw       string
	Command   string   // First word
	Args      []string // Remaining words
	Timestamp int64    // Unix timestamp if available
}

type HistoryData struct {
	Commands  []Command
	ShellType string
	FilePath  string
}

// zsh extended history format: ": timestamp:0;command"
var zshPattern = regexp.MustCompile(`^: (\d+):\d+;(.+)$`)

// Parse reads and parses a shell history file
func Parse(filePath string, shellType string) (*HistoryData, error) {
	// Auto-detect file path if not provided
	if filePath == "" {
		filePath = detectHistoryFile(shellType)
	}

	// Auto-detect shell type if not provided
	if shellType == "" {
		shellType = detectShellType(filePath)
	}

	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var commands []Command
	scanner := bufio.NewScanner(file)

	// Increase buffer size for long lines
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		cmd := parseLine(line, shellType)
		if cmd != nil {
			commands = append(commands, *cmd)
		}
	}

	return &HistoryData{
		Commands:  commands,
		ShellType: shellType,
		FilePath:  filePath,
	}, nil
}

func parseLine(line string, shellType string) *Command {
	if line == "" {
		return nil
	}

	var raw string
	var timestamp int64

	if shellType == "zsh" {
		matches := zshPattern.FindStringSubmatch(line)
		if matches != nil {
			// fmt.Sscanf(matches[1], "%d", &timestamp)
			raw = matches[2]
		} else {
			// Plain format (no timestamp)
			raw = line
		}
	} else {
		// Bash format - plain commands
		raw = line
	}

	if raw == "" {
		return nil
	}

	// Parse command and args
	parts := strings.Fields(raw)
	if len(parts) == 0 {
		return nil
	}

	return &Command{
		Raw:       raw,
		Command:   parts[0],
		Args:      parts[1:],
		Timestamp: timestamp,
	}
}

func detectHistoryFile(shellType string) string {
	home, _ := os.UserHomeDir()

	if shellType == "zsh" {
		return filepath.Join(home, ".zsh_history")
	}
	if shellType == "bash" {
		return filepath.Join(home, ".bash_history")
	}

	// Try zsh first, then bash
	zshPath := filepath.Join(home, ".zsh_history")
	if _, err := os.Stat(zshPath); err == nil {
		return zshPath
	}

	return filepath.Join(home, ".bash_history")
}

func detectShellType(filePath string) string {
	if strings.Contains(filePath, "zsh") {
		return "zsh"
	}
	if strings.Contains(filePath, "bash") {
		return "bash"
	}

	// Read first few lines to detect format
	file, err := os.Open(filePath)
	if err != nil {
		return "bash" // default
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for i := 0; i < 10 && scanner.Scan(); i++ {
		if zshPattern.MatchString(scanner.Text()) {
			return "zsh"
		}
	}

	return "bash"
}
