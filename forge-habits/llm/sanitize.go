package llm

import (
	"regexp"
	"strings"
)

// Sensitive data patterns that should be redacted before sending to LLM
var sensitivePatterns = []*regexp.Regexp{
	// API keys and tokens
	regexp.MustCompile(`(?i)(api[_-]?key|apikey|api_token|access_token|auth_token|bearer)\s*[=:]\s*['"]?[a-zA-Z0-9_\-\.]{16,}['"]?`),
	regexp.MustCompile(`(?i)bearer\s+[a-zA-Z0-9_\-\.]+`),
	regexp.MustCompile(`(?i)(sk|pk|api|key|token)[_-][a-zA-Z0-9]{20,}`),

	// Passwords
	regexp.MustCompile(`(?i)(password|passwd|pwd|secret)\s*[=:]\s*['"]?[^\s'"]{4,}['"]?`),
	regexp.MustCompile(`(?i)-p\s*['"]?[^\s'"]{4,}['"]?`),                        // mysql -p password
	regexp.MustCompile(`(?i)--password[=\s]+['"]?[^\s'"]+['"]?`),                // --password=xxx

	// URLs with credentials
	regexp.MustCompile(`https?://[^:]+:[^@]+@[^\s]+`),

	// AWS credentials
	regexp.MustCompile(`(?i)aws[_-]?(access[_-]?key[_-]?id|secret[_-]?access[_-]?key)\s*[=:]\s*['"]?[A-Za-z0-9/+=]{16,}['"]?`),
	regexp.MustCompile(`AKIA[0-9A-Z]{16}`),

	// GitHub tokens
	regexp.MustCompile(`ghp_[a-zA-Z0-9]{36}`),
	regexp.MustCompile(`gho_[a-zA-Z0-9]{36}`),
	regexp.MustCompile(`ghu_[a-zA-Z0-9]{36}`),
	regexp.MustCompile(`ghs_[a-zA-Z0-9]{36}`),
	regexp.MustCompile(`ghr_[a-zA-Z0-9]{36}`),

	// OpenAI keys
	regexp.MustCompile(`sk-[a-zA-Z0-9]{20,}`),

	// SSH keys and paths
	regexp.MustCompile(`(?i)-----BEGIN\s+(RSA|DSA|EC|OPENSSH)\s+PRIVATE\s+KEY-----`),
	regexp.MustCompile(`(?i)-i\s+[~\/][^\s]+`), // ssh -i /path/to/key

	// Database connection strings
	regexp.MustCompile(`(?i)(mysql|postgres|mongodb|redis)://[^\s]+`),

	// Environment variable exports with sensitive values
	regexp.MustCompile(`(?i)export\s+(PASSWORD|SECRET|TOKEN|KEY|API_KEY|AWS_)\w*\s*=\s*['"]?[^\s'"]+['"]?`),

	// Long hex strings (potential secrets)
	regexp.MustCompile(`[a-fA-F0-9]{32,}`),
}

// SanitizeCommand removes sensitive data from a command string
func SanitizeCommand(cmd string) string {
	result := cmd
	for _, pattern := range sensitivePatterns {
		result = pattern.ReplaceAllString(result, "[REDACTED]")
	}
	return result
}

// SanitizeCommands sanitizes a slice of command strings
func SanitizeCommands(cmds []string) []string {
	result := make([]string, len(cmds))
	for i, cmd := range cmds {
		result[i] = SanitizeCommand(cmd)
	}
	return result
}

// ContainsSensitiveData checks if a command might contain sensitive data
func ContainsSensitiveData(cmd string) bool {
	cmdLower := strings.ToLower(cmd)

	// Quick checks for common sensitive patterns
	sensitiveKeywords := []string{
		"password", "passwd", "secret", "token", "api_key", "apikey",
		"bearer", "credential", "private_key", "ssh-", "-----begin",
	}

	for _, keyword := range sensitiveKeywords {
		if strings.Contains(cmdLower, keyword) {
			return true
		}
	}

	// Check regex patterns
	for _, pattern := range sensitivePatterns {
		if pattern.MatchString(cmd) {
			return true
		}
	}

	return false
}
