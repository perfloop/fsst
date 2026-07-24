package fsst

import (
	"os"
	"testing"
)

// trainSelectionShape describes an actual first-round candidate-map cardinality
// produced from a prefix of the checked-in art_of_war corpus. The prefixes stay
// below sampleTarget, so Train uses their exact bytes without sampling.
type trainSelectionShape struct {
	name        string
	prefixBytes int
	minCount    int
	maxCount    int
}

var trainSelectionShapes = []trainSelectionShape{
	{
		name:        "below_limit",
		prefixBytes: 1024,
		minCount:    1,
		maxCount:    maxCandidateSymbols,
	},
	{
		name:        "single_partition",
		prefixBytes: 4096,
		minCount:    maxCandidateSymbols + 1,
		maxCount:    maxCandidateSymbols * 2,
	},
	{
		name:        "overflow_batch",
		prefixBytes: 7168,
		minCount:    maxCandidateSymbols*2 + 1,
	},
}

// firstRoundCandidateCount is an exact cardinality witness for Train's first
// candidate-building round. At frac=8, minCount is one and a new table has only
// distinct one-byte symbols, so every nonzero single count and every sparse pair
// contributes one unique candidate. It intentionally does not call the private
// selector, keeping this public-Train benchmark compatible across selector
// implementations.
func firstRoundCandidateCount(inputs [][]byte) int {
	table := newTable()
	var counter counters
	compressCount(table, &counter, makeSample(inputs), 8)

	singles := 0
	for _, count := range counter.single {
		if count != 0 {
			singles++
		}
	}
	return singles + len(counter.pairList)
}

func trainSelectionShapeInput(b *testing.B, shape trainSelectionShape) [][]byte {
	b.Helper()
	data, err := os.ReadFile("testdata/art_of_war.txt")
	if err != nil {
		b.Fatalf("read art_of_war corpus: %v", err)
	}
	if len(data) < shape.prefixBytes {
		b.Fatalf("art_of_war corpus is %d bytes, need %d", len(data), shape.prefixBytes)
	}

	inputs := [][]byte{data[:shape.prefixBytes:shape.prefixBytes]}
	count := firstRoundCandidateCount(inputs)
	if count < shape.minCount || (shape.maxCount != 0 && count > shape.maxCount) {
		b.Fatalf("%s produced %d first-round candidates, want [%d, %d]", shape.name, count, shape.minCount, shape.maxCount)
	}
	return inputs
}

func benchmarkTrainSelectionShape(b *testing.B, shape trainSelectionShape) {
	inputs := trainSelectionShapeInput(b, shape)
	var table *Table

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		table = Train(inputs)
	}

	if table == nil || table.nSymbols == 0 {
		b.Fatal("Train returned an empty table")
	}
}

func BenchmarkTrainSelectionBelowLimit(b *testing.B) {
	benchmarkTrainSelectionShape(b, trainSelectionShapes[0])
}

func BenchmarkTrainSelectionSinglePartition(b *testing.B) {
	benchmarkTrainSelectionShape(b, trainSelectionShapes[1])
}

func BenchmarkTrainSelectionOverflowBatch(b *testing.B) {
	benchmarkTrainSelectionShape(b, trainSelectionShapes[2])
}
