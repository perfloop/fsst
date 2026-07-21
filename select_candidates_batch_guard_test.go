package fsst

import (
	"slices"
	"testing"
)

// BenchmarkSelectCandidatesBatchBoundary exercises one candidate after the
// large-path input buffer first fills with a retained prefix and one full batch.
func BenchmarkSelectCandidatesBatchBoundary(b *testing.B) {
	candidates := candidateFixture(maxCandidateSymbols*2 + 1)
	want := sortedTopCandidates(candidates)
	heap := make(qsymHeap, 0, maxCandidateSymbols+1)
	list := make([]qsym, 0, maxCandidateSymbols)

	selectCandidates(candidates, &heap, &list)
	if !slices.Equal(list, want) {
		b.Fatal("selected candidates differ from full ordering")
	}

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		selectCandidates(candidates, &heap, &list)
	}
	b.StopTimer()

	if !slices.Equal(list, want) {
		b.Fatal("selected candidates differ from full ordering")
	}
}
