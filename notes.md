## 2026-05-13 17:00:00 - Welcome to NoteFlow-Go

If you just cloned this repo and ran `noteflow-go` in it, this note is what you're reading right now in your browser. Every other note in this file is here to show off what NoteFlow can do — scroll down for the tour.

**Core idea.** `notes.md` lives in your project folder, version-controlled with your code. NoteFlow renders it as a web UI, parses tasks into a shared SQLite index across all your folders, archives linked web pages locally, and stays out of the way. Single binary, no cloud, no account.

### Try this right now

- [ ] !p1 @2026-05-20 #onboarding Read through the rest of the notes in this file
- [ ] !p2 #onboarding Press `/` to open search; try searching for `Schrödinger`
- [ ] !p2 #onboarding Hover the `fonts` tab on the right edge — bump Notes up to `1.4×`
- [ ] !p3 #onboarding Click `+ Add Folder…` on the global tasks page; point it at another folder
- [x] Read this welcome note

You can complete a task two ways: tick the checkbox in the web UI, or run `noteflow-go tasks --toggle <hash>` from the terminal. Both write back to this file *and* to the global task DB.

### Inline task metadata

The token grammar above (`!p1`, `@2026-05-20`, `#onboarding`) drives the `noteflow-go tasks` CLI filters. From any folder with `noteflow-go` on your PATH:

```bash
noteflow-go tasks --due today           # today's planning surface
noteflow-go tasks --priority 1          # everything urgent
noteflow-go tasks --tag onboarding      # this onboarding list
noteflow-go tasks --status              # one-liner for shell prompts
```

See `noteflow-go --help` and `noteflow-go tasks --help` for the full surface.

<!-- note -->
## 2026-05-13 16:00:00 - Sprint planning example

A realistic-looking task list using the inline-metadata grammar. The priorities, due dates, and tags are real tokens — they show up in the global tasks page and the `noteflow-go tasks` CLI.

### This week

- [ ] !p1 @2026-05-15 #release Cut the v1.5.0 release tarball and update the brew formula
- [ ] !p1 @2026-05-15 #release Verify the Windows installer signs cleanly on a clean VM
- [ ] !p2 @2026-05-17 #docs Fill out the "Customizing the UI" section of the README
- [ ] !p2 #refactor Pull the migration helper out of `database.go` into its own file
- [ ] !p3 #cleanup Remove orphan `cmd/noteflow/` directory (gitignored, harmless, still confusing)

### Backlog

- [ ] !p3 #ideas Status-bar app for macOS that calls `noteflow-go tasks --status` every 60s
- [ ] !p3 #ideas Plugin-free embedded LLM chat — *no, on second thought, this violates Goal 3*

### Recently shipped

- [x] !p1 #release Stable task IDs (the schema-doc §7 blocker)
- [x] !p1 #release `noteflow-go tasks` CLI with the full filter surface
- [x] !p1 #release Search (this week's headliner — press `/` to try it)

<!-- note -->
## 2026-05-13 15:00:00 - Code-aware notes — the `+file:` sigil

NoteFlow ships a markdown sigil that embeds code straight from your repo. When you save a note containing `+file:path/to/file.go#10-25`, it expands at save time into a fenced code block with the language detected from the extension, and a `// path#range` header so you can find the source.

Example you can write in a new note:

```
The migration ordering matters — see +file:internal/services/database.go#85-120
```

When saved, that single line becomes a real code block referencing the migration code at those exact lines. The sigil supports:

- `+file:path` — entire file
- `+file:path#10` — just line 10
- `+file:path#10-25` — inclusive range

**Security.** Path resolution is sandboxed to the project folder. Absolute paths, `..` escape attempts, and symlinks pointing outside the repo are rejected with a logged warning (the sigil is left in place rather than silently failing).

### Why this matters

Project notes that *reference* code are everywhere. Notes that *contain* code at the right version, embedded next to your thinking, are rare. The sigil makes embedded-code the default — you don't paste, you just point at lines.

For more, see [`docs/20260512_notes_md_schema.md`](docs/20260512_notes_md_schema.md) §4 for the full token grammar and §6 for the diff-friendliness invariants this format promises.

<!-- note -->
## 2026-05-13 14:00:00 - Tables, formatting, and the small stuff

NoteFlow uses [Goldmark](https://github.com/yuin/goldmark) under the hood, so it supports the usual CommonMark + GitHub-flavored extensions cleanly.

### Tables

| Feature                  | Status   | Where to find it                              |
|--------------------------|----------|-----------------------------------------------|
| Markdown + MathJax       | shipped  | Just write — every note renders               |
| Task inline metadata     | shipped  | `!p1 @YYYY-MM-DD #tag` in any `- [ ]` line   |
| Cross-folder task search | shipped  | `noteflow-go tasks` CLI                       |
| `/` search               | shipped  | Press `/` anywhere on the page                |
| `+http://` archiving     | shipped  | Prefix any URL with `+` to archive on save    |
| `+file:` snippet sigil   | shipped  | Embed code by file path + line range          |
| Git context in UI        | shipped  | Branch + recent commits panel on the right    |

### Blockquotes

> Project notes that *reference* code are everywhere. Notes that *contain* code at the right version, embedded next to your thinking, are rare.

### Lists

1. Numbered lists work
2. With sub-items:
   - Like this one
   - Indented two spaces
3. And formatting inside: **bold**, *italic*, `inline code`, ~~strikethrough~~

### Inline code & fenced blocks

You can quote a function name like `processCodeSnippets()` mid-sentence, or block-quote a snippet:

```go
// stripCheckbox removes the "- [ ] " prefix when displaying tasks.
func stripCheckbox(line string) string {
    t := strings.TrimLeft(line, " ")
    return strings.TrimPrefix(t, "- ")
}
```

Code blocks scale with the Notes font size — try `⌘/Ctrl+Alt+=` while you read this.

<!-- note -->
## 2026-05-13 13:00:00 - MathJax showcase

NoteFlow ships with MathJax pre-wired: wrap an expression in **single dollar signs for inline math** or **double dollar signs for block math**, and it renders as proper LaTeX. Useful for research notebooks, lecture notes, and any project where the README isn't enough.

### Inline math

Probability of two independent events: $P(A \cap B) = P(A) \cdot P(B)$. The relativistic energy of a body at rest: $E = mc^2$. Both render inline with your prose.

### Block math — Fourier series

The Fourier series of a periodic function $f(x)$ with period $2\pi$:

$$
f(x) = \frac{a_0}{2} + \sum_{n=1}^{\infty} \left( a_n \cos(nx) + b_n \sin(nx) \right)
$$

Coefficients:

- $a_0 = \frac{1}{\pi} \int_{-\pi}^{\pi} f(x) \, dx$
- $a_n = \frac{1}{\pi} \int_{-\pi}^{\pi} f(x) \cos(nx) \, dx$
- $b_n = \frac{1}{\pi} \int_{-\pi}^{\pi} f(x) \sin(nx) \, dx$

### Matrices and eigenvalues

The characteristic polynomial of matrix $\mathbf{A}$:

$$
\det(\mathbf{A} - \lambda \mathbf{I}) = 0
$$

For a $2 \times 2$ matrix $\mathbf{A} = \begin{pmatrix} a & b \\ c & d \end{pmatrix}$, the eigenvalues are:

$$
\lambda_{1,2} = \frac{(a+d) \pm \sqrt{(a+d)^2 - 4(ad-bc)}}{2}
$$

### Statistics

The Central Limit Theorem — for i.i.d. random variables $X_1, X_2, \ldots, X_n$ with mean $\mu$ and variance $\sigma^2$:

$$
\frac{\bar{X}_n - \mu}{\sigma/\sqrt{n}} \xrightarrow{d} \mathcal{N}(0,1) \text{ as } n \to \infty
$$

Bayes' theorem:

$$
P(H \mid E) = \frac{P(E \mid H) \cdot P(H)}{P(E)}
$$

<!-- note -->
## 2026-05-13 12:00:00 - Working notes — complex analysis & quantum mechanics

A long-form example: real working notes that lean on MathJax. Realistic for anyone using NoteFlow as a research notebook.

### Cauchy-Riemann equations

For a complex function $f(z) = u(x, y) + i v(x, y)$ to be differentiable at $z_0$, the partial derivatives must satisfy:

$$
\frac{\partial u}{\partial x} = \frac{\partial v}{\partial y}, \quad \frac{\partial u}{\partial y} = -\frac{\partial v}{\partial x}
$$

If these hold *and* the partial derivatives are continuous, $f$ is **holomorphic** in the neighborhood of $z_0$.

### Residue theorem

For a function $f(z)$ meromorphic on and inside a simple closed contour $C$, with isolated singularities $z_1, z_2, \ldots, z_k$ inside $C$:

$$
\oint_C f(z) \, dz = 2\pi i \sum_{k} \operatorname{Res}(f, z_k)
$$

### Schrödinger equation

Time-independent form:

$$
-\frac{\hbar^2}{2m} \nabla^2 \Psi(\mathbf{r}) + V(\mathbf{r}) \Psi(\mathbf{r}) = E \Psi(\mathbf{r})
$$

Heisenberg uncertainty:

$$
\Delta x \cdot \Delta p \geq \frac{\hbar}{2}
$$

where $\Delta x = \sqrt{\langle x^2 \rangle - \langle x \rangle^2}$ and similarly for $\Delta p$.

### Practice problems

- [ ] !p2 @2026-05-20 #math Verify Fourier convergence for $f(x) = |x|$ on $[-\pi, \pi]$
- [ ] !p2 @2026-05-22 #math Calculate eigenvalues of $\begin{pmatrix} 3 & 1 \\ 1 & 3 \end{pmatrix}$ — sanity-check against the closed form above
- [ ] !p3 #math Sketch a proof of the residue theorem from Cauchy's integral formula
- [ ] !p3 #math Solve the harmonic oscillator using the Schrödinger equation above; compare $\langle x^2 \rangle$ to the classical RMS

<!-- note -->
## 2026-05-13 11:00:00 - Web archiving (the `+http://` sigil)

Prefix any URL with `+` and NoteFlow will fetch the page, inline every CSS/JS/image/font as a data URI, and store the result locally so the page survives even if the original gets deleted.

Example you can write in a new note:

```
Saw this technique today: +https://example.com/some-article
```

When saved, that line becomes a regular markdown link pointing to `assets/sites/<timestamp>_<title>_<host>.html` — the locally-archived copy. The archive is fully self-contained: open it in any browser, no network needed.

Useful for:

- **Reference material that might rot** — blog posts and personal sites disappear constantly
- **Citation snapshots** — record the exact state of a source you're quoting
- **Reading offline later** — archive on your laptop, read on a flight

The archive is just an HTML file. It's diff-friendly enough (each archive is one self-contained file referenced from `notes.md`) and you can hand-delete archives you no longer want from `assets/sites/`.
