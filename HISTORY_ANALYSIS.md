# Shell History Analysis

**Total commands analyzed:** 6,793

---

## Key Findings

### High-Value Alias Opportunities

These commands are typed frequently and are prime candidates for shortcuts:

| Current Command | Suggested Alias | Times Used |
|-----------------|-----------------|------------|
| `npm run dev` | `nd` or `dev` | 383x |
| `git status` | `gs` | 112x |
| `git pull` | `gpl` | 91x |
| `lsof -ti:8080 \| xargs kill -9` | `kill8080` | 52x |
| `npm run build` | `nb` | 50x |
| `git add .` | `ga` | 39x |
| `pwd \| pbcopy` | `cpwd` | 35x |
| `npm run cli` | `ncli` | 30x |
| `git push` | `gp` | 28x |
| `npm run dev:full` | `ndf` | 23x |
| `npm run dev:all` | `nda` | 22x |

### Port Killer - Script Candidate

You kill processes on ports constantly. A generic port killer would help:

```bash
# Current patterns:
lsof -ti:8080 | xargs kill -9   # 52x
lsof -ti:5173 | xargs kill -9   # 11x
lsof -ti:8000 | xargs kill -9   # 10x
```

**Suggested function:**
```bash
killport() { lsof -ti:$1 | xargs kill -9 2>/dev/null && echo "Killed port $1" || echo "Nothing on port $1"; }
```

### Directory Navigation Patterns

You bounce around these directories constantly:

| Directory | Times |
|-----------|-------|
| `..` (parent) | 269x |
| `Projects` | 69x |
| `apps` | 30x |
| `Projects/unsaid-primary` | 23x |
| `Downloads` | 16x |
| `Projects/unsaid-secondary` | 13x |

**Suggestions:**
- `alias proj="cd ~/Projects"`
- `alias dl="cd ~/Downloads"`
- `alias unsaid="cd ~/Projects/unsaid-primary"`

### Command Sequences - Workflow Patterns

Your most common flows:

1. **Navigate + List:** `cd → l` (479x) - You always list after changing dirs
   - Consider: `cl() { cd "$@" && l; }`

2. **Git workflow:** `git → git` (200x) - Multiple git commands in sequence
   - You're missing `git commit` from history - are you using a GUI?

3. **Dev startup:** `./scripts/dev-api.sh → npm run dev` (89x)
   - This is a script candidate: `devstart` that does both

4. **Clear + Claude:** `clear → claude` (66x)
   - Maybe: `alias cc="clear && claude"`

### Actual Typos Found

| Typo | Meant | Times |
|------|-------|-------|
| `clera` | `clear` | 4x |
| `cde` | `cd` | 2x |

Not many typos - you type accurately.

### False Positives (Not Typos)
- `python` vs `python3` - both valid
- `pip3` vs `pip` - both valid
- `npx` - intentional, different from npm
- `ncdu` - disk utility, not a typo

---

## Recommended .zshrc Additions

```bash
# === THE FORGE ALIASES ===

# NPM shortcuts (saves ~400 keystrokes)
alias nd="npm run dev"
alias nb="npm run build"
alias ns="npm start"
alias ni="npm install"
alias ncli="npm run cli"
alias nda="npm run dev:all"
alias ndf="npm run dev:full"

# Git shortcuts (saves ~250 keystrokes)
alias gs="git status"
alias gpl="git pull"
alias gp="git push"
alias ga="git add ."

# Navigation
alias proj="cd ~/Projects"
alias dl="cd ~/Downloads"

# Utilities
alias cpwd="pwd | pbcopy"

# Port killer function
killport() {
  lsof -ti:$1 | xargs kill -9 2>/dev/null && echo "Killed port $1" || echo "Nothing on port $1"
}

# Change dir and list
cl() { cd "$@" && l; }
```

---

## Scripts to Build

1. **`devstart`** - Start your full dev environment (api + frontend)
2. **`killport`** - Generic port killer (function above)
3. **History analyzer** - This analysis as a reusable tool

---

## Stats Summary

- **Most used command:** `cd` (1,115x)
- **Second most:** `npm` (704x) - you're doing a lot of Node.js
- **Third:** `l` (657x) - you list directories constantly
- **Claude CLI:** 433x - heavy user
- **Potential keystrokes saved with aliases:** ~2,000+
