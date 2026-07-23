package fsst

import (
	"os"
	"runtime"
	"testing"
)

// BenchmarkTrainSelectionComparisons measures the ranking comparisons reached
// by public Train on the bounded art_of_war corpus. It uses the real candidate
// maps built during Train rather than manufacturing a direct selector fixture.
func BenchmarkTrainSelectionComparisons(b *testing.B) {
	if got := runtime.GOMAXPROCS(0); got != 1 {
		b.Fatalf("GOMAXPROCS = %d, want 1", got)
	}
	data, err := os.ReadFile("testdata/art_of_war.txt")
	if err != nil {
		b.Fatal(err)
	}
	inputs := [][]byte{data}

	b.ReportAllocs()
	var (
		total  uint64
		result *Table
	)
	for b.Loop() {
		selectionComparisonCount = 0
		selectionComparisonCounting = true
		result = Train(inputs)
		selectionComparisonCounting = false
		total += selectionComparisonCount
	}
	if result == nil || result.nSymbols == 0 {
		b.Fatal("Train did not produce a populated table")
	}
	b.ReportMetric(float64(total)/float64(b.N), "qsym-comparisons/op")
}
