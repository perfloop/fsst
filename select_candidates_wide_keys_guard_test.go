package fsst

import (
	"slices"
	"testing"
)

// wideCandidateFixture uses the maximum symbol width and multi-byte gains that
// buildCandidates can produce, while keeping each candidate key unique.
func wideCandidateFixture(n int) map[[2]uint64]qsym {
	candidates := make(map[[2]uint64]qsym, n)
	for i := range n {
		value := uint64(i) | uint64(i*0x9e37)<<32
		sym := newSymbolFromBytes([]byte{
			byte(value), byte(value >> 8), byte(value >> 16), byte(value >> 24),
			byte(value >> 32), byte(value >> 40), byte(value >> 48), byte(value >> 56),
		})
		gain := uint32(0x1_0000) + uint32(i)*0x9e37
		candidates[[2]uint64{sym.val, uint64(sym.length())}] = qsym{symbol: sym, gain: gain}
	}
	return candidates
}

func sortedWideTopCandidates(candidates map[[2]uint64]qsym) []qsym {
	ordered := make([]qsym, 0, len(candidates))
	for _, candidate := range candidates {
		ordered = append(ordered, candidate)
	}
	slices.SortFunc(ordered, func(a, b qsym) int {
		switch {
		case a.betterThan(b):
			return -1
		case b.betterThan(a):
			return 1
		default:
			return 0
		}
	})
	return ordered[:min(len(ordered), maxCandidateSymbols)]
}

func benchmarkSelectCandidatesWideKeys(b *testing.B, n int) {
	candidates := wideCandidateFixture(n)
	want := sortedWideTopCandidates(candidates)
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

func BenchmarkSelectCandidatesWideKeysAboveLimit(b *testing.B) {
	benchmarkSelectCandidatesWideKeys(b, maxCandidateSymbols+1)
}

func TestSelectCandidatesMixedWidthsMatchesFullOrdering(t *testing.T) {
	const candidateCount = maxCandidateSymbols*2 + 1
	candidates := make(map[[2]uint64]qsym, candidateCount)
	for i := range candidateCount {
		length := 2 + i%7
		if i < 255 {
			length = 1
		}
		value := uint64(i) | uint64(i*0x9e37)<<32
		bytes := make([]byte, length)
		for j := range bytes {
			bytes[j] = byte(value >> (8 * j))
		}
		sym := newSymbolFromBytes(bytes)
		gain := uint32(0x1_0000) + uint32(i)*0x9e37
		candidates[[2]uint64{sym.val, uint64(sym.length())}] = qsym{symbol: sym, gain: gain}
	}

	want := sortedWideTopCandidates(candidates)
	heap := make(qsymHeap, 0, maxCandidateSymbols+1)
	list := make([]qsym, 0, maxCandidateSymbols)
	for range 8 {
		selectCandidates(candidates, &heap, &list)
		if !slices.Equal(list, want) {
			t.Fatal("selected candidates differ from full ordering")
		}
	}
}
