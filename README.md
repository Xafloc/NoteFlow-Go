# NoteFlow-Go

A fast, lightweight, cross-platform note-taking application with markdown support, designed to run from any folder and create a web-based interface for managing notes in a single markdown file.

## 🚀 Features

- **Markdown Note-Taking**: Live preview with MathJax support for mathematical notation
- **Search**: Press `/` to filter the current folder's notes; press `Cmd/Ctrl+Enter` or click `All folders` to search every NoteFlow folder you've ever opened
- **Task Management**: Persistent checkbox/task system with cross-folder synchronization
- **Global Task View**: Manage tasks across all NoteFlow projects from a central interface
- **CLI Access**: `noteflow-go tasks --due today`, `noteflow-go append`, status-line summaries — full surface from the terminal, no browser required
- **Inline Task Metadata**: `!p1 @2026-05-20 #tag` syntax in your markdown drives priority, due date, and tag filters
- **Code Snippet Attachment**: `+file:src/foo.go#10-25` expands at save time into a fenced code block referencing your repo
- **Git Context in UI**: The directory bar shows your current branch; a hover-revealed `commits` tab on the right edge lists the 5 most recent commits
- **Per-Section Font Scaling**: Independent `Aa−` / `Aa+` controls on the Notes, Tasks, and Links sections — perfect for full-screen on a large monitor. Persisted across sessions
- **Folder Management**: Explicit registered-folder panel on the global tasks page — add folders by path, soft-forget folders you no longer track, manual per-folder sync
- **Website Archiving**: Comprehensive resource inlining with `+http` prefix
- **Drag & Drop**: File and image uploads with automatic asset management
- **Multiple Themes**: Beautiful color schemes with persistence
- **Single File Storage**: All notes stored in `notes.md` in your working directory — diff-friendly, git-committable, schema documented
- **Zero Dependencies**: Single self-contained binary; no Go toolchain, no C compiler, no DLLs needed
- **Cross-Platform**: Works on Windows, macOS, and Linux
- **Fast Performance**: <50ms startup, <100ms note render (regression-tested via benchmarks)

## 🆚 Improvements Over Python Version

- **10x Faster Startup**: Go binary vs Python interpreter
- **Lower Memory Usage**: ~15MB vs ~50MB+ for Python version  
- **Cross-Folder Tasks**: SQLite-based task synchronization across projects
- **Single Binary**: No Python runtime or pip dependencies required
- **Better Concurrency**: Native Go routines for background sync
- **Embedded Assets**: All web assets bundled into binary

## 📦 Installation

### Homebrew
```bash
brew install xafloc/noteflow-go/noteflow
```

**Note**: Installs as `noteflow-go` to avoid conflicts with the Python version.

### Easy Installer (Recommended for Windows)
**One-click installation with automatic PATH setup:**

1. Download the installer for your platform from [GitHub Releases](https://github.com/Xafloc/NoteFlow-Go/releases):
   - Windows: `noteflow-installer_windows_amd64.exe`
   - macOS (Apple Silicon): `noteflow-installer_darwin_arm64`
   - macOS (Intel): `noteflow-installer_darwin_amd64`
   - Linux (ARM): `noteflow-installer_linux_arm64`
   - Linux (x86_64): `noteflow-installer_linux_amd64`

   **Pick the right architecture:** most Macs since 2020 and many newer Linux ARM machines need `arm64`; older Intel Macs and most x86 servers need `amd64`. On macOS/Linux run `uname -m` (`arm64` / `aarch64` → `arm64`, `x86_64` → `amd64`).

2. Run the installer:
   ```bash
   # Windows (double-click or run in PowerShell)
   .\noteflow-installer_windows_amd64.exe

   # macOS (Apple Silicon)
   chmod +x noteflow-installer_darwin_arm64
   ./noteflow-installer_darwin_arm64

   # Linux (x86_64)
   chmod +x noteflow-installer_linux_amd64
   ./noteflow-installer_linux_amd64
   ```

3. Follow the interactive prompts to choose installation directory
4. Optionally add to PATH for global access
5. Run `noteflow-go` from any directory!

**Perfect for users without admin access** - installs to user directory only.

### Direct Download
1. Download the prebuilt binary for your platform from [GitHub Releases](https://github.com/Xafloc/NoteFlow-Go/releases). Filenames follow the pattern `noteflow-go_<os>_<arch>` (`.exe` on Windows) — e.g. `noteflow-go_darwin_arm64` for Apple Silicon Macs. Each asset is the binary itself, no archive to extract.
2. Mark it executable and place it on your PATH:
   ```bash
   chmod +x noteflow-go_darwin_arm64
   mv noteflow-go_darwin_arm64 /usr/local/bin/noteflow-go
   ```
3. Run `noteflow-go` from any directory.

### Build from Source
```bash
git clone https://github.com/Xafloc/NoteFlow-Go.git
cd NoteFlow-Go
go build -o noteflow-go .
```

## 🎯 Quick Start

1. **Navigate to any project folder**
   ```bash
   cd ~/my-project
   ```

2. **Start NoteFlow-Go**
   ```bash
   noteflow-go
   ```
   Server starts on `http://localhost:8000` (or the next free port) and auto-opens your default browser. Pass `--no-browser` to suppress that for headless / SSH use.

3. **Create notes and tasks**
   - Write markdown; `- [ ]` lines become tasks
   - Tag tasks with `!p1 @2026-05-20 #release` for priority / due date / tag
   - `+http://example.com` archives the page locally on save
   - `+file:src/foo.go#10-25` embeds that code block from your repo
   - Drag & drop any file for uploads

4. **Discover the rest**
   ```bash
   noteflow-go --help          # top-level usage
   noteflow-go tasks --help    # full task-query surface
   noteflow-go append --help   # write-API for agents and scripts
   ```

## ⌨️ CLI Reference

Every NoteFlow capability is reachable from the terminal — no browser required.

| Command | What it does |
|---------|--------------|
| `noteflow-go` | Start the web server in the current folder (auto-opens browser) |
| `noteflow-go --no-browser` | Same, but don't open a browser tab — for headless / SSH / status bar use |
| `noteflow-go --version` / `-v` | Print version and exit |
| `noteflow-go --help` / `-h` | Top-level help |
| `noteflow-go append [BODY]` | Append a note to `notes.md` in the current directory — thin write-API for AI coding agents (Claude Code, Cursor, Aider) and shell scripts. Body comes from args or stdin |
| `noteflow-go tasks` | List open tasks across every NoteFlow folder you've opened |
| `noteflow-go tasks --due today` | Filter — also `week`, `overdue`, or a literal `YYYY-MM-DD` |
| `noteflow-go tasks --priority 1` | Filter by priority `1..3` (matching `!p1`..`!p3` in markdown) |
| `noteflow-go tasks --tag release` | Filter by `#release` (no leading `#`) |
| `noteflow-go tasks --project repo-name` | Filter by project-path substring |
| `noteflow-go tasks --status` | One-line summary `today=N overdue=N open=N` for shell prompts / tmux / status bars |
| `noteflow-go tasks --toggle HASH` | Flip a task's completion state — updates both `notes.md` and the task DB |
| `noteflow-go tasks --save-view NAME …` | Save the current filter combination as a named view |
| `noteflow-go tasks --view NAME` | Apply a saved view's filters (CLI overrides) |
| `noteflow-go tasks --json` | JSON output for scripting (composes with any filter) |

Run any subcommand with `--help` for the full flag set and worked examples.

**Example: shell-prompt integration.** Drop this into your `.zshrc` / `.bashrc` to show pending work counts in your prompt:

```bash
nf_status() {
  noteflow-go tasks --status 2>/dev/null
}
PROMPT='$(nf_status) %~ %# '
```

## 🔍 Search

Press <kbd>/</kbd> anywhere on the page (when not typing in the editor) to open a sticky search bar at the top.

- **Local search**: as you type, notes whose title or content contains the query are kept visible; the rest are hidden. Matches inside visible notes are highlighted in your theme's accent-yellow. A counter shows `N notes · M matches`.
- **Navigate matches**: <kbd>↑</kbd>/<kbd>↓</kbd> in the search input jumps between matches; the current one gets an extra accent outline.
- **Global search**: press <kbd>⌘+Enter</kbd> (Mac) / <kbd>Ctrl+Enter</kbd> (Windows/Linux), or click the **All folders** button. A modal lists every matching note across every registered NoteFlow folder, with a snippet of context around the first match per note. Click any folder path to copy it to your clipboard — a `✓ Copied to clipboard` flash confirms.
- **Close**: <kbd>Esc</kbd> clears the filter and closes the bar.

**Why note-level filtering, not line-level?** A note is the smallest unit that's reliably self-contained. Filtering by line would strip away the surrounding context that makes the match useful. Highlighting tells the eye where the match is *within* a note that's already visible.

## 🌐 Global Task Management

NoteFlow-Go introduces **cross-folder task synchronization**:

- **Local View**: See tasks for current project folder
- **Global View**: Access `/global-tasks` to see all tasks across all registered folders
- **Two-Way Sync**: Complete tasks from either view
- **Automatic Registration**: Each NoteFlow instance auto-registers its folder on first launch
- **Background Sync**: Tasks stay synchronized across all projects (30s tick)
- **Path Navigation**: Hover over folder names to see full paths, click to copy to clipboard

### Registered Folders panel

Every folder you've ever launched `noteflow-go` in is tracked in the global task DB. The **Registered Folders** panel at the top of `/global-tasks` shows the full list with open/done counts and last-synced time, plus three actions per row:

- **Sync** — re-scan that folder's `notes.md` immediately (useful when you've edited it externally)
- **Forget** — stop tracking the folder. Confirmation required; the row stays in the DB with `active=0` for audit, and re-adding the same path later restores its history with the same ID
- **+ Add Folder…** — register an arbitrary path you typed in. Useful for folders where you manually created or moved a `notes.md`, or read-only notes archives you want to scan

A `notes.md` is created automatically if one doesn't already exist at the path you add.

## 🖋️ Customizing the UI

Two persistent customizations beyond theme selection:

- **Themes** — `~/.config/noteflow/noteflow.json` stores your active theme; switch from the menu in the top-right
- **Per-section font scaling** — hover the `fonts` tab on the right edge of the page (above `admin`) for a panel with `Aa−` / `1.0×` / `Aa+` / `↺` controls for each of the three main sections (Notes, Tasks, Links). Each scales independently across `0.8×` to `1.6×` in `0.1` steps. Code blocks inside notes use relative units so they scale with the surrounding text. Scales persist to the same config file.

  **Keyboard shortcuts (Notes section):** `Ctrl/Cmd+Alt+=` larger, `Ctrl/Cmd+Alt+-` smaller, `Ctrl/Cmd+Alt+0` reset. (Tasks and Links via the on-screen buttons.)

- **Recent commits panel** — hover the `commits` tab on the right edge (below `admin`) to see the 5 most recent commits in the current git repo. Reads `.git/logs/HEAD` directly — no git binary needed.

## 🎨 Features in Detail

### Markdown & MathJax
```markdown
# My Research Notes

Calculate eigenvalues for matrix:
$$\lambda_{1,2} = \frac{(a+d) \pm \sqrt{(a+d)^2 - 4(ad-bc)}}{2}$$

## Tasks
- [ ] Complete problem set
- [x] Review lecture notes
```

### Inline Task Metadata

Tag tasks with priority, due date, and arbitrary tags right in the markdown — no UI to set them, no sidecar metadata file:

```markdown
- [ ] !p1 @2026-05-20 #release ship the changelog
- [ ] !p2 #docs update the README
- [x] !p3 #cleanup remove orphan cmd/noteflow
```

| Token | Meaning |
|-------|---------|
| `!p[0-3]` | Priority — 1 most urgent, 3 least; `!p0` normalized to 1 |
| `@YYYY-MM-DD` | Due date — strict 4-2-2 form; invalid dates ignored |
| `#word` | Tag — letters/digits/`_`/`-` (so `#1` isn't a tag, `#release-notes` is) |

Tokens stay in the markdown source — your file is the source of truth. The web UI, the CLI (`noteflow-go tasks --due today --priority 1 --tag release`), and the global tasks page all read them.

### Code Snippet Attachment

Reference code from your repo with the `+file:` sigil; NoteFlow expands it into a fenced code block on save:

```markdown
The schema decision lives at +file:internal/services/database.go#15-30
```

becomes, in the saved `notes.md`:

````markdown
The schema decision lives at
```go
// internal/services/database.go#15-30
func NewDatabaseServiceAt(dbPath string) (*DatabaseService, error) {
    ...
}
```
````

- Range syntax: `path` (whole file), `path#10` (one line), `path#10-25` (range)
- Language detection from extension — 20+ types (go, py, js, ts, rs, sql, yaml, …)
- **Security**: paths are sandboxed to the project root. Absolute paths, `..` escapes, and symlinks targeting outside the folder are rejected; the sigil stays in place and a warning is logged

### Git Context in the UI

If your project folder is a git repo, NoteFlow surfaces two pieces of context inline:

- **Branch chip** in the directory bar — shows the current branch, or `@short-sha` when detached
- **Recent commits panel** on the right — the 5 most recent commits with short SHA, subject, and date

Both are read directly from `.git/HEAD` and `.git/logs/HEAD` — no shell-out to `git`, no runtime dependency on a git binary. Works with linked worktrees.

### Website Archiving
```markdown
+https://example.com/article
```
Creates self-contained HTML with **comprehensive resource inlining**:
- CSS stylesheets and @import rules
- JavaScript files and dependencies
- Images, fonts, and binary assets (base64 encoded)
- Fully offline-capable archived pages

### File Uploads
Drag any file into the interface - automatically creates `assets/` folder and links.

## 🛠️ Configuration

NoteFlow stores user preferences in `~/.config/noteflow/noteflow.json`:

```json
{
  "theme": "light-blue",
  "port": 8000
}
```

## 🗃️ Directory Structure

```
your-project/
├── notes.md          # All your notes (auto-created)
├── assets/           # Uploaded files (auto-created)
│   ├── images/       # Drag & drop images
│   └── sites/        # Archived websites
└── noteflow-go        # The binary (optional)
```

## 📜 Specifications

If you're writing tooling against NoteFlow's data — an AI agent, a sync script, an alternate viewer — these are the contracts you can rely on:

- [`docs/20260512_notes_md_schema.md`](docs/20260512_notes_md_schema.md) — On-disk format for `notes.md`. Note separator, header grammar, task checkboxes, inline metadata, `+http` archive sigil, `+file:` snippet sigil. Includes the **diff-friendliness invariants** the format promises to anyone reading `notes.md` from git history.
- [`docs/20260512_task_db_schema.md`](docs/20260512_task_db_schema.md) — Cross-project task DB (`~/.config/noteflow/tasks.db`). Tables, indexes, the upsert sync model that keeps task IDs stable across syncs, and the roadmap mapping for what's shipped vs. open in the planning layer.

Both files are kept in lockstep with the code — changes to the on-disk format or DB schema land in the same commit as the doc update.

## 🔧 Development

Built with modern Go technologies:
- **[Fiber](https://gofiber.io/)** - Express.js-inspired web framework
- **[Goldmark](https://github.com/yuin/goldmark)** - CommonMark-compliant markdown parser
- **[modernc.org/sqlite](https://gitlab.com/cznic/sqlite)** - Pure-Go SQLite, no CGO required
- **Embedded Assets** - Single binary with all web resources

### Project Structure
```
noteflow-go/
├── cmd/              # Application entry points
├── internal/         # Private application code
│   ├── models/       # Data structures
│   ├── services/     # Business logic
│   └── handlers/     # HTTP handlers
├── web/              # Frontend assets
│   ├── templates/    # HTML templates
│   └── static/       # CSS, JS, fonts
└── docs/             # Documentation
```

## 📋 Roadmap

- [ ] Full-text search with highlighting (in progress)
- [ ] Plugin system for extensions
- [ ] Export to PDF/HTML
- [ ] Vim keybindings support
- [ ] WebSocket real-time updates
- [ ] Mobile-responsive improvements

## 📄 License

MIT License - see [LICENSE](LICENSE) file for details.

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests if applicable
5. Submit a pull request

## 📬 Support

- **Issues**: [GitHub Issues](https://github.com/Xafloc/NoteFlow-Go/issues)
- **Discussions**: [GitHub Discussions](https://github.com/Xafloc/NoteFlow-Go/discussions)

---

**NoteFlow-Go** - Fast, powerful note-taking for developers and power users. 