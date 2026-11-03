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
| `/` | filter sessions by name |

## Sessions

| Key | Action |
|-----|--------|
| `n` | new session |
| `o` | create child session (named `parent/1`, `parent/2`, …) |
| `e` | rename session |
| `a` | annotate session (adds a note shown below the session row) |
| `→` / `l` | open action menu |
| `←` / `h` | open custom actions menu (only if any are configured) |
| `c` | connect (attach) directly |
| `x` | kill directly (same `y`/`N` confirmation as the menu's **kill**) |
| `m` | set/change session color directly |
| `E` | set/change session emoji directly |
| `q` | quit (sessions keep running) |
| `Ctrl+Z` | suspend to background |

`c`, `x`, `m`, and `E` are shortcuts for actions also reachable through the `→` menu. `m` and `E` always reopen the color/emoji picker — even if the session already has one set — so picking a new value overrides the old one directly, with the cursor starting on the current value.

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
- **mark** / **reset mark** — set or clear a colored background pill on the session name
- **emoji** / **reset emoji** — set or clear an emoji tag shown after the session name
- **kill** — kill the session

## Filtering sessions (`/`)

Press `/` to filter the session list by name. Matching is a live, case-insensitive substring match — the list narrows down as you type, with non-matching sessions hidden entirely (not just dimmed).

- `Enter` — stop editing but keep the filter applied; `↑`/`↓`/`j`/`k` and the rest of the list's hotkeys now only move between and act on matching sessions
- `Esc` — clear the filter completely and show everything again (also works from the list itself, not just while typing)
- Pressing `/` again while a filter is active reopens the editor pre-filled with the current query, so you can refine it

While a filter is active, a banner appears right under the title — `🔎 filter: "weather" — 2 of 5 sessions  ·  Esc to clear` — showing the query, how many sessions match, and the key to clear it.

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

You can also drop standalone prompts in `~/.config/midnight-director/prompts/` as plain `.txt` or `.md` files — one prompt per file. The filename (without extension) becomes the prompt's name, and the file's contents become its text, placeholders included:

```
~/.config/midnight-director/prompts/
  review output.txt
  validate and fix.md
```

Prompts from both sources are merged and shown together in the picker.

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

## Custom actions (`←`)

Press `←` or `h` on a session to open a dropdown of custom actions — your own bash scripts, run against the focused session. The dropdown only opens if at least one action is configured.

Drop a `.sh` file in `~/.config/midnight-director/actions/`:

```
~/.config/midnight-director/actions/
  open-in-browser.sh
```

The filename (without `.sh`, dashes/underscores turned into spaces) becomes the action's label — `open-in-browser.sh` shows up as "open in browser". The script is run with `bash`; it doesn't need to be executable.

Each script receives the focused session's name, tmux `session_id`, and pane PID — both as positional args and as environment variables:

```bash
#!/bin/bash
# $1=name $2=session_id $3=pid, or use the env vars below
echo "$MD_SESSION_NAME $MD_SESSION_ID $MD_SESSION_PID"
```

| Positional | Env var |
|---|---|
| `$1` | `MD_SESSION_NAME` |
| `$2` | `MD_SESSION_ID` |
| `$3` | `MD_SESSION_PID` |

Example: a session named with a PR number opens that PR in the browser —

```bash
#!/bin/bash
url=$(python3 -c "print(f'https://github.com/org/repo/pull/{__import__('sys').argv[1]}')" "$MD_SESSION_NAME")
xdg-open "$url"   # or `open` on macOS
```

The script's result (success, or the first line of its output on failure) briefly replaces the tips line at the bottom of the screen. Actions are rescanned from disk every time the dropdown is opened, so new scripts show up without restarting midnight-director.

## Session hierarchy

Sessions can be nested by name: `feature`, `feature/1`, `feature/1/2`, etc.

Press `o` on a session to create the next child (`feature/1`, `feature/2`, …). Children are displayed indented below their parent in the list.

Use `{{from:parent}}` in a prompt to inject the parent session's content into a child session's context, enabling review and validation workflows across sessions.

## Global tmux bindings

Registered automatically on startup:

| Binding | Action |
|---------|--------|
| `M-r` | open prompt picker for the current session (works from inside any session) |
| `M-b` | switch back to midnight-director (works from inside any session) |

## Themes

Press `t` to toggle between dark and light themes. The prompt picker matches the active theme.
