package fsst

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"runtime"
	"testing"
)

// TestTrainArtOfWarSelectionStable locks the serialized table produced by a
// representative corpus whose training path repeatedly ranks candidates.
func TestTrainArtOfWarSelectionStable(t *testing.T) {
	data, err := os.ReadFile("testdata/art_of_war.txt")
	if err != nil {
		t.Fatalf("read corpus: %v", err)
	}

	table := Train([][]byte{data})
	serialized, err := table.MarshalBinary()
	if err != nil {
		t.Fatalf("serialize table: %v", err)
	}

	digest := sha256.Sum256(serialized)
	const wantDigest = "682a257952d7305bdc87f63e5eba2b0304d3128683d98f7c409de0b7f9be298d"
	if got := hex.EncodeToString(digest[:]); got != wantDigest {
		t.Fatalf("serialized table changed: got %s, want %s", got, wantDigest)
	}

	compressed := table.EncodeAll(data)
	if got := table.DecodeAll(compressed); !bytes.Equal(got, data) {
		t.Fatal("art_of_war roundtrip mismatch")
	}

	var restored Table
	if err := restored.UnmarshalBinary(serialized); err != nil {
		t.Fatalf("deserialize table: %v", err)
	}
	if got := restored.EncodeAll(data); !bytes.Equal(got, compressed) {
		t.Fatal("serialized table changed compressed output")
	}
}

const benchmarkSelectionScratchSymbols = maxCandidateSymbols * 2

// artOfWarCandidates reaches candidate selection through the same sample,
// counters, and candidate construction that Train uses. Its map values
// therefore satisfy buildCandidates' gain and symbol-length relationships.
func artOfWarCandidates(tb testing.TB) map[[2]uint64]qsym {
	tb.Helper()

	data, err := os.ReadFile("testdata/art_of_war.txt")
	if err != nil {
		tb.Fatalf("read corpus: %v", err)
	}

	sample := makeSample([][]byte{data})
	table := newTable()
	var counter counters
	candidates := make(map[[2]uint64]qsym, benchmarkSelectionScratchSymbols)
	heap := make(qsymHeap, 0, benchmarkSelectionScratchSymbols)
	list := make([]qsym, 0, maxCandidateSymbols)
	for _, frac := range []int{8, 38} {
		counter.reset()
		compressCount(table, &counter, sample, frac)
		buildCandidates(table, &counter, frac, candidates, &heap, &list)
	}
	if got := len(candidates); got <= benchmarkSelectionScratchSymbols {
		tb.Fatalf("candidate map has %d entries, want more than %d", got, benchmarkSelectionScratchSymbols)
	}
	return candidates
}

func TestArtOfWarCandidateMapExceedsSelectionBatch(t *testing.T) {
	const wantCandidates = 3546
	candidates := artOfWarCandidates(t)
	if got := len(candidates); got != wantCandidates {
		t.Fatalf("candidate map has %d entries, want %d", got, wantCandidates)
	}
	t.Logf("art_of_war candidates=%d", len(candidates))
}

func BenchmarkSelectCandidatesArtOfWar(b *testing.B) {
	candidates := artOfWarCandidates(b)
	heap := make(qsymHeap, 0, benchmarkSelectionScratchSymbols)
	list := make([]qsym, 0, maxCandidateSymbols)

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		selectCandidates(candidates, &heap, &list)
		runtime.KeepAlive(list)
	}
}
