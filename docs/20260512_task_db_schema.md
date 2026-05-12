# Cross-Project Task DB Schema

**Status**: documented from existing implementation as of 2026-05-12. The task DB is the planned foundation for Goal 2 (see `docs/TODO.md` → "Long-term Direction") — *"make the cross-project task graph the killer feature."* Today it backs a single "all tasks" page; this doc captures the schema as it exists and the gaps that block the planning-layer features the goal calls for.

Changes to the schema must update this doc *and* introduce a migration. Today the table-creation statement is idempotent `CREATE IF NOT EXISTS`; future columns will need an explicit migration path.

---

## 1. Location and lifecycle

- Path: `~/.config/noteflow/tasks.db` (SQLite 3 file)
- Created on first `NoteFlow` launch in any folder; the `.config/noteflow/` directory is created if missing
- Single global database shared across every working directory the user opens NoteFlow in
- Foreign-key enforcement is enabled on connection open via `PRAGMA foreign_keys = ON;`, so the `ON DELETE CASCADE` in the `tasks` schema is now active. `RemoveFolder` still deletes tasks inside a transaction (belt-and-braces; works whether FKs are on or off).

## 2. Tables

### `folders`

Registers every working directory that has ever hosted a NoteFlow session.

| Column      | Type     | Constraints                                | Meaning |
|-------------|----------|--------------------------------------------|---------|
| `id`        | INTEGER  | PRIMARY KEY AUTOINCREMENT                  | Surrogate ID, stable across sessions |
| `path`      | TEXT     | UNIQUE NOT NULL                            | Absolute filesystem path to the project folder |
| `last_scan` | DATETIME | DEFAULT CURRENT_TIMESTAMP                  | Wall-clock time of the most recent task sync from this folder |
| `active`    | BOOLEAN  | DEFAULT 1                                  | 0 when the folder is stale (path no longer exists or no longer registered); 1 when in use |

Lifecycle:
- `RegisterFolder(path)` upserts: if `path` exists, sets `active=1`; otherwise inserts.
- `RemoveFolder(id)` deletes the folder and all its tasks in a single transaction.
- Inactive folders are kept (not deleted) — their `active=0` row remains for audit, but they're filtered out of `GetGlobalTasks` and `GetActiveFolders` via `WHERE active = 1`.

### `task_views`

Saved filter combinations for the `noteflow tasks` CLI (Goal 2 — *"Save common queries as views"*).

| Column     | Type     | Constraints                                | Meaning |
|------------|----------|--------------------------------------------|---------|
| `id`       | INTEGER  | PRIMARY KEY AUTOINCREMENT                  | Surrogate ID |
| `name`     | TEXT     | UNIQUE NOT NULL                            | User-supplied view name (e.g. `blockers`, `today`) |
| `filters`  | TEXT     | NOT NULL                                   | JSON blob: `{"done":bool,"due":"...","priority":N,"tag":"...","project":"..."}`. Unrecognized keys are ignored on read, so the field set is forward-compatible. |
| `created`  | DATETIME | DEFAULT CURRENT_TIMESTAMP                  | When the view was first saved. Re-saves use upsert (`ON CONFLICT(name) DO UPDATE`) and do not advance `created`. |

No foreign keys — views describe filter shapes, not specific tasks, so they survive task DB churn.

### `tasks`

The flat list of every checkbox across every registered folder. Rewritten in full per folder on each sync.

| Column         | Type     | Constraints                                              | Meaning |
|----------------|----------|----------------------------------------------------------|---------|
| `id`           | INTEGER  | PRIMARY KEY AUTOINCREMENT                                | Surrogate ID — **stable across syncs as of 2026-05-12** (see §4) |
| `folder_id`    | INTEGER  | NOT NULL, FK → `folders.id` ON DELETE CASCADE            | Which folder this task belongs to. Cascade now active (FKs enabled). |
| `file_path`    | TEXT     | NOT NULL                                                 | Relative path within the folder; today **always** `"notes.md"` (hard-coded in `SyncFolderTasks`) |
| `line_number`  | INTEGER  | NOT NULL                                                 | **Misnomer**: stores the in-memory task index (0-based, top-down across the file), not a real line number |
| `content`      | TEXT     | NOT NULL                                                 | The raw task text including the `[ ]`/`[x]` marker, as parsed from `notes.md` |
| `completed`    | BOOLEAN  | DEFAULT 0                                                | 1 when the checkbox is `[x]` |
| `last_updated` | DATETIME | DEFAULT CURRENT_TIMESTAMP                                | Wall-clock time the task was last *modified* (content or completion changed). Identical syncs no longer touch this. |
| `task_hash`    | TEXT     | nullable                                                 | 12-char hex prefix of `sha256(content)`, with the checkbox marker normalized out before hashing (so `[ ]`/`[x]`/`[X]` all hash the same). Disambiguated with `#N` suffix for duplicate-text tasks within a folder. Used as the stable identity for the upsert sync — see §4. Nullable to permit graceful migration from pre-2026-05-12 DBs; the sync deletes any legacy NULL rows on first run. **Why normalize the checkbox?** Toggling a task's completion must not change its identity — otherwise `noteflow tasks --toggle <hash>` would only work once. |

## 3. Indexes

```sql
CREATE INDEX idx_tasks_folder         ON tasks(folder_id);
CREATE INDEX idx_tasks_completed      ON tasks(completed);
CREATE INDEX idx_tasks_folder_file    ON tasks(folder_id, file_path);
```

These cover the current query patterns: list all tasks per folder, filter completed, look up by folder+file.

## 4. Sync model

`SyncFolderTasks(folderID, tasks)` is the only write path for `tasks`. As of 2026-05-12 it is an **upsert keyed on `(folder_id, task_hash)`**:

```
BEGIN TRANSACTION;
  -- Step 1: clear any legacy rows without a hash (one-time migration cleanup).
  DELETE FROM tasks WHERE folder_id = ? AND task_hash IS NULL;

  -- Step 2: read existing hashes; delete any not in the current task list.
  -- Step 3: for each current task: UPDATE if its hash exists, else INSERT.
  --   The UPDATE bumps last_updated only when content or completed changed,
  --   via a CASE expression.

  UPDATE folders SET last_scan = NOW() WHERE id = ?;
COMMIT;
```

**Consequences — task identity is now stable.** A task's `id` no longer changes across syncs unless the task's *text* changes. This means:

- A long-lived URL like `/tasks/42` remains valid until the user actually edits the task
- Goal 2's CLI / status-line / bidirectional-toggle features can reference tasks safely
- `last_updated` now means "task last modified," not "row last touched by a sync" — answers "what changed this week?" usefully

A task whose text is edited gets a new row with a new `id` (old row deleted, new row inserted), since the content hash changes. This is intentional: the planning layer treats edits as new tasks for completion-tracking purposes, which matches how users think.

**Disambiguation for duplicate text**: if a folder has two `- [ ] same thing` tasks, the second gets hash `<base>#1`, the third gets `<base>#2`, and so on. The first occurrence keeps the bare hash. This means re-ordering duplicate-text tasks within a file can shuffle which row owns the bare hash vs. the `#N` suffix — acceptable for a rare case, and irrelevant when users avoid duplicate task text (which they generally do).

## 5. Reading queries

Two read shapes ship today:

**`GetGlobalTasks`** — `SELECT … FROM tasks JOIN folders … WHERE f.active = 1 ORDER BY f.path, t.completed, t.last_updated DESC`. Returns flat task rows plus per-folder summaries.

**`getTaskSummaries`** — `SELECT path, COUNT(*), SUM(completed), SUM(NOT completed), MAX(last_updated) FROM folders LEFT JOIN tasks GROUP BY folder`. Used to populate the per-folder rollup on the global tasks page.

Notably absent: any filter by due date, priority, tag, age, or text. Those don't exist as columns yet.

## 6. Invariants and guarantees today

What callers can rely on:

1. **`folders.path` is unique and absolute.** Two NoteFlow sessions in the same working directory share one folder row.
2. **`tasks.folder_id` always points to a real `folders.id`** during normal operation, because both writes go through application code that maintains the link. (No DB-level constraint enforces this — foreign keys are off.)
3. **`tasks` rows for a given folder are a complete snapshot** of that folder's `notes.md` as of `folders.last_scan`. There is no partial state.
4. **`completed` matches the markdown source** at sync time. Diverges between syncs — a user toggling a checkbox in NoteFlow's web UI updates the markdown immediately and triggers a re-sync.
5. **The DB schema is forward-compatible with new columns** (existing inserts name all columns explicitly) but `CREATE IF NOT EXISTS` will not add columns to an existing table — a real migration path is required for any future column.

## 7. Open questions (gaps blocking Goal 2)

These are the explicit items the Goal 2 roadmap depends on. Code should not assume any particular answer until decided:

- ~~**Stable task IDs.**~~ **RESOLVED 2026-05-12.** Sync is now an upsert keyed on `task_hash`; see §4. `tasks.id` stays stable across syncs for unchanged tasks. Inline `<!-- task:abc123 -->` markers in `notes.md` remain a possible future enhancement if we need IDs to survive text edits, but content-hash identity is enough for everything Goal 2 needs today.
- **Inline task metadata.** Schema does not yet store due date, priority, or tag in dedicated columns — those are parsed on read from `tasks.content` by `models.ParseTaskMetadata` (see `docs/20260512_notes_md_schema.md` §4). For larger task counts, promoting these to real columns (`due_date DATE NULL`, `priority INTEGER NULL`, `tags TEXT NULL`) would let SQL do the filtering. Use the existing `addColumnIfMissing` migration helper when this lands.
- **Real `line_number`.** Today it's the in-memory task index, not the line in the file. A real line number would let an external editor jump straight to the task. Cheap to fix — change `SyncFolderTasks` to track byte/line position when parsing.
- **Multiple files per folder.** `file_path` is always `"notes.md"`. The schema supports more — the column exists — but no code path uses it. Out of scope unless multi-file vaults become a thing (currently a Goal-3 "no").
- **Migration framework.** No version table, no migration runner. Adding columns will require either a `db_version` table + ordered migrations, or a one-shot `ALTER TABLE` block guarded by a version check. Pick this before the first column add.
- ~~**`last_updated` semantics.**~~ **RESOLVED 2026-05-12.** `last_updated` now advances only when content or completed actually changes — the upsert UPDATE uses a CASE expression to gate the timestamp. "What changed this week?" is answerable now.

## 8. Roadmap implications

The planning-layer features in Goal 2 map onto schema work as follows:

| Goal 2 feature                                       | Status / remaining blockers                                           |
|------------------------------------------------------|----------------------------------------------------------------------|
| `noteflow tasks --due today` CLI                     | **Shipped** (2026-05-12). Filtering done in-process via `models.ParseTaskMetadata`. |
| Filter by project / priority / tag / status          | **Shipped** in the CLI. Promote to DB columns when row count makes in-process filtering slow. |
| "Today / this week / overdue" web views              | Not started. UI work; data layer ready (the CLI is the proof).        |
| Status-line / menu-bar surface                       | Not started. Stable IDs ready; needs a long-lived local endpoint/socket. |
| Bidirectional integrity on toggle                    | **Shipped via `noteflow tasks --toggle <hash>`** (2026-05-12). Updates the source `notes.md` via `NoteManager`, then updates the DB row's `content`/`completed`/`last_updated` so state stays consistent without waiting for the next full sync. Caveat: if the web server is also running against the same folder it holds a stale in-memory view until its next sync — see TODO.md technical debt for the file-lock fix. |
| Saved views                                          | **Shipped via `task_views` table + CLI `--save-view`/`--view`/`--list-views`/`--delete-view`** (2026-05-12). View filters are JSON-encoded so the field set is forward-compatible. CLI flags override view values, so `--view daily --priority 1` works as a tweaked-copy. |

The two long-cited prerequisites (stable IDs, inline metadata parsing) are both resolved. Everything else is now incremental product work, not architectural.

## 9. How to use this doc

- **Adding a task field?** Update §2 first, write the migration, then change `SyncFolderTasks`.
- **Building a query?** Check §3 for index coverage. If your query can't use an existing index and runs often, propose a new one with rationale.
- **Touching the sync path?** Re-read §4 and §7 — the delete-and-rewrite model is convenient but it's the root cause of the unstable-ID problem. Don't entrench it further.
- **Wondering if a Goal 2 feature is feasible?** Check §8 — it maps roadmap items to the schema work each needs.
