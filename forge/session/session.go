package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"forge/rules"
)

// Session represents a single tool execution session
type Session struct {
	ID          string        `json:"session_id"`
	Tool        string        `json:"tool"`
	Timestamp   time.Time     `json:"timestamp"`
	DurationMs  int64         `json:"duration_ms"`
	ScanSummary ScanSummary   `json:"scan_summary"`
	Interactions []Interaction `json:"interactions"`
	Outcome     Outcome       `json:"outcome"`
	Context     Context       `json:"context"`
}

// ScanSummary contains overview stats from the tool
type ScanSummary struct {
	TotalScannedBytes int64 `json:"total_scanned_bytes"`
	TotalFiles        int   `json:"total_files"`
	CategoriesFound   int   `json:"categories_found"`
}

// Interaction records a single suggestion and user response
type Interaction struct {
	Category       string `json:"category"`
	Item           string `json:"item,omitempty"`
	ItemsPresented int    `json:"items_presented,omitempty"`
	TotalSize      int64  `json:"total_size"`
	Suggestion     string `json:"suggestion"`
	Confidence     string `json:"confidence"`
	UserResponse   string `json:"user_response"` // accept, reject, modify, skip
	UserComment    string `json:"user_comment,omitempty"`
	ItemsAffected  int    `json:"items_affected,omitempty"`
	BytesFreed     int64  `json:"bytes_freed,omitempty"`
}

// Outcome summarizes the session results
type Outcome struct {
	TotalFreed       int64 `json:"total_freed"`
	ItemsDeleted     int   `json:"items_deleted"`
	ItemsKept        int   `json:"items_kept"`
	Regrets          int   `json:"regrets"`
	UserSatisfaction *int  `json:"user_satisfaction,omitempty"` // 1-5 if asked
}

// Context provides additional session metadata
type Context struct {
	FlagsUsed       []string `json:"flags_used"`
	TimeOfDay       string   `json:"time_of_day"` // morning, afternoon, evening, night
	SessionDuration string   `json:"session_duration"` // short, medium, long
}

// NewSession creates a new session with a unique ID
func NewSession(tool string) *Session {
	now := time.Now()
	return &Session{
		ID:        fmt.Sprintf("sess_%s", now.Format("20060102_150405")),
		Tool:      tool,
		Timestamp: now,
		Context: Context{
			TimeOfDay: timeOfDay(now),
		},
	}
}

// AddInteraction records a user interaction
func (s *Session) AddInteraction(i Interaction) {
	s.Interactions = append(s.Interactions, i)
}

// Finish completes the session and calculates duration
func (s *Session) Finish() {
	s.DurationMs = time.Since(s.Timestamp).Milliseconds()
	s.Context.SessionDuration = sessionDuration(s.DurationMs)
}

// Save writes the session to disk
func (s *Session) Save() error {
	sessionsDir := filepath.Join(rules.ForgeDir(), "sessions")
	if err := os.MkdirAll(sessionsDir, 0755); err != nil {
		return err
	}

	filename := filepath.Join(sessionsDir, s.ID+".json")
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filename, data, 0644)
}

// LoadSession reads a session from disk
func LoadSession(id string) (*Session, error) {
	filename := filepath.Join(rules.ForgeDir(), "sessions", id+".json")
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var s Session
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}

	return &s, nil
}

// ListSessions returns all session IDs, newest first
func ListSessions(limit int) ([]string, error) {
	sessionsDir := filepath.Join(rules.ForgeDir(), "sessions")
	entries, err := os.ReadDir(sessionsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}

	var sessions []string
	for i := len(entries) - 1; i >= 0 && len(sessions) < limit; i-- {
		entry := entries[i]
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".json" {
			sessions = append(sessions, entry.Name()[:len(entry.Name())-5])
		}
	}

	return sessions, nil
}

// LoadRecentSessions loads the N most recent sessions
func LoadRecentSessions(n int) ([]*Session, error) {
	ids, err := ListSessions(n)
	if err != nil {
		return nil, err
	}

	var sessions []*Session
	for _, id := range ids {
		s, err := LoadSession(id)
		if err == nil {
			sessions = append(sessions, s)
		}
	}

	return sessions, nil
}

// CountSessions returns the total number of sessions
func CountSessions() int {
	sessionsDir := filepath.Join(rules.ForgeDir(), "sessions")
	entries, err := os.ReadDir(sessionsDir)
	if err != nil {
		return 0
	}
	count := 0
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".json" {
			count++
		}
	}
	return count
}

func timeOfDay(t time.Time) string {
	hour := t.Hour()
	switch {
	case hour < 6:
		return "night"
	case hour < 12:
		return "morning"
	case hour < 18:
		return "afternoon"
	default:
		return "evening"
	}
}

func sessionDuration(ms int64) string {
	switch {
	case ms < 60000:
		return "short"
	case ms < 300000:
		return "medium"
	default:
		return "long"
	}
}
