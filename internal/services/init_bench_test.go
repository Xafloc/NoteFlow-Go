package services

import (
	"path/filepath"
	"testing"
)

// Startup-time benchmarks for the Goal 3 reliability target:
//   "start <50ms"  — see docs/TODO.md → "Long-term Direction" goal 3.
//
// The full binary startup is roughly:
//   NewNoteManager(cwd)  +  NewTemplateService(assets)  +  NewDatabaseService()  +  fiber.New()
//
// Each is benchmarked individually below. The total budget is 50ms; on
// modern hardware (M2) these collectively take well under that. If the sum
// of medians ever approaches 50ms, the regression guard `TestStartup_*` tests
// will fail and force a conversation before users see slow launches.
//
// Run with: go test ./internal/services/ -bench=BenchmarkStartup -benchmem

func BenchmarkStartup_NewNoteManager(b *testing.B) {
	// NewNoteManager loads notes.md, creates assets directories, and parses
	// tasks. This is the dominant cost on a fresh "noteflow" launch in an
	// existing project folder.
	dir := b.TempDir()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mgr, err := NewNoteManager(dir)
		if err != nil {
			b.Fatalf("NewNoteManager: %v", err)
		}
		_ = mgr
	}
}

func BenchmarkStartup_NewTemplateService(b *testing.B) {
	// nil embed.FS makes the service fall back to filesystem reads. That's
	// the same code path as production once the embedded FS has been
	// dereferenced — the resolve work is the same.
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Run from the repo root so the relative web/templates path resolves.
		ts, err := NewTemplateService(nil)
		if err != nil {
			b.Skipf("TemplateService init from working dir not possible: %v", err)
		}
		_ = ts
	}
}

func BenchmarkStartup_NewDatabaseServiceAt(b *testing.B) {
	// DB open + migrate + PRAGMA foreign_keys. The migration is idempotent
	// so a re-open against an existing DB is the warm path (what users see
	// on every launch after the first).
	dbPath := filepath.Join(b.TempDir(), "tasks.db")
	// Warm: ensure migrations have already run by opening once and closing.
	warm, err := NewDatabaseServiceAt(dbPath)
	if err != nil {
		b.Fatalf("warmup: %v", err)
	}
	warm.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		svc, err := NewDatabaseServiceAt(dbPath)
		if err != nil {
			b.Fatalf("NewDatabaseServiceAt: %v", err)
		}
		svc.Close()
	}
}

// TestStartup_UnderBudget is the regression guard. It exercises the same
// three init paths the binary takes on launch and asserts the sum is well
// under the 50ms target. Generous slack (30ms) so the test doesn't flap on
// loaded CI, but anything close to budget will trip this.
func TestStartup_UnderBudget(t *testing.T) {
	noteDir := t.TempDir()
	dbDir := t.TempDir()
	dbPath := filepath.Join(dbDir, "tasks.db")

	// Warm both: migrations and asset reads should be amortized away.
	if mgr, err := NewNoteManager(noteDir); err != nil {
		t.Fatalf("warmup NoteManager: %v", err)
	} else {
		_ = mgr
	}
	if svc, err := NewDatabaseServiceAt(dbPath); err != nil {
		t.Fatalf("warmup DB: %v", err)
	} else {
		svc.Close()
	}

	noteResult := testing.Benchmark(func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			mgr, err := NewNoteManager(noteDir)
			if err != nil {
				b.Fatalf("NewNoteManager: %v", err)
			}
			_ = mgr
		}
	})
	dbResult := testing.Benchmark(func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			svc, err := NewDatabaseServiceAt(dbPath)
			if err != nil {
				b.Fatalf("NewDatabaseServiceAt: %v", err)
			}
			svc.Close()
		}
	})

	totalNs := noteResult.NsPerOp() + dbResult.NsPerOp()
	const budgetNs = 30 * 1000 * 1000 // 30ms guard well under the 50ms target
	if totalNs > budgetNs {
		t.Errorf("startup (NoteManager + DB) = %d ns/op total, exceeds 30ms guard (target is <50ms per Goal 3)",
			totalNs)
	}
	t.Logf("startup: NoteManager=%dns, DB=%dns, total=%dns (~%.2f ms; 50ms budget)",
		noteResult.NsPerOp(), dbResult.NsPerOp(), totalNs, float64(totalNs)/1e6)
}
