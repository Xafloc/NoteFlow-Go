# notes.md Schema

**Status**: documented from existing implementation as of 2026-05-12. Use this as the contract for parsers (NoteFlow itself, AI agents, external tools). Changes to the format must update this doc *and* the parser tests in lockstep.

This doc serves Goal 1 — *"be the best notes that follow your repo tool"* — by making the file format a stable, committable, agent-readable artifact rather than an implementation detail.

---

## 1. File location and encoding

- Filename: `notes.md`, in the working directory (the folder where `noteflow` was launched)
- Encoding: UTF-8, no BOM
- Line endings: LF (`\n`). Files with CRLF should still parse, but NoteFlow writes LF
- File is rewritten in full on every save (no append-only mode today — see §7 *Open questions*)

## 2. Top-level structure

A `notes.md` file is a sequence of **notes** separated by a literal separator line:

```
<note 1>
<!-- note -->
<note 2>
<!-- note -->
<note 3>
```

The exact separator emitted by NoteFlow is `\n<!-- note -->\n` (newline, separator, newline). On read, content is split on `<!-- note -->` and each chunk is trimmed.

**Ordering invariant**: notes are stored newest-first. New notes are prepended to the file. Editing a note preserves its position.

**Empty file**: a `notes.md` containing only whitespace is valid and parses to zero notes. NoteFlow creates an empty file if none exists.

## 3. Note structure

Each note is:

```
## <TIMESTAMP>[ - <TITLE>]
<BLANK LINE>
<BODY>
```

Field-by-field:

| Field       | Required | Format                                                           | Notes |
|-------------|----------|------------------------------------------------------------------|-------|
| Header `##` | yes      | Exactly `## ` (two hashes + space) at start of first line        | Any chunk between separators that does *not* start with `## ` is silently dropped on load |
| TIMESTAMP   | yes      | `YYYY-MM-DD HH:MM:SS` (24h, local time, no timezone offset)      | Regex: `^\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}$` |
| TITLE       | no       | Free text after literal ` - ` separator                          | Everything after the first ` - ` is the title; subsequent ` - ` are part of the title |
| BODY        | no       | Arbitrary CommonMark + GFM checkboxes                            | May be empty; trailing whitespace trimmed on read |

**Render template** (what NoteFlow emits):

```go
fmt.Sprintf("## %s%s\n\n%s\n", timestampStr, titleStr, content)
// where titleStr is " - <title>" when title is non-empty, else ""
```

Notes without titles render as just `## 2026-05-12 09:30:45`.

## 4. Tasks (checkboxes)

Tasks are GFM-style task list items inside a note body:

- `- [ ] unchecked task`
- `- [x] checked task` (lowercase `x` canonical; uppercase `X` accepted on read, normalized to `x` on next write)

**Indexing**: each task gets a stable, file-wide integer index assigned at load time, counting top-to-bottom (newest note first → oldest). Indices are not persisted in the file — they are derived. This means **adding a note at the top renumbers every existing task index in the runtime**, which is fine for in-memory ops but means task IDs are *not* stable across sessions or across machines. (See §7 *Open questions*.)

**Toggle semantics**: completing a task replaces `[ ]` with `[x]` (or vice versa) on the exact source line. The surrounding text is preserved byte-for-byte.

**Inline metadata** is parsed from each task line (since 2026-05-12). Tokens stay in the source — the file is the source of truth — and are extracted by `models.ParseTaskMetadata` into the in-memory `Task` struct. Three token types:

| Token form          | Meaning                  | Constraint |
|---------------------|--------------------------|-----------|
| `!p[0-3]`           | priority (1 = top, 3 = low; `!p0` normalized to 1) | preceded by whitespace or start-of-line; followed by a non-word boundary |
| `@YYYY-MM-DD`       | due date                 | preceded by whitespace or start-of-line; exact 4-2-2 digit form; invalid dates ignored |
| `#word`             | tag (multiple allowed)   | preceded by whitespace or start-of-line; `[A-Za-z_][A-Za-z0-9_-]*` so pure-numeric `#123` is not a tag |

Example:

```
- [ ] !p1 @2026-05-20 #release #docs ship the changelog
```

Parsers should ignore any token that doesn't match these shapes. **Not yet persisted to the task DB** — the metadata travels with the raw text in the `tasks.content` column until stable IDs land and dedicated columns are added (see `docs/20260512_task_db_schema.md` §7).

## 5. Archived-link sigil

If a note body contains `+http://...` or `+https://...`, NoteFlow archives the page at save time and rewrites the sigil in-place to a markdown link:

```
[<page title>](assets/sites/<YYYY_MM_DD_HHMMSS>_<title-slug>-<host-slug>.html) (archived YYYY-MM-DD HH:MM)
```

Archived files live in `assets/sites/` next to `notes.md`. Archiving failures leave the original `+http...` text untouched and log a warning — they are non-fatal.

**Delete semantics**: when an archived file is removed via the UI, any line in `notes.md` referencing the filename is rewritten to:

```
~~<original line>~~ _(archived link deleted)_
```

(Strikethrough + italic suffix.) This is the only place NoteFlow rewrites note bodies after creation outside of explicit user edits and task toggles.

## 6. Invariants (the "diff-friendliness" contract)

These are the promises NoteFlow makes to anyone reading `notes.md` in git history. Future changes to the format must not break these without a version bump:

1. **Stable identity**: a note's header line (`## TIMESTAMP[ - TITLE]`) does not change unless the user edits the title. The timestamp is set once at creation and never moves.
2. **Local edits → local diffs**: editing one note's body must not touch any other note's bytes. Task toggles must not touch any line other than the toggled task line.
3. **No renumbering artifacts**: task indices are not persisted, so `git diff` will not show spurious index changes when notes are added or reordered.
4. **Deterministic render**: given the in-memory model, `Render()` produces byte-identical output. No timestamps-of-now, no random IDs, no map iteration order leaking into the file.
5. **Round-trip safety**: `parse(render(note)) == note` for any well-formed note. This is what makes the file safe to hand-edit in an external editor and have NoteFlow pick up the result.
6. **Append-friendly separator**: `<!-- note -->` is unambiguous in markdown (HTML comment, ignored by renderers) and unlikely to collide with user content. Parsers should match it literally.

## 7. Open questions (not yet specified)

These are deliberate gaps. Code should not depend on a particular answer until decided:

- **Frontmatter**: no per-file YAML frontmatter today. If we add one (for repo metadata, schema version, agent hints), it would go above the first note and below an opening `---` fence.
- **Schema versioning**: no version marker. If the format evolves, the first plan is a `<!-- noteflow:schema=2 -->` comment at file top.
- **Stable task IDs**: indices are derived today. A future format may embed `<!-- task:abc123 -->` after each checkbox so external systems (CLI, status-line) can reference tasks across sessions. Not decided.
- **Branch awareness**: notes do not record the git branch they were authored on. A proposed extension is an optional ` @branch-name` suffix in the header, but this is unimplemented.
- **Multi-file vaults**: out of scope. NoteFlow is intentionally one file per folder. Aggregation across folders happens in the SQLite registry, not by globbing `*.md`.
- **Append API for agents**: today, agents must read the whole file, prepend a note, and write it back. A `noteflow append < message` subcommand (or HTTP endpoint) is on the roadmap to let agents add a note without owning the whole-file rewrite.

## 8. Examples

### Minimal valid file

```markdown
## 2026-05-12 09:30:45 - First note
Hello, world.
```

### Two notes, newest first

```markdown
## 2026-05-12 14:22:10 - Bug investigation
- [ ] Reproduce on main
- [x] File issue
<!-- note -->
## 2026-05-12 09:30:45 - Morning standup
Plans for the day.
```

### Note with archived link (post-archive)

```markdown
## 2026-05-12 15:00:00 - Reference
See [Go 1.23 release notes](assets/sites/2026_05_12_150003_Go_1_23-go_dev.html) (archived 2026-05-12 15:00) for the relevant changes.
```

### Empty-title note

```markdown
## 2026-05-12 16:00:00

Just a quick thought, no title.
```

---

## 9. How to use this doc

- **Adding a parser feature?** Update §3 or §4 first, add the test case, then change code.
- **Writing an AI-agent integration?** Match §3 exactly for the header regex, use §6 invariants as your safety guarantees, and watch §7 for upcoming changes that may affect you.
- **Reviewing a PR that touches the format?** Confirm the invariants in §6 still hold. Confirm the doc was updated.
- **Curious why the format is shaped this way?** Read this file alongside `internal/models/note.go` and `internal/storage/file.go` — those are the implementation; this is the contract.
