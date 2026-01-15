# Forge Learning System

## Overview

The Forge toolkit implements a self-improving system where:
1. Tools execute and gather findings
2. LLM assesses and suggests actions
3. User responds (accept/reject/modify)
4. System logs the session
5. Periodic reflection analyzes patterns
6. Rules calibrate based on evidence

## The Learning Loop

```
SESSION EXECUTION
    │
    ▼
SESSION LOG (findings, suggestions, responses, outcomes)
    │
    ▼ (after N sessions)
REFLECTION LAYER (LLM analyzes patterns)
    │
    ▼
RULE EVOLUTION (calibrations.yaml updated)
    │
    ▼
NEXT SESSION (uses updated rules)
```

## Expert Panel Synthesis

### Dr. Reinforcement (ML Systems)
- Every session generates signal (accept/reject/modify)
- Store as training examples
- Rules need calibration, not retraining

### Prof. Knowledge Engineering (Ontology)
- Separate facts (objective) from heuristics (learned) from preferences (user)
- Facts are static, heuristics calibrate, preferences are personal

### Dr. Meta-Cognition (AI Self-Improvement)
- Reflective AI: act → observe → reflect → adjust
- LLM reasons ABOUT its learning, not just learns FROM data

### Admiral Ops (Production Systems)
- Changes are proposed, not auto-applied
- Require N observations before suggesting
- Keep audit trail, always allow reset

### Coach UX (Human-Centered Design)
- Transparent: show what's learned
- Controllable: user can correct mistakes
- Non-creepy: local only, explain why

## Architecture

```
~/.forge/
├── rules/
│   ├── base.yaml           # shipped defaults, immutable
│   ├── calibrations.yaml   # auto-learned adjustments
│   └── preferences.yaml    # explicit user overrides
├── sessions/
│   └── *.json              # session logs for reflection
├── learning.log            # audit trail of changes
└── config.yaml             # general settings
```

## Rule Priority

```
overrides (user explicit) > calibrations (learned) > base (defaults)
```

## Rule File Formats

### base.yaml (shipped, immutable)
```yaml
version: 1
categories:
  node_modules:
    type: cache
    patterns: ['node_modules/']
    confidence: high
    risk: low
    reversible: true
    rebuild_command: 'npm install'
    default_action: suggest_delete

  rust_target:
    type: cache
    patterns: ['target/']
    confidence: high
    risk: low
    reversible: true
    rebuild_command: 'cargo build'
    default_action: suggest_delete

  personal_media:
    type: personal
    patterns: ['*.mov', '*.mp4', '*.wav', '*.jpg', '*.png']
    locations: ['~/Documents', '~/Pictures', '~/Movies']
    confidence: low
    risk: high
    reversible: false
    default_action: inform_only

  downloads_installers:
    type: temporary
    patterns: ['*.dmg', '*.pkg']
    locations: ['~/Downloads']
    confidence: medium
    risk: low
    reversible: false  # but re-downloadable
    default_action: suggest_delete
```

### calibrations.yaml (learned)
```yaml
version: 1
last_reflection: '2024-01-15T10:30:00Z'
total_sessions: 23

adjustments:
  - id: cal_001
    pattern: '*.wav'
    location: '~/Downloads'
    original:
      confidence: medium
      action: suggest_delete
    calibrated:
      confidence: low
      action: ask_first
    evidence:
      observations: 10
      accept_rate: 0.20
      sessions: [12, 13, 14, 15, 16, 17, 18, 19, 20, 21]
    reason: "User kept 8/10 .wav files in Downloads"
    learned_at: '2024-01-15T10:30:00Z'

  - id: cal_002
    pattern: 'target/'
    location: '~/Projects/*'
    original:
      confidence: high
    calibrated:
      confidence: very_high
      action: auto_suggest
    evidence:
      observations: 47
      accept_rate: 1.00
      sessions: [1, 2, 3, ...]
    reason: "User accepted 47/47 deletions"
    learned_at: '2024-01-10T14:22:00Z'
```

### preferences.yaml (user explicit)
```yaml
version: 1

always_delete:
  - pattern: '*.dmg'
    location: '~/Downloads'
    added: '2024-01-08'
    reason: 'User said "always delete old installers"'

  - pattern: 'node_modules/'
    condition: 'project_inactive > 30d'
    added: '2024-01-12'

never_delete:
  - pattern: '*.mov'
    location: '~/Documents/Family'
    added: '2024-01-09'
    reason: 'User said "never touch family videos"'

always_ask:
  - pattern: '*.csv'
    location: '~/Projects/*'
    reason: 'User wants to review data files'

interaction_style: efficient  # efficient | thorough | minimal
auto_clean_threshold: 100MB   # auto-clean items under this size if safe
```

## Session Log Format

```json
{
  "session_id": "sess_20240115_103045",
  "tool": "forge-dust",
  "timestamp": "2024-01-15T10:30:45Z",
  "duration_ms": 45230,
  "scan_summary": {
    "total_scanned_bytes": 76543210000,
    "total_files": 274093,
    "categories_found": 4
  },
  "interactions": [
    {
      "category": "cache_directories",
      "items_presented": 15,
      "total_size": 15700000000,
      "suggestion": "delete_all",
      "confidence": "high",
      "user_response": "accept",
      "items_affected": 15,
      "bytes_freed": 15700000000
    },
    {
      "category": "downloads_media",
      "item": "recording_2024.wav",
      "size": 224000000,
      "suggestion": "delete",
      "confidence": "medium",
      "user_response": "reject",
      "user_comment": "keeping this"
    }
  ],
  "outcome": {
    "total_freed": 15700000000,
    "items_deleted": 15,
    "items_kept": 3,
    "regrets": 0,
    "user_satisfaction": null
  },
  "context": {
    "flags_used": ["--quick"],
    "time_of_day": "morning",
    "session_duration": "short"
  }
}
```

## Reflection Prompt

```
You are analyzing usage patterns for the Forge toolkit to improve its suggestions.

CURRENT RULES:
{merged view of base + calibrations + preferences}

RECENT SESSIONS:
{last N session logs}

ANALYSIS TASKS:

1. ACCEPTANCE RATES
   For each rule category, calculate:
   - Total suggestions made
   - Accepted / Rejected / Modified
   - Acceptance rate percentage

2. PATTERN DETECTION
   Look for:
   - Rules with low acceptance (< 50%) → confidence too high
   - Rules with high acceptance (> 95%) → could auto-execute
   - Contextual patterns (same type, different behavior by location/time)
   - New patterns not covered by existing rules

3. CALIBRATION PROPOSALS
   For rules needing adjustment:
   - What is the current setting?
   - What should it change to?
   - What's the evidence?
   - How confident are you?

4. NEW RULE PROPOSALS
   For patterns not in base rules:
   - What pattern did you observe?
   - What rule would you propose?
   - What's the evidence?

OUTPUT FORMAT:
{
  "analysis_summary": {
    "sessions_analyzed": N,
    "total_interactions": N,
    "overall_acceptance_rate": 0.XX
  },
  "calibrations": [
    {
      "rule_id": "existing_rule_id or null for new",
      "pattern": "*.ext",
      "location": "path pattern",
      "current_confidence": "high|medium|low",
      "proposed_confidence": "high|medium|low",
      "current_action": "action",
      "proposed_action": "action",
      "evidence": {
        "observations": N,
        "accept_rate": 0.XX,
        "reject_rate": 0.XX
      },
      "rationale": "explanation",
      "confidence_in_proposal": 0.XX
    }
  ],
  "new_rules": [
    {
      "pattern": "...",
      "proposed_settings": {...},
      "evidence": {...},
      "rationale": "..."
    }
  ],
  "insights": "Free-form observations about user behavior"
}

CONSTRAINTS:
- Only propose calibrations with >= 5 observations
- High-risk categories require >= 10 observations
- New rules require >= 7 observations of consistent behavior
- Never propose removing user preferences
- Be conservative - when uncertain, don't change
```

## Learning Triggers

| Trigger | Action |
|---------|--------|
| Session ends | Log to sessions/ |
| 10 new sessions | Auto-run reflection |
| `forge learn` | Force reflection now |
| User says "always/never" | Update preferences.yaml immediately |
| User explicitly corrects | Log correction, weight heavily in reflection |
| `forge review` | Show learned rules |
| `forge forget <pattern>` | Remove calibration |
| `forge reset` | Clear calibrations (keep preferences) |

## CLI Commands

```bash
# Run tool with learning enabled (default)
forge dust
forge habits

# Review what's been learned
forge review

# Force learning reflection now
forge learn

# Manage rules
forge always "*.dmg in ~/Downloads"
forge never "*.mov in ~/Documents/Family"
forge forget "*.wav"
forge reset                    # Clear calibrations
forge reset --all              # Clear calibrations AND preferences

# Debug/inspect
forge rules                    # Show merged ruleset
forge rules --source           # Show which file each rule comes from
forge sessions                 # List recent sessions
forge session <id>             # Show session details
```

## Privacy & Safety

1. **All local** - No data leaves the machine
2. **Audit trail** - Every change logged to learning.log
3. **Reversible** - Can reset at any time
4. **Explicit wins** - User preferences override everything
5. **Conservative** - High thresholds before behavior changes
6. **Transparent** - User can inspect all learned rules

## Success Metrics

1. **Calibration accuracy** - Do adjusted rules get higher acceptance?
2. **Time savings** - Sessions getting faster over time?
3. **Regret rate** - Any "undo" or complaints after learning?
4. **Coverage** - What % of findings are handled confidently?
