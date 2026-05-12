package services

import (
	"strings"
	"testing"
)

// These benchmarks check the Goal 3 reliability target:
//   "render a note <100ms"  — see docs/TODO.md → "Long-term Direction" goal 3.
//
// Run with:  go test ./internal/services/ -bench=. -benchmem
// Run a single one with:  go test ./internal/services/ -bench=BenchmarkRender_Typical

const typicalNote = `# Project notes for 2026-05-12

Working on the cross-project task graph. A few open threads:

- [ ] Stable task IDs — needs migration framework first
- [ ] Inline metadata: !p1 @2026-05-20 #release ` + "`" + `ship release notes` + "`" + `
- [x] Foreign-key pragma enabled
- [x] Documented the task DB schema

## Findings

Reading ` + "`internal/services/database.go`" + ` it's clear the sync model is the root cause.
The ` + "`SyncFolderTasks`" + ` function:

` + "```go" + `
DELETE FROM tasks WHERE folder_id = ?;
INSERT INTO tasks (folder_id, file_path, ...) VALUES (...);
` + "```" + `

| Field         | Stable? | Notes                                       |
|---------------|---------|---------------------------------------------|
| ` + "`id`" + `          | no      | AUTOINCREMENT reassigned every sync         |
| ` + "`folder_id`" + `   | yes     | comes from ` + "`folders.id`" + ` which is upserted |
| ` + "`content`" + `     | mostly  | matches markdown source at sync time        |

Math sanity check: $x^2 + y^2 = z^2$ should render via MathJax. Block form:

$$
\sum_{i=1}^{n} i = \frac{n(n+1)}{2}
$$

> A blockquote pulled from the schema doc:
> Stable task IDs and inline metadata are the two prerequisites for almost everything Goal 2 wants.

End of note.
`

// BenchmarkRender_Typical times rendering a realistic note (code blocks,
// tables, math, blockquotes, task lists, headers). Per-op time is the
// number to compare against the <100ms reliability target. As of writing,
// a typical note renders in well under 1ms — the target has 100× headroom.
func BenchmarkRender_Typical(b *testing.B) {
	r := NewMarkdownRenderer()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := r.RenderToHTML(typicalNote); err != nil {
			b.Fatalf("RenderToHTML: %v", err)
		}
	}
}

// BenchmarkRender_Small times the simplest realistic note — a one-liner.
// Useful as a floor for renderer overhead.
func BenchmarkRender_Small(b *testing.B) {
	r := NewMarkdownRenderer()
	const small = "Quick thought: try the upsert variant tomorrow."
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := r.RenderToHTML(small); err != nil {
			b.Fatalf("RenderToHTML: %v", err)
		}
	}
}

// BenchmarkRender_Large stresses the renderer with a 10kB note built from
// repeated paragraphs and task lists. The reliability target is per-note,
// so this is more about catching pathological scaling than about a hard
// budget. If this gets dramatically slower per byte, something regressed.
func BenchmarkRender_Large(b *testing.B) {
	r := NewMarkdownRenderer()
	var sb strings.Builder
	for i := 0; i < 200; i++ {
		sb.WriteString("Paragraph ")
		sb.WriteString(strings.Repeat("word ", 8))
		sb.WriteString("\n\n")
		sb.WriteString("- [ ] task item ")
		sb.WriteString(strings.Repeat("detail ", 4))
		sb.WriteString("\n\n")
	}
	large := sb.String()

	b.ResetTimer()
	b.SetBytes(int64(len(large)))
	for i := 0; i < b.N; i++ {
		if _, err := r.RenderToHTML(large); err != nil {
			b.Fatalf("RenderToHTML: %v", err)
		}
	}
}

// TestRender_TypicalUnderBudget is the regression guard: actual wall-clock
// render time for the typical note must stay under the Goal 3 budget. We
// give plenty of slack (50ms vs. the 100ms goal) so this doesn't flap on
// loaded CI, but a regression that pushes us anywhere near the budget will
// trip this and force a conversation.
func TestRender_TypicalUnderBudget(t *testing.T) {
	r := NewMarkdownRenderer()
	// Warm-up: first call may pay one-time allocation costs.
	if _, err := r.RenderToHTML(typicalNote); err != nil {
		t.Fatalf("warmup RenderToHTML: %v", err)
	}

	result := testing.Benchmark(func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			if _, err := r.RenderToHTML(typicalNote); err != nil {
				b.Fatalf("RenderToHTML: %v", err)
			}
		}
	})

	nsPerOp := result.NsPerOp()
	const budgetNs = 50 * 1000 * 1000 // 50ms in nanoseconds
	if nsPerOp > budgetNs {
		t.Errorf("typical-note render = %d ns/op, exceeds 50ms guard (target is <100ms per Goal 3)",
			nsPerOp)
	}
	t.Logf("typical-note render = %d ns/op (~%.2f ms)", nsPerOp, float64(nsPerOp)/1e6)
}
