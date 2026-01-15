package llm

import (
	"strings"
	"testing"
)

func TestSanitizeCommand(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantSafe bool // true if output should NOT contain sensitive data
	}{
		{
			name:     "safe command",
			input:    "git status",
			wantSafe: true,
		},
		{
			name:     "api key in env",
			input:    "export API_KEY=sk_live_1234567890abcdef",
			wantSafe: true,
		},
		{
			name:     "password flag",
			input:    "mysql -u admin -pSecretPassword123",
			wantSafe: true,
		},
		{
			name:     "bearer token",
			input:    "curl -H 'Authorization: Bearer sk-proj-abc123def456' https://api.example.com",
			wantSafe: true,
		},
		{
			name:     "url with credentials",
			input:    "git clone https://user:password123@github.com/repo.git",
			wantSafe: true,
		},
		{
			name:     "aws key",
			input:    "export AWS_SECRET_ACCESS_KEY=wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
			wantSafe: true,
		},
		{
			name:     "github token",
			input:    "git clone https://ghp_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx@github.com/repo.git",
			wantSafe: true,
		},
		{
			name:     "openai key",
			input:    "export OPENAI_API_KEY=sk-abcdefghijklmnopqrstuvwxyz1234567890",
			wantSafe: true,
		},
		{
			name:     "database connection string",
			input:    "psql postgres://admin:secret@prod.db.com:5432/mydb",
			wantSafe: true,
		},
		{
			name:     "ssh key path",
			input:    "ssh -i ~/.ssh/production_key user@server.com",
			wantSafe: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeCommand(tt.input)

			if tt.wantSafe {
				// Check that sensitive patterns are redacted
				sensitiveStrings := []string{
					"sk_live_", "SecretPassword", "Bearer sk-", "password123",
					"wJalrXUtnFEMI", "ghp_", "sk-abcdef", "admin:secret",
					"production_key",
				}

				for _, sensitive := range sensitiveStrings {
					if strings.Contains(tt.input, sensitive) && strings.Contains(result, sensitive) {
						t.Errorf("SanitizeCommand() did not redact %q from input %q, got %q",
							sensitive, tt.input, result)
					}
				}

				// Should contain REDACTED for sensitive inputs
				if tt.input != result && !strings.Contains(result, "[REDACTED]") {
					t.Errorf("SanitizeCommand() changed input but didn't add [REDACTED], got %q", result)
				}
			}
		})
	}
}

func TestContainsSensitiveData(t *testing.T) {
	tests := []struct {
		cmd  string
		want bool
	}{
		{"git status", false},
		{"ls -la", false},
		{"export API_KEY=secret", true},
		{"mysql -p password", true},
		{"curl -H 'Authorization: Bearer token'", true},
		{"-----BEGIN RSA PRIVATE KEY-----", true},
		{"export PASSWORD=test", true},
	}

	for _, tt := range tests {
		t.Run(tt.cmd, func(t *testing.T) {
			if got := ContainsSensitiveData(tt.cmd); got != tt.want {
				t.Errorf("ContainsSensitiveData(%q) = %v, want %v", tt.cmd, got, tt.want)
			}
		})
	}
}
