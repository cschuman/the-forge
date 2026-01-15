# The Forge ⚒

Adaptive system optimization tools that read the room.

The Forge is a collection of CLI utilities that help you optimize your system—cleaning disk space, analyzing habits, and more. What makes it different: **it adapts to you**. Instead of dumping reports and walking away, The Forge assesses confidence and risk, then chooses how to interact: auto-executing safe cleanups, guiding you through uncertain decisions, or stepping back when you know best.

## Tools

### `forge dust`
Disk space analyzer that finds cache directories, large files, old downloads, and forgotten clutter.

```bash
forge dust              # Scan home directory
forge dust --quick      # Fast scan, skip hidden dirs
forge dust --no-llm     # Skip AI recommendations
```

### `forge habits`
Shell history analyzer that finds repetitive commands, suggests aliases, and spots workflow inefficiencies.

```bash
forge habits            # Analyze shell history
forge habits --no-llm   # Skip AI recommendations
```

## The Adaptive Philosophy

Most tools blast you with information and leave. The Forge reads the room:

| Confidence | Risk | Behavior |
|------------|------|----------|
| High | Low | Auto-execute (burn off the slag) |
| High | Medium | Suggest with Y/n confirmation |
| Medium | Medium | Guide through each category |
| Low | High | Discuss before touching anything |

Over time, The Forge learns your preferences. Always skip `.mov` files? It remembers. Always delete `node_modules`? It'll stop asking.

```bash
forge always "*.dmg"    # Always auto-delete DMG files
forge never "*.mov"     # Never suggest deleting videos
forge review            # See what's been learned
forge reset             # Start fresh
```

## Installation

Requires Go 1.21+ and [Ollama](https://ollama.ai) for AI features.

```bash
# Clone and build
git clone https://github.com/cschuman/the-forge.git
cd the-forge

# Build all tools
(cd forge-dust && go build -o forge-dust .)
(cd forge-habits && go build -o forge-habits .)
(cd forge && go build -o forge .)

# Symlink to your PATH
ln -s $(pwd)/forge/forge ~/.local/bin/forge
ln -s $(pwd)/forge-dust/forge-dust ~/.local/bin/forge-dust
ln -s $(pwd)/forge-habits/forge-habits ~/.local/bin/forge-habits
```

## Design

See [FORGE_PHILOSOPHY.md](FORGE_PHILOSOPHY.md) for the adaptive interaction model and [LEARNING_SYSTEM.md](LEARNING_SYSTEM.md) for how the self-calibrating rules work.

## Status

Early development. The forge is hot, but the metal's still being shaped.

---

*"Forged and finished."*
