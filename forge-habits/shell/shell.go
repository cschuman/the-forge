package shell

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const forgeHeader = "# === Added by forge-habits ==="
const forgeFooter = "# === End forge-habits ==="

// GetRCFile returns the path to the shell RC file
func GetRCFile() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	// Check which shell
	shell := os.Getenv("SHELL")

	if strings.Contains(shell, "zsh") {
		return filepath.Join(home, ".zshrc"), nil
	} else if strings.Contains(shell, "bash") {
		// Check for .bash_profile first (macOS preference)
		bashProfile := filepath.Join(home, ".bash_profile")
		if _, err := os.Stat(bashProfile); err == nil {
			return bashProfile, nil
		}
		return filepath.Join(home, ".bashrc"), nil
	}

	// Default to .zshrc
	return filepath.Join(home, ".zshrc"), nil
}

// HasAlias checks if an alias/function already exists in the RC file
func HasAlias(rcPath, name string) (bool, error) {
	file, err := os.Open(rcPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		// Check for alias name= or function name()
		if strings.HasPrefix(line, "alias "+name+"=") ||
			strings.HasPrefix(line, name+"()") ||
			strings.HasPrefix(line, "function "+name) {
			return true, nil
		}
	}

	return false, scanner.Err()
}

// AddToRC adds code to the shell RC file
func AddToRC(rcPath string, entries []string) error {
	if len(entries) == 0 {
		return nil
	}

	// Read existing content
	existingContent := ""
	if data, err := os.ReadFile(rcPath); err == nil {
		existingContent = string(data)
	}

	// Check if we already have a forge section
	hasForgeSection := strings.Contains(existingContent, forgeHeader)

	// Build new content
	var newSection strings.Builder
	newSection.WriteString(fmt.Sprintf("\n%s\n", forgeHeader))
	newSection.WriteString(fmt.Sprintf("# Added on %s\n\n", time.Now().Format("2006-01-02 15:04")))

	for _, entry := range entries {
		newSection.WriteString(entry)
		newSection.WriteString("\n\n")
	}

	newSection.WriteString(fmt.Sprintf("%s\n", forgeFooter))

	var finalContent string
	if hasForgeSection {
		// Replace existing forge section
		start := strings.Index(existingContent, forgeHeader)
		end := strings.Index(existingContent, forgeFooter)
		if end != -1 {
			end += len(forgeFooter)
		} else {
			end = len(existingContent)
		}

		// Get content before and after forge section
		before := existingContent[:start]
		after := ""
		if end < len(existingContent) {
			after = existingContent[end:]
		}

		// Combine old forge content with new
		oldForgeContent := existingContent[start:end]
		// Extract just the entries from old section
		oldEntries := extractForgeEntries(oldForgeContent)

		// Build combined section
		var combined strings.Builder
		combined.WriteString(fmt.Sprintf("%s\n", forgeHeader))
		combined.WriteString(fmt.Sprintf("# Updated on %s\n\n", time.Now().Format("2006-01-02 15:04")))

		// Add old entries
		for _, e := range oldEntries {
			combined.WriteString(e)
			combined.WriteString("\n\n")
		}

		// Add new entries
		for _, entry := range entries {
			combined.WriteString(entry)
			combined.WriteString("\n\n")
		}

		combined.WriteString(fmt.Sprintf("%s\n", forgeFooter))

		finalContent = before + combined.String() + after
	} else {
		// Append new section
		finalContent = existingContent + newSection.String()
	}

	// Preserve original permissions if file exists, otherwise use secure default
	var fileMode os.FileMode = 0600 // Secure default: owner read/write only
	if info, err := os.Stat(rcPath); err == nil {
		fileMode = info.Mode()
		// Warn if permissions are too permissive
		if fileMode&0077 != 0 {
			fmt.Fprintf(os.Stderr, "Warning: %s has permissive permissions (%o). Consider chmod 600.\n", rcPath, fileMode)
		}
	}

	// Write back with secure permissions
	return os.WriteFile(rcPath, []byte(finalContent), fileMode)
}

func extractForgeEntries(section string) []string {
	var entries []string
	lines := strings.Split(section, "\n")

	var current strings.Builder
	inEntry := false

	for _, line := range lines {
		// Skip header/footer/comments at start
		if strings.HasPrefix(line, "# ===") ||
			strings.HasPrefix(line, "# Added on") ||
			strings.HasPrefix(line, "# Updated on") {
			continue
		}

		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			if inEntry && current.Len() > 0 {
				entries = append(entries, strings.TrimSpace(current.String()))
				current.Reset()
				inEntry = false
			}
			continue
		}

		inEntry = true
		if current.Len() > 0 {
			current.WriteString("\n")
		}
		current.WriteString(line)
	}

	if current.Len() > 0 {
		entries = append(entries, strings.TrimSpace(current.String()))
	}

	return entries
}

// Backup creates a backup of the RC file
func Backup(rcPath string) (string, error) {
	backupPath := rcPath + ".forge-backup-" + time.Now().Format("20060102-150405")

	data, err := os.ReadFile(rcPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil // No file to backup
		}
		return "", fmt.Errorf("failed to read source file: %w", err)
	}

	// Write backup with secure permissions (owner read/write only)
	if err := os.WriteFile(backupPath, data, 0600); err != nil {
		return "", fmt.Errorf("failed to write backup: %w", err)
	}

	// Verify backup integrity
	backupData, err := os.ReadFile(backupPath)
	if err != nil {
		os.Remove(backupPath)
		return "", fmt.Errorf("failed to verify backup: %w", err)
	}

	if len(data) != len(backupData) {
		os.Remove(backupPath)
		return "", fmt.Errorf("backup verification failed: size mismatch")
	}

	return backupPath, nil
}
