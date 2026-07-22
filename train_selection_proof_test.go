package fsst

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"sort"
	"testing"
)

const selectionProofTableSHA256 = "682a257952d7305bdc87f63e5eba2b0304d3128683d98f7c409de0b7f9be298d"

// trainSelectionProofCorpus keeps the public-Train correctness oracle on
// candidate states produced by the production counting and merging pipeline.
func trainSelectionProofCorpus(tb testing.TB) []byte {
	tb.Helper()
	data, err := os.ReadFile("testdata/art_of_war.txt")
	if err != nil {
		tb.Fatalf("read training corpus: %v", err)
	}
	return data
}

func TestTrainSelectionCorpusDeterminismAndRoundTrip(t *testing.T) {
	data := trainSelectionProofCorpus(t)
	inputs := [][]byte{data}

	var wantTable []byte
	for range 4 {
		table := Train(inputs)

		var serialized bytes.Buffer
		if _, err := table.WriteTo(&serialized); err != nil {
			t.Fatalf("serialize trained table: %v", err)
		}
		if got, want := serialized.Len(), 534; got != want {
			t.Fatalf("serialized table length is %d, want %d", got, want)
		}
		tableHash := sha256.Sum256(serialized.Bytes())
		if got := hex.EncodeToString(tableHash[:]); got != selectionProofTableSHA256 {
			t.Fatalf("serialized table hash is %s, want %s", got, selectionProofTableSHA256)
		}
		if wantTable == nil {
			wantTable = bytes.Clone(serialized.Bytes())
		} else if !bytes.Equal(wantTable, serialized.Bytes()) {
			t.Fatal("training produced a different serialized table for the same corpus")
		}

		compressed := table.EncodeAll(data)
		if got := table.DecodeAll(compressed); !bytes.Equal(got, data) {
			t.Fatal("trained table did not round-trip the corpus")
		}

		var restored Table
		if _, err := restored.ReadFrom(bytes.NewReader(serialized.Bytes())); err != nil {
			t.Fatalf("deserialize trained table: %v", err)
		}
		if got := restored.EncodeAll(data); !bytes.Equal(got, compressed) {
			t.Fatal("deserialized table changed corpus compression")
		}
	}
}

const selectionProofCandidateCount = maxCandidateSymbols * 8

// selectionProofCandidates models four-byte merged candidates that
// buildCandidates can emit: every positive gain is an occurrence count times
// the symbol length. The varied counts create ties, while unique symbol values
// exercise betterThan's deterministic total-order fallback.
func selectionProofCandidates(tb testing.TB) map[[2]uint64]qsym {
	tb.Helper()
	candidates := make(map[[2]uint64]qsym, selectionProofCandidateCount)
	for i := range selectionProofCandidateCount {
		symbol := newSymbolFromBytes([]byte{byte(i), byte(i >> 8), byte(i >> 16), byte(i >> 24)})
		candidates[[2]uint64{symbol.val, uint64(symbol.length())}] = qsym{
			symbol: symbol,
			gain:   uint32(1+(i*37)%257) * uint32(symbol.length()),
		}
	}
	return candidates
}

func TestSelectionProofCandidatesAreBuildCandidatesRepresentable(t *testing.T) {
	candidates := selectionProofCandidates(t)
	if got, want := len(candidates), selectionProofCandidateCount; got != want {
		t.Fatalf("candidate count is %d, want %d", got, want)
	}
	for key, candidate := range candidates {
		if got, want := candidate.symbol.length(), uint32(4); got != want {
			t.Fatalf("candidate length is %d, want %d", got, want)
		}
		if candidate.gain == 0 || candidate.gain%candidate.symbol.length() != 0 {
			t.Fatalf("candidate gain %d is not a positive multiple of its length", candidate.gain)
		}
		wantKey := [2]uint64{candidate.symbol.val, uint64(candidate.symbol.length())}
		if key != wantKey {
			t.Fatalf("candidate key %v does not match symbol key %v", key, wantKey)
		}
	}
}

func TestSelectionProofCandidatesKeepExactTopOrder(t *testing.T) {
	candidates := selectionProofCandidates(t)
	heap := make(qsymHeap, 0, maxCandidateSymbols*2)
	var selected []qsym
	selectCandidates(candidates, &heap, &selected)
	if got, want := len(selected), maxCandidateSymbols; got != want {
		t.Fatalf("selected %d candidates, want %d", got, want)
	}

	want := make([]qsym, 0, len(candidates))
	for _, candidate := range candidates {
		want = append(want, candidate)
	}
	sort.Slice(want, func(i, j int) bool {
		return want[i].betterThan(want[j])
	})
	for i := range selected {
		if selected[i] != want[i] {
			t.Fatalf("candidate %d is %+v, want %+v", i, selected[i], want[i])
		}
	}
}

func BenchmarkSelectCandidatesRealizable(b *testing.B) {
	candidates := selectionProofCandidates(b)
	heap := make(qsymHeap, 0, maxCandidateSymbols*2)
	var selected []qsym

	b.ReportAllocs()
	for b.Loop() {
		selectCandidates(candidates, &heap, &selected)
		if len(selected) != maxCandidateSymbols || selected[0].gain == 0 {
			b.Fatal("candidate selection produced an invalid result")
		}
	}
}
