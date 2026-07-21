package fsst

import (
	"slices"
	"strconv"
	"testing"
)

// candidateFixture builds candidates with repeated gains so selection must use
// qsym's full deterministic ordering, not only the gain field.
func candidateFixture(n int) map[[2]uint64]qsym {
	candidates := make(map[[2]uint64]qsym, n)
	for i := range n {
		sym := newSymbolFromBytes([]byte{byte(i), byte(i >> 8), byte(i >> 16), byte(i >> 24)})
		candidate := qsym{
			symbol: sym,
			gain:   uint32((i * 37) % 67),
		}
		candidates[[2]uint64{sym.val, uint64(sym.length())}] = candidate
	}
	return candidates
}

func sortedTopCandidates(candidates map[[2]uint64]qsym) []qsym {
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

func TestSelectCandidatesMatchesFullOrdering(t *testing.T) {
	for _, n := range []int{
		maxCandidateSymbols - 1,
		maxCandidateSymbols,
		maxCandidateSymbols + 1,
		maxCandidateSymbols * 8,
	} {
		t.Run("candidates="+strconv.Itoa(n), func(t *testing.T) {
			candidates := candidateFixture(n)
			want := sortedTopCandidates(candidates)
			heap := make(qsymHeap, 0, maxCandidateSymbols+1)
			list := make([]qsym, 0, maxCandidateSymbols)

			for range 8 {
				selectCandidates(candidates, &heap, &list)
				if !slices.Equal(list, want) {
					t.Fatalf("selected candidates differ from full ordering")
				}
			}
		})
	}
}

func BenchmarkSelectCandidatesCardinality(b *testing.B) {
	for _, tc := range []struct {
		name string
		n    int
	}{
		{name: "below_limit", n: maxCandidateSymbols - 1},
		{name: "above_limit", n: maxCandidateSymbols + 1},
		{name: "oversized", n: maxCandidateSymbols * 8},
	} {
		b.Run(tc.name, func(b *testing.B) {
			candidates := candidateFixture(tc.n)
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
		})
	}
}
