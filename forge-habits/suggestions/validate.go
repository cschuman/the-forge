package suggestions

import (
	"fmt"
	"regexp"
	"strings"
)

// Validation for LLM-generated suggestions to prevent code injection

var (
	// Valid name pattern: alphanumeric, underscore, dash, 2-20 chars
	validNamePattern = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_-]{1,19}$`)

	// Dangerous patterns that could indicate malicious code
	dangerousPatterns = []string{
		"`",            // Backticks (command substitution)
		"$(curl",       // Command substitution with curl
		"$(wget",       // Command substitution with wget
		"| bash",       // Pipe to bash
		"| sh",         // Pipe to sh
		"| zsh",        // Pipe to zsh
		"; bash",       // Chain bash
		"; sh",         // Chain sh
		"eval ",        // Eval command
		"source <(",    // Process substitution
		"/dev/tcp",     // Bash TCP redirects
		"/dev/udp",     // Bash UDP redirects
		">.ssh",        // SSH key manipulation
		">~/.ssh",      // SSH key manipulation
		"nc -",         // Netcat
		"netcat",       // Netcat
		"base64 -d",    // Base64 decode (often used to hide payloads)
		"\\x",          // Hex escapes
	}

	// Patterns that are suspicious but might be legitimate
	suspiciousPatterns = []string{
		"$(",     // Command substitution (legitimate in some cases)
		"&&",     // Command chaining
		"||",     // Command chaining
		">/dev/", // Redirects to devices
	}
)

// ValidationError represents a validation failure
type ValidationError struct {
	Field   string
	Message string
	Pattern string
}

func (e *ValidationError) Error() string {
	if e.Pattern != "" {
		return fmt.Sprintf("%s: %s (found: %q)", e.Field, e.Message, e.Pattern)
	}
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// ValidateSuggestion checks if an LLM suggestion is safe to use
func ValidateSuggestion(s *LLMSuggestion) error {
	// Validate name
	if err := validateName(s.Name); err != nil {
		return err
	}

	// Validate type
	if err := validateType(s.Type); err != nil {
		return err
	}

	// Validate code is not empty
	if strings.TrimSpace(s.Code) == "" {
		return &ValidationError{Field: "code", Message: "code cannot be empty"}
	}

	// Check for dangerous patterns
	if err := validateCodeSafety(s.Code); err != nil {
		return err
	}

	// Validate code format matches type
	if err := validateCodeFormat(s.Name, s.Type, s.Code); err != nil {
		return err
	}

	return nil
}

func validateName(name string) error {
	if name == "" {
		return &ValidationError{Field: "name", Message: "name cannot be empty"}
	}

	if !validNamePattern.MatchString(name) {
		return &ValidationError{
			Field:   "name",
			Message: "name must be 2-20 alphanumeric characters, starting with letter or underscore",
			Pattern: name,
		}
	}

	// Check for shell reserved words
	reserved := []string{"if", "then", "else", "elif", "fi", "case", "esac", "for", "while",
		"until", "do", "done", "in", "function", "select", "time", "coproc"}
	nameLower := strings.ToLower(name)
	for _, r := range reserved {
		if nameLower == r {
			return &ValidationError{Field: "name", Message: "name is a shell reserved word", Pattern: name}
		}
	}

	return nil
}

func validateType(t string) error {
	if t != "alias" && t != "function" {
		return &ValidationError{
			Field:   "type",
			Message: "type must be 'alias' or 'function'",
			Pattern: t,
		}
	}
	return nil
}

func validateCodeSafety(code string) error {
	codeLower := strings.ToLower(code)

	// Check for dangerous patterns
	for _, pattern := range dangerousPatterns {
		if strings.Contains(codeLower, strings.ToLower(pattern)) {
			return &ValidationError{
				Field:   "code",
				Message: "contains potentially dangerous pattern",
				Pattern: pattern,
			}
		}
	}

	// Check for unbalanced quotes (could indicate injection)
	if !hasBalancedQuotes(code) {
		return &ValidationError{
			Field:   "code",
			Message: "unbalanced quotes detected",
		}
	}

	// Check for unbalanced braces
	if !hasBalancedBraces(code) {
		return &ValidationError{
			Field:   "code",
			Message: "unbalanced braces detected",
		}
	}

	return nil
}

func validateCodeFormat(name, sugType, code string) error {
	if sugType == "alias" {
		// Alias should start with "alias name="
		expectedPrefix := "alias " + name + "="
		if !strings.HasPrefix(code, expectedPrefix) {
			return &ValidationError{
				Field:   "code",
				Message: fmt.Sprintf("alias code must start with 'alias %s='", name),
				Pattern: code[:min(50, len(code))],
			}
		}
	} else if sugType == "function" {
		// Function should contain "name()" or "function name"
		hasFuncSyntax := strings.Contains(code, name+"()") ||
			strings.Contains(code, name+" ()") ||
			strings.Contains(code, "function "+name)
		if !hasFuncSyntax {
			return &ValidationError{
				Field:   "code",
				Message: fmt.Sprintf("function code must define '%s()' or 'function %s'", name, name),
				Pattern: code[:min(50, len(code))],
			}
		}
	}

	return nil
}

func hasBalancedQuotes(s string) bool {
	singleQuotes := 0
	doubleQuotes := 0
	escaped := false

	for _, ch := range s {
		if escaped {
			escaped = false
			continue
		}
		if ch == '\\' {
			escaped = true
			continue
		}
		if ch == '\'' {
			singleQuotes++
		}
		if ch == '"' {
			doubleQuotes++
		}
	}

	return singleQuotes%2 == 0 && doubleQuotes%2 == 0
}

func hasBalancedBraces(s string) bool {
	braceCount := 0
	parenCount := 0
	bracketCount := 0

	for _, ch := range s {
		switch ch {
		case '{':
			braceCount++
		case '}':
			braceCount--
		case '(':
			parenCount++
		case ')':
			parenCount--
		case '[':
			bracketCount++
		case ']':
			bracketCount--
		}

		// Early exit if unbalanced
		if braceCount < 0 || parenCount < 0 || bracketCount < 0 {
			return false
		}
	}

	return braceCount == 0 && parenCount == 0 && bracketCount == 0
}

// IsSuspicious checks if code contains patterns that warrant user attention
// Returns the suspicious patterns found (empty if none)
func IsSuspicious(code string) []string {
	var found []string
	codeLower := strings.ToLower(code)

	for _, pattern := range suspiciousPatterns {
		if strings.Contains(codeLower, strings.ToLower(pattern)) {
			found = append(found, pattern)
		}
	}

	return found
}
