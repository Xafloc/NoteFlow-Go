# NoteFlow Development Log & TODOs

## Current Sprint - Week of 2026-05-11

### In Progress
- [ ] Begin reprioritized roadmap below (start with unit tests as foundation)

### Blocked
- [ ] Nothing currently blocked

### Up Next (prioritized, reordered 2026-05-12)
1. [ ] **Unit tests for services and handlers** — project has zero tests today; gates every refactor and underwrites the "boring/reliable" goal
2. [ ] **Full-text search across all NoteFlow-managed folders** — not just current `notes.md`; pairs naturally with the cross-project task graph
3. [ ] **`noteflow tasks` CLI subcommand** — query the global task DB from the terminal (`--due today`, `--project foo`, `--complete <id>`)
4. [ ] **Polished task views on the global tasks page** — today / this week / overdue / by project / by tag
5. [ ] **AI-agent-friendly `notes.md` convention** — document the exact format and add a thin append API (e.g. `noteflow append < message`) so Claude Code / Cursor / Aider don't have to re-serialize the file
6. [ ] **Git-awareness in `notes.md`** — surface current branch in the UI; optionally tag new notes with the branch they were authored on
7. [ ] **Lightweight inline task metadata** — `- [ ] !p1 @2026-05-20 ship release notes` syntax, preserved across the per-project ↔ global sync
8. [ ] **Export to PDF/HTML** — useful, not differentiating; ship when convenient
9. [ ] **Release hygiene** — re-tag a release once a feature lands; verify CI workflow runs end-to-end

### Deferred (do not pursue without a concrete trigger)
- [ ] **WebSocket for real-time updates** — solo-dev local tool has no multi-client sync need; file-watch + HTTP polling suffices if live-reload becomes desired. Erodes the "single binary, no moving parts" promise.
- [ ] **Plugin system architecture** — premature abstraction; once a plugin API exists it's a backwards-compat anchor forever. Build features users actually ask for instead.
- [ ] **Out of scope:** real-time multi-client collab, graph/backlink view, embedded LLM sidebar, cloud sync, mobile app, WYSIWYG editor. Each undermines the single-binary/local-first promise.

## Completed

### Week of 2026-05-11
- [x] Documented the `notes.md` schema in `docs/20260512_notes_md_schema.md` — captures the current on-disk format, the diff-friendliness invariants (§6), and the open questions (§7) that future format changes will need to resolve. Foundational for Goal 1 (committable/agent-readable) and a prerequisite for parser tests.
- [x] First test suite for the project: `internal/models/note_test.go` (11 cases) and `internal/storage/file_test.go` (9 cases) — covers header parsing, task parsing, render round-trip, render determinism, task-toggle byte-stability (§6 invariant 2), separator semantics, ordering preservation, save/load round-trip, and `EnsureDirectories`. All 20 pass against the current implementation, validating the schema doc is accurate. Project is no longer at zero tests.
- [x] Flagged one schema gap: `NewNoteFromText` accepts empty input rather than rejecting it per §3 — documented in `TestEmptyInputReturnsError`, candidate for a future parser tightening.
- [x] Goal 1 UI step: directory bar now shows the current git branch (or `@short-sha` when detached). New `internal/git` package reads `.git/HEAD` directly — no shell-out, no new dependencies, supports linked worktrees. 7 tests cover the branch / detached / worktree / not-a-repo / missing-HEAD paths. Wired through `TemplateService.RenderIndex` → `index.html`; verified live via `/tmp/noteflow-test` against this repo's `main` branch. Errors from `git.Inspect` are non-fatal: the UI just renders without the branch chip.
- [x] Goal 1 agent API: `noteflow append [--title T] [BODY...]` subcommand. Reads stdin when no body args, writes a single schema-compliant note via the existing `NoteManager.AddNote` path, prints `appended: <timestamp>[ - <title>]` to stdout. New `internal/cli` package with 5 tests covering args/stdin/precedence/empty-rejection/prepend-order. Smoke-tested end-to-end in a temp dir. This is the thin write-API Goal 1 called for — agents (Claude Code, Cursor, Aider) can append a note without re-serializing notes.md.
- [x] Goal 2 foundation: documented the cross-project task DB schema in `docs/20260512_task_db_schema.md`. Captures the current `folders` and `tasks` tables, the sync model (§4 — delete-and-rewrite, which is the root cause of unstable IDs), today's invariants (§6), and the explicit blockers for Goal 2 features (§7). §8 maps each Goal 2 feature to the schema work it needs. **Headline finding**: stable task IDs and inline metadata (due/priority/tag) are prerequisites for almost everything Goal 2 wants — `noteflow tasks` CLI, today/week views, status-line surface, bidirectional integrity. Cannot build these cleanly without addressing the sync model first.
- [x] Goal 3 reliability fix: enabled `PRAGMA foreign_keys = ON;` on the task DB connection (was a documented gap in §7 of the schema doc). The `ON DELETE CASCADE` on `tasks.folder_id` now actually fires; `RemoveFolder`'s manual DELETE-tasks-first remains as belt-and-braces. Build clean, all 32 tests still pass.
- [x] Goal 1 UI step (recent commits): the right column now shows the 5 most recent commits (short SHA · subject · date) under the active-tasks box. New `git.RecentCommits(basePath, limit)` parses `.git/logs/HEAD` directly — no exec, no object-graph walk, filters out non-commit reflog entries (checkouts, resets, merges). Handles linked worktrees and missing reflogs. Five new tests cover parsing, ordering, limit, zero-limit, and not-a-repo paths (37 total project tests). New `.commits-box` CSS uses existing theme variables (`box_background`, `tasks_border`, `text_color`) so it matches every theme automatically. Verified live: the rendered HTML in this repo shows `3952f74 · temp: Remove workflow file… · 2025-…` and four more, matching `git log --oneline -5`.
- [x] Goal 3 perf verification: benchmarks for the markdown renderer in `internal/services/renderer_test.go` — three cases (small one-liner, typical mixed note, large 10kB note). **Result on M2 Max: typical note renders in 0.18ms; the Goal 3 budget is 100ms, so we're at 555× under target.** Small notes render in 13µs; a stressed 10kB note renders in 4.6ms (still 22× under target). The regression guard `TestRender_TypicalUnderBudget` trips if any future change pushes typical-note rendering past 50ms — a built-in tripwire well before the user-visible 100ms target is at risk. Project now at 38 tests + 3 benchmarks.
- [x] Goal 2 prerequisite: inline task metadata parser (`models.ParseTaskMetadata`). Tokens `!p[0-3]` (priority), `@YYYY-MM-DD` (due), `#word` (tags) are extracted from every task line and attached to the in-memory `Task` struct without modifying the source text (file remains source of truth). Regex bug caught during testing — trailing whitespace-consumption broke adjacent tokens (`#a #b`); fixed by replacing trailing `\s` with zero-width `\b`. 6 new tests cover priority normalization, due-date validation, tag adjacency, combined tokens, and the `CleanTaskText` display helper. Schema doc §4 updated from "not implemented yet" to "parsed in-memory; not yet persisted to DB." This unblocks the read side of Goal 2's filtering features even before the DB sync model is rewritten — the data is parseable today.
- [x] Goal 2 surface: `noteflow tasks` CLI subcommand. Read-only listing of tasks across all registered folders, with filters `--done`, `--due today|week|overdue|YYYY-MM-DD`, `--priority N`, `--tag T`, `--project SUBSTR`, and `--json` for scripting. Filtering uses the in-memory metadata tokens (so it works today, no DB schema changes needed). Output is sorted priority-then-due-then-project. `services.NewDatabaseServiceAt(path)` introduced so the CLI and tests can target arbitrary DB paths (production main.go uses `DefaultDatabasePath()`). 9 new tests against a real SQLite DB cover default-open, --done, priority/due/tag/project filters, JSON, missing-DB (returns no error, "no tasks" is not a failure), and invalid --due. Smoke-tested against this developer's real `~/.config/noteflow/tasks.db` — shows actual tasks from registered folders. This is the first piece of Goal 2's "real planning surface" beyond the global tasks web page.
- [x] **Goal 2 — stable task IDs (the headline §7 blocker).** Replaced the delete-and-rewrite sync model with an upsert keyed on `(folder_id, task_hash)`. New `task_hash` column (12-char hex prefix of `sha256(content)`, with `#N` disambiguation for duplicate text within a folder); new `idx_tasks_hash` index. `last_updated` only advances when content or completion actually changes (CASE expression in the UPDATE), so it now means "task last modified" — enables "what changed this week?" queries. `addColumnIfMissing` helper provides idempotent one-shot migration for pre-2026-05-12 DBs; legacy NULL-hash rows are cleared on first new sync. 6 new tests pin the invariants: IDs stable across identical syncs, editing one task leaves siblings untouched, removing one only deletes that row, `last_updated` advances only on change, duplicate-text disambiguation, and migration idempotency. Schema doc §4 rewritten to describe the upsert model; §7 blockers list now shows the two long-cited prerequisites (stable IDs + inline metadata) both resolved. §8 roadmap mapping now shows remaining Goal 2 features as incremental product work, not architectural.
- [x] Goal 3 first handler tests: `internal/handlers/notes_test.go` plants a flag on the long-standing zero-tests gap in the handlers package. Uses Fiber's `app.Test` against an in-memory `NoteManager` rooted at `t.TempDir()` (no real config dir touched). 6 cases: empty-state GET, JSON POST round-trip (verifies actual persistence via a follow-up GET), empty-content rejection (400), malformed-JSON rejection (400), out-of-range index (404), non-integer index (400). The pattern here is the template for handler tests in `tasks.go`, `files.go`, `themes.go`, and `globaltasks.go` going forward.
- [x] **Goal 2 — bidirectional toggle (`noteflow tasks --toggle HASH`).** Closes the loop: the CLI can now flip a task's completion state, updating both the source `notes.md` (via `NoteManager.UpdateTask` so file parsing/serialization stays authoritative) and the DB row (`content`, `completed`, `last_updated`) so subsequent queries see consistent state without waiting for the next full sync. Surfaced a real bug during testing — the hash included the checkbox marker, so the second toggle of any task failed; fixed by normalizing `[ ]`/`[x]`/`[X]` to `[]` before hashing in both `TaskHashFromText` and `ComputeTaskHashes`. Stale-file detection: if the DB hash isn't in the current file (file edited externally since last sync), the toggle refuses with a clear message rather than writing to the wrong line. `ComputeTaskHashes` exported for the CLI to use the same identity scheme as the DB. 3 new tests cover round-trip toggle, unknown hash, and stale-file refusal. Caveat (documented): concurrent toggle while the server is running against the same folder leaves the server's in-memory view stale until its next sync — a file-lock would tighten this; tracked as technical debt.
- [x] Goal 3 themes tests: `internal/themes/themes_test.go` covers the static theme palette (4 tests). Guards against drift in the things that downstream CSS templating depends on: every advertised theme exists, every theme exposes the same key set (a missing key in one theme would leave a `{{.key}}` literal in rendered CSS for that theme but not others — silent bug), every color value is a syntactically valid `#rgb`/`#rrggbb` hex literal (with `code_style` allowlisted as a known non-color key), and the map key matches `Theme.Name` so consumers can look up either way.
- [x] **Goal 2 status-line surface (`noteflow tasks --status`).** Single-line summary in the form `today=N overdue=N open=N` — designed for embedding in shell prompts / tmux / status bars. Honors `--project SUBSTR` for per-repo views, and `--json` for scripting (`{"today":N,"overdue":N,"open":N}`). Returns zeros (no error) when the DB doesn't exist yet. 4 new tests cover the default format, JSON, project filtering, and the no-DB case. Smoke-tested against the real DB: `today=0 overdue=0 open=4` across this developer's actual registered folders.
- [x] **Goal 3 startup-time verification.** New `internal/services/init_bench_test.go` benchmarks the major init paths and asserts a regression budget. **Result on M2 Max: NewNoteManager = 25µs, NewDatabaseServiceAt = 126µs, total ~0.15ms — 333× under the 50ms target.** `TestStartup_UnderBudget` trips if the sum ever exceeds 30ms (a 20ms-of-headroom guard well before the user-visible 50ms target is at risk).
- [x] **Goal 2 — today/week/overdue web views.** Filter bar added to the global tasks page: All / Today / This Week / Overdue, with active-state styling that matches the theme palette (accent for the active button). Client-side filtering — the data is already JSON-encoded in `globalTasksData`, so the filter is instant with no server round-trip. Uses the same `@YYYY-MM-DD` grammar as Go's `models.ParseTaskMetadata` (with a stay-in-sync comment in the JS). A "shown of total" hint appears next to the filter buttons when a filter is active. Verified live against the real DB.
- [x] **Migration bug fix caught by live testing.** The original `migrate()` created `idx_tasks_hash` before `ALTER TABLE ADD COLUMN task_hash` had run, so opening a pre-2026-05-12 DB failed with `no such column: task_hash`. This bit the live `~/.config/noteflow/tasks.db` on first launch after the stable-IDs change. Fixed by splitting migrate into three explicit steps: create core schema → ALTER for new columns → CREATE INDEX on those columns. New regression test `TestMigrate_FromPreHashSchema` writes a legacy schema directly via `sql.Open` (bypassing the production migration path), then opens the DB through `NewDatabaseServiceAt` and asserts the migration completes, the column exists, and a follow-up sync correctly cleans up NULL-hash legacy rows.
- [x] **Goal 1 — code-snippet attachment (`+file:path#N-M`).** Mirrors the existing `+http://` archive sigil: writing `+file:cmd/main.go#10-30` in a note replaces it at save time with a fenced markdown code block containing the referenced lines, a `// path#range` header for the source link, and language detection from the extension (20+ extensions mapped: go, py, js, ts, rs, java, sql, json, yaml, etc.). Security: `resolveSnippetPath` rejects absolute paths, `..` escapes, and symlinks whose target is outside the project root (verified with a real symlink test). Non-fatal: a bad path leaves the sigil in place and logs a warning, matching the archive flow. Line-range syntax `#N` for one line, `#N-M` for inclusive range, no fragment for whole file. 9 new tests cover path sandboxing (5 cases), line slicing (7 sub-cases with EOF clamp + inverted-range rejection), end-to-end note save with sigil expansion, language detection across .py/.js/.yaml, and the two non-fatal degradation paths (path escape, missing file).
- [x] **Goal 2 — saved views.** New `task_views` table (id, unique name, JSON filters, created) plus four CLI flags: `--save-view NAME` (captures current filter set after any `--view` merge — so `--view A --save-view B` clones-with-tweaks), `--view NAME` (loads filters; CLI flags still override), `--list-views` (alphabetical), `--delete-view NAME` (idempotent — non-existent views don't error). Filters are JSON-encoded so the schema is forward-compatible (new filter fields can be added without a migration). 6 new tests cover save→apply round-trip, CLI override of view values, listing (alphabetical), delete, delete-idempotent, and apply-missing-view error message. Smoke-tested against the real DB: `save-view daily --due today` → `list-views` → `view daily` → `delete-view daily` → `list-views` (empty) all worked end-to-end. **Project total: 86 tests + 4 benchmarks across 7 packages.**

### Week of 2026-05-04 (resumed after ~8 month pause)
- [x] Restored `.github/workflows/release.yml` (was removed in 3952f74); bumped to setup-go@v5/goreleaser-action@v6 with Go 1.23
- [x] Removed tracked cruft: `go1.21.5.darwin-amd64.tar.gz` (76-byte HTML stub) and `notes.md.bak`
- [x] Bumped go.mod to Go 1.23; refreshed Fiber v2.52.0→v2.52.13, goldmark v1.6.0→v1.8.2, sqlite3 v1.14.30→v1.14.44
- [x] Verified clean build and `go vet` after dep bumps

### Week of 2025-08-14
- [x] Added comprehensive note collapse/expand functionality
- [x] Implemented hover menu with collapse controls on note headers
- [x] Created individual collapse/expand for each note with click-anywhere-to-expand
- [x] Added collapse all, expand all, and focus (collapse others) operations
- [x] Designed distinct menus for expanded vs collapsed states
- [x] Styled collapsed notes with greyed-out headers and hidden content
- [x] Fixed menu positioning and visibility issues
- [x] Updated TODO.md management rules and documentation standards
- [x] Implemented tagged release system for stable Homebrew distribution
- [x] Added comprehensive version management procedures to CLAUDE.md
- [x] Fixed version constant/tag alignment issues that caused user confusion
- [x] Validated end-to-end Homebrew installation and version reporting

### Week of 2025-08-12
- [x] Fixed binary naming consistency to noteflow-go
- [x] Updated README with latest features and improvements
- [x] Added full path tooltips and click-to-copy for folder names on global tasks page
- [x] Fixed MathJax rendering for sidebar tasks
- [x] Enhanced website archiving with comprehensive resource inlining

### Week of 2025-08-10  
- [x] Implemented website archiving for +http links
- [x] Added automatic cleanup for stale folders in global task registry
- [x] Fixed automatic port detection for multiple instances
- [x] Updated documentation for noteflow-go binary name

### Week of 2025-01-07 (Project Foundation)
- [x] Analyzed existing Python noteflow.py architecture
- [x] Researched optimal Go technology stack  
- [x] Created complete Go project structure
- [x] Implemented core note management with goldmark
- [x] Built Fiber web server with embedded assets
- [x] Added task/checkbox management system
- [x] Implemented website archiving functionality
- [x] Added theme system and persistence
- [x] Set up cross-platform build and distribution
- [x] Created Homebrew tap for distribution

## Notes

### Long-term Direction (set 2026-05-12)

NoteFlow's identity is **"a local-first developer notebook that lives next to your code"** — not a general note-taking app. Obsidian owns vault-style PKM, Notion owns collaboration, Logseq owns the outliner-graph niche. NoteFlow's natural moat is the `notes.md`-per-project model plus cross-project task roll-up — nobody else does that well. Three goals anchor the work:

1. **Be the best "notes that follow your repo" tool.** Lean into the per-folder `notes.md` model: git-friendly storage, branch awareness, AI-agent integrations (Claude Code, Cursor, Aider), and treat `notes.md` as a committable artifact alongside the code it describes. This is the moat no cloud tool can match.

2. **Make the cross-project task graph the killer feature.** Today it's a "view all tasks" page; the long-term version is a real planning surface — filters, due dates, priorities, today/this-week views, CLI access, and bidirectional integrity between per-project `notes.md` and the global SQLite index. A developer juggling five repos needs to see what's blocking them this week without leaving the filesystem.

3. **Stay boring on distribution, stack, and surface area.** No account, no cloud, no telemetry, no auto-update, no plugin marketplace, no embedded LLM. Single binary, single file, Homebrew/Scoop/`go install`/direct download only. The discipline of saying no is the feature — in 2026 this promise is getting more valuable, not less.

**Audience trade-off:** narrowing to "developer notebook" forgoes the broader note-taking market. The bet is that a focused tool 1,000 developers love beats a generic one 100,000 people install and forget. Every roadmap decision should be checked against the three goals above; if a feature doesn't advance one of them, it probably doesn't belong.

### Recent Decisions & Rollbacks
- **Formatting Issues Rollback (Aug 2025)**: Rolled back some TODO formatting changes due to rendering/display issues. Maintained core functionality while reverting problematic formatting decisions to ensure TODO.md remains readable and functional.

### Architecture Decisions
- **Web Framework**: Fiber chosen for performance and simplicity
- **Markdown Parser**: goldmark with extensions for CommonMark compliance
- **Asset Embedding**: Go 1.16+ embed package for zero-dependency distribution
- **Database**: SQLite for cross-project task tracking + file-based notes storage
- **WebSocket**: For real-time updates and multi-client synchronization
- **Build**: Standard Go toolchain + GoReleaser for cross-platform builds

### Technical Debt
- Need to implement proper file locking for concurrent access
- Website archiving needs improved error handling and timeout management
- Theme system should be more extensible

### Performance Optimizations
- Target startup time: <50ms (improvement over Python's 100ms target)
- Target memory usage: 8-10MB baseline (improvement over Python's 15MB)
- Implement efficient markdown caching for large files
- Use goroutines for non-blocking website archiving

### Known Issues & Solutions
- **Cross-project task synchronization**: Use embedded SQLite database with file watchers
- **Static asset serving**: Embed all assets in binary using Go embed package
- **MathJax integration**: Continue using client-side rendering for offline support
- **File encoding issues**: Go's UTF-8 default should resolve Python's encoding problems

### Testing Strategy
- Unit tests for core note/task management
- Integration tests for web server endpoints
- Cross-platform build verification
- Performance benchmarking against Python version
- Manual testing checklist for all UI features

### Development Priorities
1. **Phase 1**: Core functionality (notes, tasks, markdown rendering)
2. **Phase 2**: Web interface and theme system
3. **Phase 3**: Website archiving and file uploads
4. **Phase 4**: Cross-project task management
5. **Phase 5**: Distribution and packaging