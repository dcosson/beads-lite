package filesystem

import (
	"context"
	"fmt"
	"testing"

	"beads-lite/internal/issuestorage"
)

func setupBenchmarkStorage(b *testing.B) *FilesystemStorage {
	b.Helper()
	dir := b.TempDir()
	s := New(dir)
	if err := s.Init(context.Background()); err != nil {
		b.Fatal(err)
	}
	return s
}

// BenchmarkCreate measures the time to create a new issue.
// Target performance: 1-5ms per create.
func BenchmarkCreate(b *testing.B) {
	s := setupBenchmarkStorage(b)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := s.Create(ctx, &issuestorage.Issue{
			Title:       fmt.Sprintf("Benchmark issue %d", i),
			Description: "A test issue for benchmarking",
			Priority:    issuestorage.PriorityMedium,
			Type:        issuestorage.TypeTask,
		})
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkGet measures the time to retrieve an issue by ID.
// Target performance: <1ms per get.
func BenchmarkGet(b *testing.B) {
	s := setupBenchmarkStorage(b)
	ctx := context.Background()

	// Create an issue to retrieve
	id, err := s.Create(ctx, &issuestorage.Issue{
		Title:       "Benchmark issue",
		Description: "A test issue for benchmarking",
		Priority:    issuestorage.PriorityMedium,
		Type:        issuestorage.TypeTask,
	})
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := s.Get(ctx, id)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkListOpen1000 measures the time to list 1000 open issues.
// Target performance: ~50ms for listing 1000 issues.
func BenchmarkListOpen1000(b *testing.B) {
	s := setupBenchmarkStorage(b)
	ctx := context.Background()

	// Create 1000 issues
	for i := 0; i < 1000; i++ {
		_, err := s.Create(ctx, &issuestorage.Issue{
			Title:       fmt.Sprintf("Issue %d", i),
			Description: "A test issue for benchmarking list performance",
			Priority:    issuestorage.PriorityMedium,
			Type:        issuestorage.TypeTask,
		})
		if err != nil {
			b.Fatal(err)
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		issues, err := s.List(ctx, nil)
		if err != nil {
			b.Fatal(err)
		}
		if len(issues) != 1000 {
			b.Fatalf("expected 1000 issues, got %d", len(issues))
		}
	}
}

// BenchmarkConcurrentReads measures concurrent read performance.
// Uses b.RunParallel to simulate multiple goroutines reading simultaneously.
func BenchmarkConcurrentReads(b *testing.B) {
	s := setupBenchmarkStorage(b)
	ctx := context.Background()

	// Create an issue to read
	id, err := s.Create(ctx, &issuestorage.Issue{
		Title:       "Benchmark issue",
		Description: "A test issue for benchmarking concurrent reads",
		Priority:    issuestorage.PriorityMedium,
		Type:        issuestorage.TypeTask,
	})
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := s.Get(ctx, id)
			if err != nil {
				b.Error(err)
			}
		}
	})
}
