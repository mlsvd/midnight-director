# midnight director

A terminal TUI for coordinating multiple AI agents (Claude by default) via tmux.

Run `midnight-director` to get a live view of all tmux sessions, launch agents, send prompts, watch them work, and pipe output between them without leaving the interface.

## Requirements

- Go 1.21+
- tmux

## Build

```bash
make build      # Linux
make build_mac  # macOS (Apple Silicon)
```

Add an alias for convenience:

```bash
# ~/.bashrc or ~/.zshrc
alias md='midnight-director'
```

## Navigation

| Key | Action |
|-----|--------|
| `↑` / `k` | move up |
| `↓` / `j` | move down |
| `PgDn` / `Ctrl+D` | page down |
| `PgUp` / `Ctrl+U` | page up |

## Sessions

| Key | Action |
|-----|--------|
| `n` | new session |
| `c` | create child session (named `parent/1`, `parent/2`, …) |
| `e` | rename session |
| `a` | annotate session (adds a note shown below the session row) |
| `→` / `l` | open action menu |
| `q` | quit (sessions keep running) |
| `Ctrl+Z` | suspend to background |

Child sessions are displayed indented under their parent. The tree is derived from session names — no state is stored.

## Interacting with sessions

| Key | Action |
|-----|--------|
| `i` / `Enter` | send input to a session waiting for text |
| `Space` | capture and display current screen content (press again to close) |
| `p` | open prompt picker |
| `g` | generate AI summary of last completed run |
| `s` | toggle auto-summarize |

### Action menu (`→`)

- **connect** — attach to the session (`M-b` to return, or `Ctrl+B D` to detach)
- **command** — run a shell command in the session
- **use prompt** — open prompt picker (same as `p`)
- **get screen** — capture current screen content
- **summarize** — generate AI summary
- **kill** — kill the session

## Prompt picker

Press `p` or choose **use prompt** from the menu to open the prompt picker.

Prompts are loaded from `~/.config/midnight-director/prompts.json`:

```json
[
  {
    "name": "review output",
    "text": "Review the following output and summarize what was done:\n\n{{from:parent}}"
  },
  {
    "name": "validate and fix",
    "text": "Here is the result:\n\n{{from:parent:30l}}\n\nCheck if {{what}} is correct. If not, fix it."
  },
  {
    "name": "continue in direction",
    "text": "Continue working on {{task}}. Focus on {{aspect}}."
  },
  {
    "name": "explain",
    "text": "Explain what this does in simple terms:\n\n{{from:parent:20l}}"
  }
]
```

### Placeholders

Placeholders inside `"text"` are filled interactively when a prompt is selected:

| Syntax | Behaviour |
|--------|-----------|
| `{{topic}}` | prompted to enter a value before sending |
| `{{from:parent}}` | auto-filled with the full parent session content |
| `{{from:parent:10l}}` | auto-filled with the last 10 lines of the parent session |
| `{{from:session-name:20l}}` | auto-filled with the last 20 lines of any named session |

`{{from:…}}` placeholders also work when sending directly with `i` - they are resolved before the text is sent.

### Safety check

Before sending, the picker checks whether the target session is waiting for input. If it is not (e.g. a shell prompt or running process is in the foreground), a warning is shown:

- **`s`** - send anyway
- **`c`** - copy to clipboard instead (tries `xclip`, `xsel`, `wl-copy`)
- **`Esc`** - go back to edit

After a successful send the picker closes and midnight-director connects to the session automatically.

## Session hierarchy

Sessions can be nested by name: `feature`, `feature/1`, `feature/1/2`, etc.

Press `c` on a session to create the next child (`feature/1`, `feature/2`, …). Children are displayed indented below their parent in the list.

Use `{{from:parent}}` in a prompt to inject the parent session's content into a child session's context, enabling review and validation workflows across sessions.

## Global tmux bindings

Registered automatically on startup:

| Binding | Action |
|---------|--------|
| `M-r` | open prompt picker for the current session (works from inside any session) |
| `M-b` | switch back to midnight-director (works from inside any session) |

## Themes

Press `t` to toggle between dark and light themes. The prompt picker matches the active theme.
