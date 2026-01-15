# The Forge Philosophy

## Core Mental Model: "Read the Room"

Every Forge tool follows a unified interaction pattern where the LLM assesses results and adapts its behavior from auto-pilot to hands-off advisor.

## The Comfort Gradient

```
AUTO ◀━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━▶ HANDS-OFF
 │                                                    │
 │  "I've got this"              "Here's info,       │
 │  "Done."                       you decide"        │
 │                                                    │
 └────────────────────────────────────────────────────┘
```

Each finding gets positioned independently, then aggregated to determine session behavior.

## Assessment Dimensions

| Dimension | Question |
|-----------|----------|
| **Confidence** | How sure am I what this is? |
| **Risk** | What's the downside of wrong action? |
| **Reversibility** | Can this be recovered/rebuilt? |
| **Effort** | Does this need human judgment? |
| **Urgency** | Is there time pressure? |

## The Assessment Matrix

```
                    LOW RISK         MEDIUM RISK        HIGH RISK
                 ┌──────────────┬────────────────┬─────────────────┐
    HIGH CONF    │  AUTO        │  SUGGEST       │  GUIDED         │
                 │  "Done."     │  "Do this?"    │  "Let me        │
                 │              │  [Y/n]         │   explain..."   │
                 ├──────────────┼────────────────┼─────────────────┤
    MED CONF     │  SUGGEST     │  GUIDED        │  COLLABORATIVE  │
                 │  "Do this?"  │  "Walk         │  "Help me       │
                 │  [Y/n]       │   through..."  │   understand.." │
                 ├──────────────┼────────────────┼─────────────────┤
    LOW CONF     │  GUIDED      │  COLLABORATIVE │  INFORMATIVE    │
                 │  "Walk       │  "Let's        │  "Here's info,  │
                 │   through.." │   discuss..."  │   you decide"   │
                 └──────────────┴────────────────┴─────────────────┘
```

## Interaction Modes

### 1. AUTO - "I've got this"
- **Conditions**: High confidence, low risk, clearly reversible
- **Behavior**: Execute immediately, report after
- **Example**: "Cleared 847MB of Homebrew cache ✓"

### 2. SUGGEST - "Here's what I'd do"
- **Conditions**: High confidence, low-medium risk
- **Behavior**: Present action, ask for single confirmation
- **Example**: "Found 6GB of node_modules. Clean? [Y/n]"

### 3. GUIDED - "Let me walk you through this"
- **Conditions**: Medium confidence OR mixed findings
- **Behavior**: Break into categories, explain each
- **Example**: "Found 3 types. Let's review each..."

### 4. COLLABORATIVE - "Let's figure this out together"
- **Conditions**: Lower confidence, needs human judgment
- **Behavior**: Ask questions, learn from answers, refine
- **Example**: "I found a 700MB video. Is this something you want to keep?"

### 5. INFORMATIVE - "Here's what I found, you decide"
- **Conditions**: Low confidence on action, high risk, sensitive data
- **Behavior**: Present information only, no suggestions
- **Example**: "Found files in ~/Documents/Financial. I won't suggest actions."

### 6. NULL - "All clear"
- **Conditions**: No significant findings
- **Behavior**: Brief report, exit
- **Example**: "No obvious optimization opportunities."

## Aggregation Rules

1. **Any HIGH-RISK finding pulls session cautious** - don't auto-execute anything
2. **Homogeneous findings → match that mode** - all caches? Go AUTO
3. **Mixed findings → GUIDED as default** - walk through categories
4. **User signals override** - `--quick`, `--careful`, remembered preferences

## "Reading the Room" Signals

| Signal | Detection | Implication |
|--------|-----------|-------------|
| User expertise | Technical terms, flag usage | Less/more explanation |
| Session history | Repeat user, previous choices | Remember preferences |
| Explicit flags | `--quick`, `--explain` | Direct mode override |
| Response patterns | Short answers | Speed up |
| Frustration | Repeated similar requests | Offer more help |

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│                      FORGE CLI                          │
│  $ forge dust | $ forge habits | $ forge repos          │
└─────────────────────────────────────────────────────────┘
                           │
                           ▼
┌─────────────────────────────────────────────────────────┐
│              TOOL EXECUTION LAYER                       │
│  Tools output structured JSON with findings + metadata  │
└─────────────────────────────────────────────────────────┘
                           │
                           ▼
┌─────────────────────────────────────────────────────────┐
│              ASSESSMENT LAYER (LLM)                     │
│  Evaluate each finding → determine session mode         │
└─────────────────────────────────────────────────────────┘
                           │
                           ▼
┌─────────────────────────────────────────────────────────┐
│              CONVERSATION LOOP                          │
│  Present → Await Input → Process → Execute              │
└─────────────────────────────────────────────────────────┘
```

## Tool Output Schema

```json
{
  "tool": "forge-dust",
  "categories": [
    {
      "id": "cache_directories",
      "name": "Cache Directories",
      "total_size": 15700000000,
      "metadata": {
        "typical_risk": "low",
        "reversible": true,
        "safe_action": "delete"
      },
      "items": [...]
    }
  ]
}
```

## Success Metrics

1. **Trust rate**: % of suggestions user accepts
2. **Zero regrets**: No accidental deletions
3. **Time-to-action**: Faster than manual exploration
4. **Return usage**: Users come back regularly

---

*This philosophy applies to all Forge tools: forge-dust, forge-habits, forge-repos, and future utilities.*
