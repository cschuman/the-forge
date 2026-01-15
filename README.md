# The Forge ⚒

Tools that read the room, then swing the hammer.

Most CLI tools dump ore at your feet and walk away. The Forge heats the metal, examines the grain, and helps you strike. It assesses confidence and risk, then adapts—auto-executing when the path is clear, guiding you through uncertain terrain, or stepping back when you know the craft better.

## The Workshop

### `forge dust`
Smelts away disk clutter. Finds cache slag, oversized ingots, forgotten downloads, and rusted files.

```bash
forge dust              # Survey the home directory
forge dust --quick      # Quick pass, skip the deep corners
forge dust --no-llm     # Work without the oracle
```

### `forge habits`
Examines your workflow at the anvil. Spots repetitive hammer strikes, suggests better techniques, then offers to temper them into your shell.

```bash
forge habits            # Analyze and offer to forge improvements
forge habits --report   # Just show the ore, don't swing
```

## The Smith's Philosophy

Most tools blast you with information and leave you holding raw metal. The Forge reads the room:

| Confidence | Risk | The Smith's Approach |
|------------|------|----------------------|
| High | Low | Strikes immediately (burns off the slag) |
| High | Medium | Shows the plan, awaits your nod |
| Medium | Medium | Walks you through each piece |
| Low | High | Discusses before touching the metal |

The Forge learns your preferences over time. Always skip `.mov` files? It remembers. Always melt down `node_modules`? It stops asking.

```bash
forge always "*.dmg"    # Always burn these down
forge never "*.mov"     # Never suggest these for the crucible
forge review            # See what the forge has learned
forge reset             # Cool the metal, start fresh
```

## Firing Up the Forge

Requires Go 1.21+ and [Ollama](https://ollama.ai) for the oracle's wisdom.

```bash
# Clone the workshop
git clone https://github.com/cschuman/the-forge.git
cd the-forge

# Forge all tools
(cd forge-dust && go build -o forge-dust .)
(cd forge-habits && go build -o forge-habits .)
(cd forge && go build -o forge .)

# Hang them on your PATH
ln -s $(pwd)/forge/forge ~/.local/bin/forge
ln -s $(pwd)/forge-dust/forge-dust ~/.local/bin/forge-dust
ln -s $(pwd)/forge-habits/forge-habits ~/.local/bin/forge-habits
```

## The Blueprints

See [FORGE_PHILOSOPHY.md](FORGE_PHILOSOPHY.md) for the adaptive tempering model and [LEARNING_SYSTEM.md](LEARNING_SYSTEM.md) for how the self-calibrating bellows work.

## Status

The forge is hot. The metal's taking shape. More tools warming in the coals.

---

*"Forged and finished."*
