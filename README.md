# midnight director

A terminal TUI for coordinating multiple AI agents (Claude by default) via tmux.

Run `mindight director` to get a live view of all tmux sessions to launch agents, send prompts, watch them work, and pipe output between them without leaving the interface.

## Requirements

- Go 1.21+
- tmux

## Build

```bash
make build      # Linux
make build_mac  # macOS (Apple Silicon)
```


| Key | Action |
|-----|--------|
| `n` | new session |
| `↑` / `↓` | navigate |
| `→` | open menu (command / get screen / connect / kill) |
| `i` | send input to waiting session |
| `e` / `a` | rename / annotate session |
| `q` | quit (sessions keep running) |
